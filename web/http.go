package web

import (
	"encoding/json"
	"errors"
	"fmt"
	"gfep/internal/logx"
	"gfep/utils"
	"gfep/zlog"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Server HTTP 控制台（嵌入静态资源与 API）。
type Server struct {
	AbsLogRoot string
	Provider   *Provider
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readSession(r *http.Request) *sessionRec {
	c, err := r.Cookie(sessionCookieName)
	if err != nil || c.Value == "" {
		return nil
	}
	return sessionTouch(c.Value)
}

func clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		MaxAge:   int(sessionTTL().Seconds()),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) requireAuth(w http.ResponseWriter, r *http.Request) *sessionRec {
	sess := readSession(r)
	if sess == nil {
		clearSessionCookie(w)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "未登录或会话已过期"})
		return nil
	}
	return sess
}

func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) *sessionRec {
	sess := s.requireAuth(w, r)
	if sess == nil {
		return nil
	}
	if sess.Role != roleAdmin {
		writeJSON(w, http.StatusForbidden, map[string]string{"error": "需要管理员权限"})
		return nil
	}
	return sess
}

func protocolFilterMatch(have, want string) bool {
	have = strings.TrimSpace(strings.ToLower(have))
	want = strings.TrimSpace(strings.ToLower(want))
	if want == "" {
		return true
	}
	if have == "" {
		return false
	}
	if have == want {
		return true
	}
	return strings.Contains(have, want) || strings.Contains(want, have)
}

// liveLineMatchesFilters：addr 匹配 JSON 的 addr、remoteTcp 或整行子串；protocol 非空时仅保留带 protocol 字段且匹配的 JSON 行（kind=pkt），或不含 kind 的 JSON 若含 protocol 也可匹配。
func liveLineMatchesFilters(line, addrFilter, protoFilter string) bool {
	addrFilter = strings.TrimSpace(addrFilter)
	protoFilter = strings.TrimSpace(protoFilter)
	if addrFilter == "" && protoFilter == "" {
		return true
	}
	var m map[string]any
	if json.Unmarshal([]byte(line), &m) != nil {
		if protoFilter != "" {
			return false
		}
		if addrFilter == "" {
			return true
		}
		return strings.Contains(strings.ToLower(line), strings.ToLower(addrFilter))
	}
	if protoFilter != "" {
		p, _ := m["protocol"].(string)
		if !protocolFilterMatch(p, protoFilter) {
			return false
		}
	}
	if addrFilter == "" {
		return true
	}
	f := strings.ToLower(addrFilter)
	if a, ok := m["addr"].(string); ok && strings.Contains(strings.ToLower(a), f) {
		return true
	}
	if rt, ok := m["remoteTcp"].(string); ok && strings.Contains(strings.ToLower(rt), f) {
		return true
	}
	return strings.Contains(strings.ToLower(line), f)
}

func (s *Server) handleAuthLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var body struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效请求体"})
		return
	}
	role, ok := authenticate(body.Username, body.Password)
	if !ok {
		logx.Printf("web audit: login failed user=%q remote=%s", body.Username, r.RemoteAddr)
		writeJSON(w, http.StatusUnauthorized, map[string]string{"error": "用户名或密码错误"})
		return
	}
	tok := sessionCreate(body.Username, role)
	setSessionCookie(w, tok)
	logx.Printf("web audit: login ok user=%q role=%s remote=%s", body.Username, role, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "username": body.Username, "role": string(role)})
}

func (s *Server) handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		SessionDelete(c.Value)
	}
	clearSessionCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) handleAuthMe(w http.ResponseWriter, r *http.Request) {
	sess := s.requireAuth(w, r)
	if sess == nil {
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"username": sess.Username, "role": string(sess.Role)})
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	var host HostStatus
	var byProto map[string]int
	if s.Provider != nil {
		if s.Provider.HostStatus != nil {
			host = s.Provider.HostStatus()
		}
		if s.Provider.TerminalCounts != nil {
			byProto = s.Provider.TerminalCounts()
		}
	}
	if byProto == nil {
		byProto = map[string]int{}
	}
	srv := utils.GlobalObject.TCPServer
	connN := 0
	if srv != nil {
		connN = srv.GetConnMgr().Len()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"host":                host,
		"tcpConnTotal":        connN,
		"terminalsByProtocol": byProto,
		"workerPoolSize":      utils.GlobalObject.WorkerPoolSize,
		"maxWorkerTaskLen":    utils.GlobalObject.MaxWorkerTaskLen,
		"maxMsgChanLen":       utils.GlobalObject.MaxMsgChanLen,
		"forwardWorkers":      utils.GlobalObject.ForwardWorkers,
		"forwardQueueLen":     utils.GlobalObject.ForwardQueueLen,
		"version":             utils.GlobalObject.Version,
		"name":                utils.GlobalObject.Name,
	})
}

func (s *Server) handleTerminals(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	var rows []TerminalRow
	if s.Provider != nil && s.Provider.Terminals != nil {
		q := r.URL.Query()
		expand := q.Get("expand") == "1" || strings.EqualFold(q.Get("expand"), "true")
		rows = s.Provider.Terminals(expand, q.Get("protocol"), q.Get("q"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

func (s *Server) handleApps(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	var rows []AppRow
	if s.Provider != nil && s.Provider.Apps != nil {
		rows = s.Provider.Apps(r.URL.Query().Get("q"))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rows": rows, "help": "上行/下行相对 GFEP：上行=主站→FEP，下行=FEP→主站（与终端表相反）"})
}

func logPathUnderRoot(root, rel string) (string, bool) {
	rel = filepath.ToSlash(strings.TrimSpace(rel))
	if rel == "" || strings.Contains(rel, "..") {
		return "", false
	}
	rel = strings.TrimPrefix(rel, "/")
	full := filepath.Join(root, filepath.FromSlash(rel))
	rootAbs, err1 := filepath.Abs(root)
	fullAbs, err2 := filepath.Abs(full)
	if err1 != nil || err2 != nil {
		return "", false
	}
	relPath, err := filepath.Rel(rootAbs, fullAbs)
	if err != nil || strings.HasPrefix(relPath, "..") {
		return "", false
	}
	return fullAbs, true
}

type logListEntry struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	ModTime string `json:"modTime"`
	IsDir   bool   `json:"isDir"`
}

func (s *Server) handleLogFiles(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	var entries []logListEntry
	_ = filepath.WalkDir(s.AbsLogRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(s.AbsLogRoot, path)
		if err != nil || rel == "." {
			return nil
		}
		if strings.Count(rel, string(filepath.Separator)) > 3 {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		fi, err := d.Info()
		if err != nil {
			return nil
		}
		entries = append(entries, logListEntry{
			Name:    filepath.ToSlash(rel),
			Size:    fi.Size(),
			ModTime: FormatDisplayUTC(fi.ModTime()),
			IsDir:   d.IsDir(),
		})
		return nil
	})
	writeJSON(w, http.StatusOK, map[string]any{"root": s.AbsLogRoot, "files": entries})
}

func (s *Server) handleLogDownload(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	name := r.URL.Query().Get("name")
	full, ok := logPathUnderRoot(s.AbsLogRoot, name)
	if !ok {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	st, err := os.Stat(full)
	if err != nil || st.IsDir() {
		http.NotFound(w, r)
		return
	}
	f, err := os.Open(full)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer f.Close()
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Disposition", "attachment; filename=\""+filepath.Base(full)+"\"")
	_, _ = io.Copy(w, f)
	logx.Printf("web audit: log download user remote=%s file=%s", r.RemoteAddr, name)
}

func (s *Server) handleLiveStream(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	addrF := r.URL.Query().Get("addr")
	protoF := r.URL.Query().Get("protocol")
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	ch, ok := liveSubscribe()
	if !ok {
		http.Error(w, "too many subscribers", http.StatusServiceUnavailable)
		return
	}
	defer liveUnsubscribe(ch)
	tick := time.NewTicker(25 * time.Second)
	defer tick.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-tick.C:
			_, _ = fmt.Fprintf(w, ": ping\n\n")
			fl.Flush()
		case ev, ok := <-ch:
			if !ok {
				return
			}
			if !liveLineMatchesFilters(ev.Line, addrF, protoF) {
				continue
			}
			b, _ := json.Marshal(map[string]any{"ts": FormatDisplayUTC(ev.TS), "line": ev.Line})
			_, _ = fmt.Fprintf(w, "data: %s\n\n", string(b))
			fl.Flush()
		}
	}
}

var configWritableKeys = map[string]bool{
	"LogPacketHex": true, "LogLinkLayer": true, "LogForwardEgressHex": true,
	"LogDebugClose": true, "LogConnTrace": true, "LogNetVerbose": true,
	"Timeout": true, "FirstFrameTimeoutMin": true, "PostLoginRxIdleMinutes": true,
	"ForwardWorkers": true, "ForwardQueueLen": true,
}

func (s *Server) handleConfigGet(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	raw := buildEffectiveConfigMap()
	writeJSON(w, http.StatusOK, map[string]any{
		"effective": RedactEffectiveConfig(raw),
	})
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	if s.requireAdmin(w, r) == nil {
		return
	}
	switch r.Method {
	case http.MethodGet:
		list, err := ListWebUsers()
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": list})
	case http.MethodPost:
		var body struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效 JSON"})
			return
		}
		role, err := ParseRole(body.Role)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		if err := AddWebUser(body.Username, body.Password, role); err != nil {
			writeJSON(w, userMgmtHTTPCode(err), map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodPut:
		var body struct {
			Username string  `json:"username"`
			Password *string `json:"password"`
			Role     *string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效 JSON"})
			return
		}
		if strings.TrimSpace(body.Username) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": errEmptyUsername.Error()})
			return
		}
		var newRole *roleKind
		if body.Role != nil {
			rk, err := ParseRole(*body.Role)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			newRole = &rk
		}
		pwArg := body.Password
		if pwArg != nil && *pwArg == "" {
			pwArg = nil
		}
		if newRole == nil && pwArg == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "请提供 password 或 role"})
			return
		}
		if err := UpdateWebUser(body.Username, pwArg, newRole); err != nil {
			writeJSON(w, userMgmtHTTPCode(err), map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodDelete:
		u := strings.TrimSpace(r.URL.Query().Get("username"))
		if u == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "缺少 username"})
			return
		}
		if err := DeleteWebUser(u); err != nil {
			writeJSON(w, userMgmtHTTPCode(err), map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func userMgmtHTTPCode(err error) int {
	switch {
	case errors.Is(err, errUserNotFound):
		return http.StatusNotFound
	case errors.Is(err, errLastAdmin):
		return http.StatusConflict
	default:
		return http.StatusBadRequest
	}
}

func (s *Server) handleConfigPut(w http.ResponseWriter, r *http.Request) {
	sess := s.requireAdmin(w, r)
	if sess == nil {
		return
	}
	var patch map[string]json.RawMessage
	if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效 JSON"})
		return
	}
	path := utils.GlobalObject.ConfFilePath
	data, err := os.ReadFile(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "无法读取配置文件"})
		return
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "配置文件格式错误"})
		return
	}
	for k, raw := range patch {
		if !configWritableKeys[k] {
			continue
		}
		root[k] = raw
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "序列化失败"})
		return
	}
	if err := os.WriteFile(path, out, 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "写入失败"})
		return
	}
	utils.GlobalObject.Reload()
	if utils.GlobalObject.LogDebugClose {
		zlog.CloseDebug()
	} else {
		zlog.OpenDebug()
	}
	logx.Printf("web audit: config patched by %q keys=%v", sess.Username, keysOfPatch(patch))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func keysOfPatch(m map[string]json.RawMessage) []string {
	var ks []string
	for k := range m {
		if configWritableKeys[k] {
			ks = append(ks, k)
		}
	}
	return ks
}

func (s *Server) handleLogLevel(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		if s.requireAuth(w, r) == nil {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"logDebugClose": utils.GlobalObject.LogDebugClose,
			"hint":          "LogDebugClose=true 时关闭 Debug 级别（与 zlog 一致）；修改可写配置或本接口 PUT",
		})
	case http.MethodPut:
		sess := s.requireAdmin(w, r)
		if sess == nil {
			return
		}
		var body struct {
			LogDebugClose *bool `json:"logDebugClose"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.LogDebugClose == nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "需要 logDebugClose 布尔值"})
			return
		}
		path := utils.GlobalObject.ConfFilePath
		data, err := os.ReadFile(path)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "无法读取配置"})
			return
		}
		var root map[string]json.RawMessage
		if err := json.Unmarshal(data, &root); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "配置解析失败"})
			return
		}
		raw, _ := json.Marshal(*body.LogDebugClose)
		root["LogDebugClose"] = raw
		out, _ := json.MarshalIndent(root, "", "  ")
		if err := os.WriteFile(path, out, 0644); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "写入失败"})
			return
		}
		utils.GlobalObject.Reload()
		if *body.LogDebugClose {
			zlog.CloseDebug()
		} else {
			zlog.OpenDebug()
		}
		logx.Printf("web audit: log level by %q LogDebugClose=%v", sess.Username, *body.LogDebugClose)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleBlacklistGet(w http.ResponseWriter, r *http.Request) {
	if s.requireAuth(w, r) == nil {
		return
	}
	EnsureBlacklistLoaded()
	writeJSON(w, http.StatusOK, map[string]any{"addrs": SnapshotTerminalBlacklist()})
}

func (s *Server) handleBlacklistPut(w http.ResponseWriter, r *http.Request) {
	sess := s.requireAdmin(w, r)
	if sess == nil {
		return
	}
	var body struct {
		Addrs []string `json:"addrs"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "无效 JSON"})
		return
	}
	if err := SaveTerminalBlacklist(body.Addrs); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	logx.Printf("web audit: blacklist updated by %q count=%d", sess.Username, len(body.Addrs))
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

// Routes 返回挂载了 API 与静态资源的 Handler。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/auth/login", s.handleAuthLogin)
	mux.HandleFunc("/api/auth/logout", s.handleAuthLogout)
	mux.HandleFunc("/api/auth/me", s.handleAuthMe)
	mux.HandleFunc("/api/users", s.handleUsers)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/terminals", s.handleTerminals)
	mux.HandleFunc("/api/apps", s.handleApps)
	mux.HandleFunc("/api/logs/files", s.handleLogFiles)
	mux.HandleFunc("/api/logs/download", s.handleLogDownload)
	mux.HandleFunc("/api/logs/stream", s.handleLiveStream)
	mux.HandleFunc("/api/config", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleConfigGet(w, r)
		case http.MethodPut:
			s.handleConfigPut(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/api/log-level", s.handleLogLevel)
	mux.HandleFunc("/api/blacklist", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			s.handleBlacklistGet(w, r)
		case http.MethodPut:
			s.handleBlacklistPut(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		logx.Errorf("web: embed static: %v", err)
	} else {
		mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(sub))))
	}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		b, err := staticFS.ReadFile("static/index.html")
		if err != nil {
			http.Error(w, "missing ui", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(b)
	})
	return securityHeaders(mux)
}
