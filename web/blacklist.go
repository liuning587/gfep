package web

import (
	"encoding/json"
	"gfep/internal/logx"
	"gfep/utils"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

type blacklistFile struct {
	Addrs []string `json:"addrs"`
}

var (
	blacklistMu sync.RWMutex
	blacklist   map[string]struct{}
)

func blacklistPath() string {
	dir := filepath.Dir(utils.GlobalObject.ConfFilePath)
	if dir == "" || dir == "." {
		dir = "conf"
	}
	return filepath.Join(dir, "terminal_blacklist.json")
}

func normalizeBlacklistKey(s string) string {
	return strings.TrimSpace(s)
}

// ReloadTerminalBlacklist 从磁盘加载黑名单。
func ReloadTerminalBlacklist() error {
	p := blacklistPath()
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			blacklistMu.Lock()
			blacklist = make(map[string]struct{})
			blacklistMu.Unlock()
			return nil
		}
		return err
	}
	var f blacklistFile
	if err := json.Unmarshal(data, &f); err != nil {
		return err
	}
	next := make(map[string]struct{}, len(f.Addrs))
	for _, a := range f.Addrs {
		k := normalizeBlacklistKey(a)
		if k != "" {
			next[k] = struct{}{}
		}
	}
	blacklistMu.Lock()
	blacklist = next
	blacklistMu.Unlock()
	return nil
}

// TerminalAddrBlacklisted 终端业务地址是否在黑名单中（供 fep 鉴权路径调用）。
func TerminalAddrBlacklisted(tmnStr string) bool {
	k := normalizeBlacklistKey(tmnStr)
	if k == "" {
		return false
	}
	blacklistMu.RLock()
	defer blacklistMu.RUnlock()
	if blacklist == nil {
		return false
	}
	_, ok := blacklist[k]
	return ok
}

// SnapshotTerminalBlacklist 返回排序后的地址列表（API 用）。
func SnapshotTerminalBlacklist() []string {
	blacklistMu.RLock()
	defer blacklistMu.RUnlock()
	if len(blacklist) == 0 {
		return nil
	}
	out := make([]string, 0, len(blacklist))
	for a := range blacklist {
		out = append(out, a)
	}
	sort.Strings(out)
	return out
}

// SaveTerminalBlacklist 持久化并 reload。
func SaveTerminalBlacklist(addrs []string) error {
	p := blacklistPath()
	seen := make(map[string]struct{})
	uniq := make([]string, 0, len(addrs))
	for _, a := range addrs {
		k := normalizeBlacklistKey(a)
		if k == "" {
			continue
		}
		if _, ok := seen[k]; ok {
			continue
		}
		seen[k] = struct{}{}
		uniq = append(uniq, k)
	}
	sort.Strings(uniq)
	payload, err := json.MarshalIndent(blacklistFile{Addrs: uniq}, "", "  ")
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
	return ReloadTerminalBlacklist()
}

// EnsureBlacklistLoaded 懒加载黑名单（首次检查前调用）。
func EnsureBlacklistLoaded() {
	blacklistMu.RLock()
	ready := blacklist != nil
	blacklistMu.RUnlock()
	if ready {
		return
	}
	if err := ReloadTerminalBlacklist(); err != nil {
		logx.Errorf("web blacklist: load %s: %v", blacklistPath(), err)
	}
}
