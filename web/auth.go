package web

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"gfep/internal/logx"
	"gfep/utils"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const (
	sessionCookieName = "gfep_session"
	sessionTokenBytes = 32
)

type roleKind string

const (
	roleAdmin roleKind = "admin"
	roleUser  roleKind = "user"
)

type userRec struct {
	Username string   `json:"username"`
	Role     roleKind `json:"role"`
	Hash     string   `json:"password_hash"`
}

type usersFile struct {
	Users []userRec `json:"users"`
}

type sessionRec struct {
	Username  string
	Role      roleKind
	ExpiresAt time.Time
}

var (
	sessMu sync.Mutex
	sess   = make(map[string]*sessionRec)
)

func usersPath() string {
	dir := filepath.Dir(utils.GlobalObject.ConfFilePath)
	if dir == "" || dir == "." {
		dir = "conf"
	}
	return filepath.Join(dir, "web_users.json")
}

func sessionTTL() time.Duration {
	m := utils.GlobalObject.LogWebSessionIdleMin
	if m <= 0 {
		m = 480
	}
	return time.Duration(m) * time.Minute
}

func randomToken() string {
	b := make([]byte, sessionTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return base64.RawURLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

var usersMu sync.RWMutex

func readUsersFile() ([]userRec, error) {
	p := usersPath()
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	data = bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))
	var f usersFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.Users, nil
}

func writeUsersFile(users []userRec) error {
	p := usersPath()
	payload, err := json.MarshalIndent(usersFile{Users: users}, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		return err
	}
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, payload, 0600); err != nil {
		return err
	}
	if err := os.Rename(tmp, p); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func loadUsers() ([]userRec, error) {
	usersMu.RLock()
	defer usersMu.RUnlock()
	return readUsersFile()
}

// TryBootstrapUsers 若不存在 web_users.json 且设置了 GFEP_WEB_BOOTSTRAP_PASSWORD，则创建初始管理员。
func TryBootstrapUsers() error {
	p := usersPath()
	usersMu.Lock()
	defer usersMu.Unlock()
	if _, err := os.Stat(p); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	pw := strings.TrimSpace(os.Getenv("GFEP_WEB_BOOTSTRAP_PASSWORD"))
	if pw == "" {
		return os.ErrNotExist
	}
	user := strings.TrimSpace(os.Getenv("GFEP_WEB_BOOTSTRAP_USER"))
	if user == "" {
		user = "admin"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u := []userRec{{Username: user, Role: roleAdmin, Hash: string(hash)}}
	if err := writeUsersFile(u); err != nil {
		return err
	}
	logx.Printf("web: created initial admin user %q from GFEP_WEB_BOOTSTRAP_PASSWORD (remove env after first login)", user)
	return nil
}

func authenticate(username, password string) (roleKind, bool) {
	users, err := loadUsers()
	if err != nil {
		return "", false
	}
	for _, u := range users {
		if !strings.EqualFold(strings.TrimSpace(u.Username), strings.TrimSpace(username)) {
			continue
		}
		if bcrypt.CompareHashAndPassword([]byte(u.Hash), []byte(password)) != nil {
			return "", false
		}
		role := u.Role
		if role != roleAdmin && role != roleUser {
			role = roleUser
		}
		return role, true
	}
	return "", false
}

func sessionCreate(username string, role roleKind) string {
	sessMu.Lock()
	defer sessMu.Unlock()
	tok := randomToken()
	sess[tok] = &sessionRec{
		Username:  username,
		Role:      role,
		ExpiresAt: time.Now().Add(sessionTTL()),
	}
	return tok
}

func sessionTouch(tok string) *sessionRec {
	sessMu.Lock()
	defer sessMu.Unlock()
	s := sess[tok]
	if s == nil || time.Now().After(s.ExpiresAt) {
		delete(sess, tok)
		return nil
	}
	s.ExpiresAt = time.Now().Add(sessionTTL())
	return s
}

// SessionDelete 登出时删除服务端会话。
func SessionDelete(tok string) {
	sessMu.Lock()
	delete(sess, tok)
	sessMu.Unlock()
}

// PruneSessions 清理过期会话（定时调用）。
func PruneSessions() {
	sessMu.Lock()
	now := time.Now()
	for k, s := range sess {
		if now.After(s.ExpiresAt) {
			delete(sess, k)
		}
	}
	sessMu.Unlock()
}
