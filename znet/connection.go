package znet

import (
	"errors"
	"gfep/bridge"
	"gfep/internal/logx"
	"gfep/utils"
	"gfep/ziface"
	"gfep/zptl"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

// buffPayload 异步写出：n 为待写切片；slab 非空表示来自 sendBuffPool，Write 后须归还。
type buffPayload struct {
	n    []byte
	slab *[]byte
}

var sendBuffPool = sync.Pool{
	New: func() any {
		b := make([]byte, zptl.PmaxPtlFrameLen)
		return &b
	},
}

func cloneToBuffPayload(data []byte) buffPayload {
	if len(data) == 0 {
		return buffPayload{}
	}
	if len(data) <= zptl.PmaxPtlFrameLen {
		pslab := sendBuffPool.Get().(*[]byte)
		slab := *pslab
		copy(slab[:len(data)], data)
		return buffPayload{n: slab[:len(data)], slab: pslab}
	}
	b := make([]byte, len(data))
	copy(b, data)
	return buffPayload{n: b, slab: nil}
}

func releaseBuffPayload(item buffPayload) {
	if item.slab != nil {
		slab := *item.slab
		*item.slab = slab[:cap(slab)]
		sendBuffPool.Put(item.slab)
	}
}

// Connection connection
type Connection struct {
	//当前Conn属于哪个Server
	TCPServer ziface.IServer
	//当前连接的socket TCP套接字
	Conn *net.TCPConn
	//当前连接的ID 也可以称作为SessionID，ID全局唯一
	ConnID uint32
	// stopped 为 true 表示 Stop 已执行（幂等）。
	stopped atomic.Bool
	// needStop 为 true 表示业务要求结束读循环。
	needStop atomic.Bool
	//消息管理MsgId和对应处理方法的消息管理模块
	MsgHandler ziface.IMsgHandle
	//告知该链接已经退出/停止的channel
	ExitBuffChan chan bool
	//无缓冲管道，用于读、写两个goroutine之间的消息通信
	msgChan chan buffPayload
	//有缓冲管道，用于读、写两个goroutine之间的消息通信
	msgBuffChan         chan buffPayload
	msgBuffChanIsClosed bool

	//链接属性
	// property map[string]interface{}
	//保护链接属性修改的锁
	propertyLock sync.RWMutex

	//链接属性: 先不用map节约资源
	status int          //当前状态
	addr   string       //终端/主站地址字符串
	ctime  time.Time    //连接时间
	ltime  time.Time    //登录时间
	htime  time.Time    //心跳时间
	rtime  time.Time    //最近一次报文接收时间
	binfo  *bridge.Conn //桥接信息
	// 级联终端信息
}

// NewConntion 创建连接的方法
func NewConntion(server ziface.IServer, conn *net.TCPConn, connID uint32, msgHandler ziface.IMsgHandle) *Connection {
	//初始化Conn属性
	c := &Connection{
		TCPServer:           server,
		Conn:                conn,
		ConnID:              connID,
		MsgHandler:          msgHandler,
		ExitBuffChan:        make(chan bool, 1),
		msgChan:             make(chan buffPayload),
		msgBuffChan:         make(chan buffPayload, utils.GlobalObject.MaxMsgChanLen),
		msgBuffChanIsClosed: false,
		// property:     make(map[string]interface{}),
		binfo: nil,
	}

	//将新创建的Conn添加到链接管理中
	c.TCPServer.GetConnMgr().Add(c)
	return c
}

// StartWriter 写消息Goroutine， 用户将数据发送给客户端
func (c *Connection) StartWriter() {
	// fmt.Println("[Writer Goroutine is running]")
	// defer fmt.Println(c.RemoteAddr().String(), "[conn Writer exit!]")

	for {
		select {
		case item := <-c.msgChan:
			_, err := c.Conn.Write(item.n)
			releaseBuffPayload(item)
			if err != nil {
				_ = c.Conn.Close()
				logx.Errorf("Send Data error: %v Conn Writer exit", err)
				return
			}
		case item, ok := <-c.msgBuffChan:
			if ok {
				_, err := c.Conn.Write(item.n)
				releaseBuffPayload(item)
				if err != nil {
					c.closeMsgBuffChan()
					_ = c.Conn.Close()
					logx.Errorf("Send Buff Data error: %v Conn Writer exit", err)
					return
				}
			} else if utils.GlobalObject.LogNetVerbose {
				logx.Println("msgBuffChan is Closed")
			}

		case <-c.ExitBuffChan:
			return
		}
	}
}

func cbRecvPacket(ptype uint32, data []byte, arg interface{}) {
	c, ok := arg.(*Connection)
	if ok {
		// data 指向 Chkfrm 内部缓冲，异步 worker 可能晚于下一帧解析执行，必须拷贝。
		payload := make([]byte, len(data))
		copy(payload, data)
		req := Request{
			conn: c,
			msg:  NewMsgPackage(ptype, payload),
		}
		c.MsgHandler.SendMsgToTaskQueue(&req)
	} else {
		logx.Errorf("arg is not Connection")
	}
}

// StartReader 读消息Goroutine，用于从客户端中读取数据
func (c *Connection) StartReader() {
	// fmt.Println("[Reader Goroutine is running]")
	// defer fmt.Println(c.RemoteAddr().String(), "[conn Reader exit!]")

	ptlChk := zptl.NewChkfrm(zptl.PTL_698_45|zptl.PTL_NW|zptl.PTL_1376_1, 1000, cbRecvPacket, c)
	rbuf := make([]byte, zptl.PmaxPtlFrameLen/2)

	for {
		rlen, err := c.Conn.Read(rbuf)
		if err != nil {
			break
		}
		ptlChk.Chkfrm(rbuf[0:rlen])
		if c.needStop.Load() {
			break
		}
	}
	c.Stop()
}

// Start 启动连接，让当前连接开始工作
func (c *Connection) Start() {
	//按照用户传递进来的创建连接时需要处理的业务，执行钩子方法
	c.TCPServer.CallOnConnStart(c)
	//1 开启用户从客户端读取数据流程的Goroutine
	go c.StartReader()
	//2 开启用于写回客户端数据流程的Goroutine
	go c.StartWriter()
}

func (c *Connection) closeMsgBuffChan() {
	c.propertyLock.Lock()
	if !c.msgBuffChanIsClosed {
		c.msgBuffChanIsClosed = true
		close(c.msgBuffChan)
		c.msgBuffChan = nil
	}
	c.propertyLock.Unlock()
}

// Stop 停止连接，结束当前连接状态（幂等，可并发调用）。
func (c *Connection) Stop() {
	if !c.stopped.CompareAndSwap(false, true) {
		return
	}

	c.TCPServer.CallOnConnStop(c)

	_ = c.Conn.Close()
	c.ExitBuffChan <- true

	c.TCPServer.GetConnMgr().Remove(c)

	close(c.ExitBuffChan)
	c.closeMsgBuffChan()
}

// IsStop 是否停止连接
func (c *Connection) IsStop() bool {
	return c.needStop.Load() || c.stopped.Load()
}

// NeedStop 需要停止连接
func (c *Connection) NeedStop() {
	c.needStop.Store(true)
}

// GetTCPConnection 从当前连接获取原始的socket TCPConn
func (c *Connection) GetTCPConnection() *net.TCPConn {
	return c.Conn
}

// GetConnID 获取当前连接ID
func (c *Connection) GetConnID() uint32 {
	return c.ConnID
}

// RemoteAddr 获取远程客户端地址信息
func (c *Connection) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

// SendMsg 直接将Message数据发送数据给远程的TCP客户端
func (c *Connection) SendMsg(data []byte) (err error) {
	if c.stopped.Load() {
		return errors.New("Connection closed when send msg")
	}

	defer func() {
		if recover() != nil {
			err = errors.New("Connection closed when send msg")
		}
	}()

	c.msgChan <- cloneToBuffPayload(data)
	return nil
}

// SendBuffMsg 直接将Message数据发送数据给远程的TCP客户端
func (c *Connection) SendBuffMsg(data []byte) (err error) {
	c.propertyLock.RLock()
	closed := c.msgBuffChanIsClosed
	ch := c.msgBuffChan
	c.propertyLock.RUnlock()
	if closed || ch == nil {
		return errors.New("Connection closed when send buff msg")
	}

	defer func() {
		if recover() != nil {
			err = errors.New("Connection closed when send buff msg")
		}
	}()

	bp := cloneToBuffPayload(data)
	select {
	case ch <- bp:
		return nil
	default:
		releaseBuffPayload(bp)
		return errors.New("send buffer full")
	}
}

// SendMsgByConnID 直接将Message数据发送数据给远程的TCP客户端
func (c *Connection) SendMsgByConnID(connID uint32, data []byte) error {
	conn, err := c.TCPServer.GetConnMgr().Get(connID)
	if err != nil {
		return errors.New("Connection closed when send msg")
	}

	return conn.SendBuffMsg(data)
}

// SetProperty 设置链接属性
func (c *Connection) SetProperty(key string, value interface{}) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	// c.property[key] = value
	switch key {
	case "status":
		if v, ok := value.(int); ok {
			c.status = v
		}
	case "addr":
		if v, ok := value.(string); ok {
			c.addr = v
		}
	case "ctime":
		if v, ok := value.(time.Time); ok {
			c.ctime = v
		}
	case "ltime":
		if v, ok := value.(time.Time); ok {
			c.ltime = v
		}
	case "htime":
		if v, ok := value.(time.Time); ok {
			c.htime = v
		}
	case "rtime":
		if v, ok := value.(time.Time); ok {
			c.rtime = v
		}
	case "bridge":
		if v, ok := value.(*bridge.Conn); ok {
			c.binfo = v
		}
	}
}

// GetProperty 获取链接属性
func (c *Connection) GetProperty(key string) (interface{}, error) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	// if value, ok := c.property[key]; ok {
	// 	return value, nil
	// }
	switch key {
	case "status":
		return c.status, nil
	case "addr":
		return c.addr, nil
	case "ctime":
		return c.ctime, nil
	case "ltime":
		return c.ltime, nil
	case "htime":
		return c.htime, nil
	case "rtime":
		return c.rtime, nil
	case "bridge":
		if c.binfo == nil {
			return nil, errors.New("binfo is nil")
		}
		return c.binfo, nil
	}
	return nil, errors.New("no property found")
}

// FastTouchRx 仅更新最近接收时间（单锁，热路径用）。
func (c *Connection) FastTouchRx(t time.Time) {
	c.propertyLock.Lock()
	c.rtime = t
	c.propertyLock.Unlock()
}

// FastTouchRxAndStatus 合并更新接收时间与角色状态。
func (c *Connection) FastTouchRxAndStatus(t time.Time, status int) {
	c.propertyLock.Lock()
	c.rtime = t
	c.status = status
	c.propertyLock.Unlock()
}

// FastSetRoutingStatus 仅更新 status。
func (c *Connection) FastSetRoutingStatus(status int) {
	c.propertyLock.Lock()
	c.status = status
	c.propertyLock.Unlock()
}

// FastSetRoutingAddr 仅更新 addr。
func (c *Connection) FastSetRoutingAddr(addr string) {
	c.propertyLock.Lock()
	c.addr = addr
	c.propertyLock.Unlock()
}

// FastSetLtime 登录时间。
func (c *Connection) FastSetLtime(t time.Time) {
	c.propertyLock.Lock()
	c.ltime = t
	c.propertyLock.Unlock()
}

// FastSetHtime 心跳时间。
func (c *Connection) FastSetHtime(t time.Time) {
	c.propertyLock.Lock()
	c.htime = t
	c.propertyLock.Unlock()
}

// FastGetRouting 读取 status、addr（单锁）。
func (c *Connection) FastGetRouting() (status int, addr string) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()
	return c.status, c.addr
}

// RemoveProperty 移除链接属性
func (c *Connection) RemoveProperty(key string) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	switch key {
	case "bridge":
		c.binfo = nil
	}
}
