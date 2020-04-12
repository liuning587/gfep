package fep

import (
	"errors"
	"sync"
)

/*
  地址映射
*/
type AddrMap struct {
	connIDs map[string][]uint32
	lock    sync.RWMutex //读写连接的读写锁
}

func NewAddrMap() *AddrMap {
	return &AddrMap{
		connIDs: make(map[string][]uint32),
	}
}

func (addrMap *AddrMap) Add(addr string, connID uint32) {
	addrMap.lock.Lock()
	defer addrMap.lock.Unlock()

	if addrMap.connIDs[addr] == nil {
		addrMap.connIDs[addr] = make([]uint32)
	}

	insert := true
	for i := 0; i < len(addrMap.connIDs[addr]); i++ {
		if addrMap.connIDs[addr][i] == connID {
			insert = false
			break
		}
	}
	if insert {
		addrMap.connIDs[addr] = append(addrMap.connIDs[addr], connID)
	}
}

func (addrMap *AddrMap) Remove(addr string, connID uint32) {
	AddrMap.lock.Lock()
	defer AddrMap.lock.Unlock()

	if AddrMap.connIDs[addr] == nil {
		return
	}

	//todo: 清理
}

func (addrMap *AddrMap) Get(addr string) ([]uint32, error) {
	AddrMap.lock.Lock()
	defer AddrMap.lock.Unlock()

	if conns, ok := addrMap.connIDs[addr]; ok {
		return conns, nil
	}
	return nil, errors.New("addr not found")
}

func (addrMap *AddrMap) Len() int {
	return len(addrMap.connIDs)
}

func (addrMap *AddrMap) Clear() {
	AddrMap.lock.Lock()
	defer AddrMap.lock.Unlock()

	//todo: 遍历list, 删除每个[]uint32
}
