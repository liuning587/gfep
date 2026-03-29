package web

import (
	"errors"
	"fmt"
	"strings"

	"gfep/internal/logx"

	"golang.org/x/crypto/bcrypt"
)

// UserSummary Web 用户列表项（不含密码）。
type UserSummary struct {
	Username string `json:"username"`
	Role     string `json:"role"`
}

var (
	errUserExists    = errors.New("用户名已存在")
	errUserNotFound  = errors.New("用户不存在")
	errLastAdmin     = errors.New("不能删除或降级最后一个管理员")
	errWeakPassword  = errors.New("密码至少 6 位")
	errEmptyUsername = errors.New("用户名为空")
)

// ParseRole 解析 API 中的角色字符串。
func ParseRole(s string) (roleKind, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "admin":
		return roleAdmin, nil
	case "user", "":
		return roleUser, nil
	default:
		return "", fmt.Errorf("无效角色: %q", s)
	}
}

func adminCount(users []userRec) int {
	n := 0
	for _, u := range users {
		if u.Role == roleAdmin {
			n++
		}
	}
	return n
}

func findUserIndex(users []userRec, name string) int {
	name = strings.TrimSpace(name)
	for i, u := range users {
		if strings.EqualFold(strings.TrimSpace(u.Username), name) {
			return i
		}
	}
	return -1
}

// ListWebUsers 列出用户（不含密码哈希）。
func ListWebUsers() ([]UserSummary, error) {
	users, err := loadUsers()
	if err != nil {
		return nil, err
	}
	out := make([]UserSummary, 0, len(users))
	for _, u := range users {
		out = append(out, UserSummary{Username: u.Username, Role: string(u.Role)})
	}
	return out, nil
}

// AddWebUser 新增控制台用户。
func AddWebUser(username, password string, role roleKind) error {
	if len(strings.TrimSpace(password)) < 6 {
		return errWeakPassword
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errEmptyUsername
	}
	usersMu.Lock()
	defer usersMu.Unlock()
	users, err := readUsersFile()
	if err != nil {
		return err
	}
	if findUserIndex(users, username) >= 0 {
		return errUserExists
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if role != roleAdmin && role != roleUser {
		role = roleUser
	}
	users = append(users, userRec{Username: username, Role: role, Hash: string(hash)})
	if err := writeUsersFile(users); err != nil {
		return err
	}
	logx.Printf("web audit: user added %q role=%s", username, role)
	return nil
}

// UpdateWebUser 修改指定用户：newPassword 非 nil 且非空则更新密码；newRole 非 nil 则更新角色。
func UpdateWebUser(username string, newPassword *string, newRole *roleKind) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errEmptyUsername
	}
	usersMu.Lock()
	defer usersMu.Unlock()
	users, err := readUsersFile()
	if err != nil {
		return err
	}
	idx := findUserIndex(users, username)
	if idx < 0 {
		return errUserNotFound
	}
	if newRole != nil {
		r := *newRole
		if r != roleAdmin && r != roleUser {
			r = roleUser
		}
		if users[idx].Role == roleAdmin && r != roleAdmin {
			if adminCount(users) <= 1 {
				return errLastAdmin
			}
		}
		users[idx].Role = r
	}
	if newPassword != nil && *newPassword != "" {
		if len(*newPassword) < 6 {
			return errWeakPassword
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(*newPassword), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		users[idx].Hash = string(hash)
	}
	if err := writeUsersFile(users); err != nil {
		return err
	}
	logx.Printf("web audit: user updated %q", username)
	return nil
}

// DeleteWebUser 删除用户。
func DeleteWebUser(username string) error {
	username = strings.TrimSpace(username)
	if username == "" {
		return errEmptyUsername
	}
	usersMu.Lock()
	defer usersMu.Unlock()
	users, err := readUsersFile()
	if err != nil {
		return err
	}
	idx := findUserIndex(users, username)
	if idx < 0 {
		return errUserNotFound
	}
	if users[idx].Role == roleAdmin && adminCount(users) <= 1 {
		return errLastAdmin
	}
	users = append(users[:idx], users[idx+1:]...)
	if err := writeUsersFile(users); err != nil {
		return err
	}
	logx.Printf("web audit: user deleted %q", username)
	return nil
}
