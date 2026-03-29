package fep

import "sync"

// addrConnID 在线列表快照项（终端地址或主站 MSA 与 connID）。
type addrConnID struct {
	addrStr string
	connID  uint32
}

// appRegistry 后台(MSA)索引：按 connID / 按 MSA 双向映射，用于 O(1) 量级转发。
type appRegistry struct {
	mu     sync.RWMutex
	byConn map[uint32]string              // connID -> msa
	byMsa  map[string]map[uint32]struct{} // msa -> conn 集合
}

func newAppRegistry() *appRegistry {
	return &appRegistry{
		byConn: make(map[uint32]string),
		byMsa:  make(map[string]map[uint32]struct{}),
	}
}

// registerOrUpdate 与原先 list 语义一致：同 conn 同 msa 返回 false；同 conn 换 msa 会更新并返回 true。
func (a *appRegistry) registerOrUpdate(connID uint32, msaStr string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if oldMsa, ok := a.byConn[connID]; ok {
		if oldMsa == msaStr {
			return false
		}
		delete(a.byMsa[oldMsa], connID)
		if len(a.byMsa[oldMsa]) == 0 {
			delete(a.byMsa, oldMsa)
		}
	}
	a.byConn[connID] = msaStr
	if a.byMsa[msaStr] == nil {
		a.byMsa[msaStr] = make(map[uint32]struct{})
	}
	a.byMsa[msaStr][connID] = struct{}{}
	return true
}

func (a *appRegistry) removeConn(connID uint32) {
	a.mu.Lock()
	defer a.mu.Unlock()
	msa, ok := a.byConn[connID]
	if !ok {
		return
	}
	delete(a.byConn, connID)
	delete(a.byMsa[msa], connID)
	if len(a.byMsa[msa]) == 0 {
		delete(a.byMsa, msa)
	}
}

// forwardTargets msaStr=="0" 时转发所有在线后台。
func (a *appRegistry) forwardTargets(msaStr string) []uint32 {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if msaStr == "0" {
		out := make([]uint32, 0, len(a.byConn))
		for id := range a.byConn {
			out = append(out, id)
		}
		return out
	}
	set := a.byMsa[msaStr]
	if len(set) == 0 {
		return nil
	}
	out := make([]uint32, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func (a *appRegistry) snapshot() []addrConnID {
	a.mu.RLock()
	defer a.mu.RUnlock()
	out := make([]addrConnID, 0, len(a.byConn))
	for id, msa := range a.byConn {
		out = append(out, addrConnID{addrStr: msa, connID: id})
	}
	return out
}

// connKey 若 connID 在本注册表中则返回其业务键（终端 addr 或主站 MSA）。
func (a *appRegistry) connKey(connID uint32) (key string, ok bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	k, ok := a.byConn[connID]
	return k, ok
}

// tmnRegistry 终端地址索引。
type tmnRegistry struct {
	mu     sync.RWMutex
	byConn map[uint32]string
	byAddr map[string]map[uint32]struct{}
}

func newTmnRegistry() *tmnRegistry {
	return &tmnRegistry{
		byConn: make(map[uint32]string),
		byAddr: make(map[string]map[uint32]struct{}),
	}
}

func (t *tmnRegistry) removeConnLocked(connID uint32) {
	addr, ok := t.byConn[connID]
	if !ok {
		return
	}
	delete(t.byConn, connID)
	delete(t.byAddr[addr], connID)
	if len(t.byAddr[addr]) == 0 {
		delete(t.byAddr, addr)
	}
}

func (t *tmnRegistry) setAddrLocked(connID uint32, addr string) {
	if old, ok := t.byConn[connID]; ok && old != addr {
		delete(t.byAddr[old], connID)
		if len(t.byAddr[old]) == 0 {
			delete(t.byAddr, old)
		}
	}
	t.byConn[connID] = addr
	if t.byAddr[addr] == nil {
		t.byAddr[addr] = make(map[uint32]struct{})
	}
	t.byAddr[addr][connID] = struct{}{}
}

// Login 与原先 list + 扫描语义对齐。
// evictedOthers：被踢掉的其它 connID（同址重复登录等），698 场景需对其停 bridge。
// resetBridgeCur：当前连接地址变更（SupportComm 且 !SupportCasLink）时需停当前 conn 的 bridge。
func (t *tmnRegistry) Login(connID uint32, tmnStr string, supportComm, supportCasLink bool) (isNew bool, evictedOthers []uint32, resetBridgeCur bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var evicted []uint32

	if !supportComm {
		if set, ok := t.byAddr[tmnStr]; ok {
			toRemove := make([]uint32, 0, len(set))
			for id := range set {
				if id != connID {
					toRemove = append(toRemove, id)
				}
			}
			for _, id := range toRemove {
				t.removeConnLocked(id)
				evicted = append(evicted, id)
			}
		}
		if t.byConn[connID] == tmnStr {
			return false, evicted, false
		}
		t.setAddrLocked(connID, tmnStr)
		return true, evicted, false
	}

	if old, ok := t.byConn[connID]; ok && old == tmnStr {
		return false, evicted, false
	}

	resetBridgeCur = false
	if !supportCasLink {
		if old, ok := t.byConn[connID]; ok && old != tmnStr {
			delete(t.byAddr[old], connID)
			if len(t.byAddr[old]) == 0 {
				delete(t.byAddr, old)
			}
			delete(t.byConn, connID)
			resetBridgeCur = true
		}
	}
	t.setAddrLocked(connID, tmnStr)
	return true, evicted, resetBridgeCur
}

func (t *tmnRegistry) removeConn(connID uint32) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.removeConnLocked(connID)
}

// forwardTargetsAppToTmn broadcast 为 true 时向所有在线终端转发（376/NW 通配 FF、698 通配 AA 等由调用方传入）。
func (t *tmnRegistry) forwardTargetsAppToTmn(tmnStr string, broadcast bool) []uint32 {
	t.mu.RLock()
	defer t.mu.RUnlock()
	if broadcast {
		out := make([]uint32, 0, len(t.byConn))
		for id := range t.byConn {
			out = append(out, id)
		}
		return out
	}
	set := t.byAddr[tmnStr]
	if len(set) == 0 {
		return nil
	}
	out := make([]uint32, 0, len(set))
	for id := range set {
		out = append(out, id)
	}
	return out
}

func (t *tmnRegistry) snapshot() []addrConnID {
	t.mu.RLock()
	defer t.mu.RUnlock()
	out := make([]addrConnID, 0, len(t.byConn))
	for id, addr := range t.byConn {
		out = append(out, addrConnID{addrStr: addr, connID: id})
	}
	return out
}

// addrForConn O(1) 查询终端 conn 当前登记地址（供实时日志等）。
func (t *tmnRegistry) addrForConn(connID uint32) (addr string, ok bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	a, ok := t.byConn[connID]
	return a, ok
}

// msaForConn O(1) 查询主站 conn 对应 MSA（供实时日志等）。
func (a *appRegistry) msaForConn(connID uint32) (msa string, ok bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	m, ok := a.byConn[connID]
	return m, ok
}

func (t *tmnRegistry) connAddr(connID uint32) (addr string, ok bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	a, ok := t.byConn[connID]
	return a, ok
}
