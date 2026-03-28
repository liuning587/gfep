package znet

import (
	"errors"
	"gfep/internal/logx"
	"gfep/utils"
	"gfep/ziface"
	"sync"
)

// ConnManager 连接管理模块
type ConnManager struct {
	connections map[uint32]ziface.IConnection //管理的连接信息
	connLock    sync.RWMutex                  //读写连接的读写锁
}

// NewConnManager 创建一个链接管理
func NewConnManager() *ConnManager {
	return &ConnManager{
		connections: make(map[uint32]ziface.IConnection),
	}
}

// Add 添加链接
func (connMgr *ConnManager) Add(conn ziface.IConnection) {
	//保护共享资源Map 加写锁
	connMgr.connLock.Lock()
	defer connMgr.connLock.Unlock()

	//将conn连接添加到ConnMananger中
	connMgr.connections[conn.GetConnID()] = conn

	if utils.GlobalObject.LogConnTrace {
		logx.Printf("connection add to ConnManager successfully: conn num = %d", len(connMgr.connections))
	}
}

// Remove 删除连接
func (connMgr *ConnManager) Remove(conn ziface.IConnection) {
	//保护共享资源Map 加写锁
	connMgr.connLock.Lock()
	defer connMgr.connLock.Unlock()

	//删除连接信息
	delete(connMgr.connections, conn.GetConnID())

	if utils.GlobalObject.LogConnTrace {
		logx.Printf("connection Remove ConnID=%d successfully: conn num = %d", conn.GetConnID(), len(connMgr.connections))
	}
}

// Get 利用ConnID获取链接
func (connMgr *ConnManager) Get(connID uint32) (ziface.IConnection, error) {
	//保护共享资源Map 加读锁
	connMgr.connLock.RLock()
	defer connMgr.connLock.RUnlock()

	if conn, ok := connMgr.connections[connID]; ok {
		return conn, nil
	}
	return nil, errors.New("connection not found")
}

// Len 获取当前连接
func (connMgr *ConnManager) Len() int {
	connMgr.connLock.RLock()
	defer connMgr.connLock.RUnlock()
	return len(connMgr.connections)
}

// ClearConn 清除并停止所有连接
func (connMgr *ConnManager) ClearConn() {
	connMgr.connLock.Lock()
	conns := make([]ziface.IConnection, 0, len(connMgr.connections))
	for _, c := range connMgr.connections {
		conns = append(conns, c)
	}
	connMgr.connLock.Unlock()

	for _, c := range conns {
		c.Stop()
	}

	if utils.GlobalObject.LogConnTrace {
		connMgr.connLock.RLock()
		n := len(connMgr.connections)
		connMgr.connLock.RUnlock()
		logx.Printf("Clear All Connections successfully: conn num = %d", n)
	}
}
