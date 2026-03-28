package bridge

import (
	"context"
	"errors"
	"gfep/timewriter"
	"gfep/utils"
	"gfep/zptl"
	"log"
	"net"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"
)

var (
	logBridge *log.Logger
	dialer    = net.Dialer{Timeout: 15 * time.Second}
)

// init bridge
func init() {
	logBridge = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "bridge",
	}, "", log.LstdFlags|log.Lmicroseconds)
}

type connStatus int32

const (
	unConnect connStatus = iota
	tcpConnectOK
	loginOK
	exitRequest
)

// SendMsgHandler 消息回调
type SendMsgHandler func([]byte)

// Conn 桥接信息
type Conn struct {
	mu          sync.Mutex
	host        string // 桥接主站IP:Port
	addr        []byte
	ptype       uint32
	heartCycle  time.Duration
	sendMsg     SendMsgHandler
	status      atomic.Int32 // connStatus
	loginTime   time.Time
	heartTime   time.Time
	heartUnAck  int32
	conn        net.Conn
	loginWaitCh chan struct{}      // 容量 1，仅通知，不 close，避免与晚到 ACK 竞态
	loginCancel context.CancelFunc // 在 disConnectServer 时取消，登录 select 不必空等 5s
	downSem     chan struct{}      // 限制异步下行回调并发
	stopped     chan struct{}
	stopOnce    sync.Once
	isExit      atomic.Bool
}

// NewConn 新建桥接连接
func NewConn(host string, addr []byte, ptype uint32, heart time.Duration, sendMsg SendMsgHandler) *Conn {
	return &Conn{
		host:       host,
		addr:       addr,
		ptype:      ptype,
		heartCycle: heart,
		sendMsg:    sendMsg,
		stopped:    make(chan struct{}),
		downSem:    make(chan struct{}, 64),
	}
}

// Start 启动桥接
func (c *Conn) Start() {
	go c.serve()
}

// Stop 终止桥接
func (c *Conn) Stop() {
	c.isExit.Store(true)
	c.status.Store(int32(exitRequest))
	c.stopOnce.Do(func() { close(c.stopped) })
	c.disConnectServer()
}

// sleepInterruptible 可被 Stop 打断的等待；返回 false 表示应结束 serve。
func (c *Conn) sleepInterruptible(d time.Duration) bool {
	if d <= 0 {
		return true
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-c.stopped:
		return false
	}
}

// Send 发送报文
func (c *Conn) Send(buf []byte) error {
	if connStatus(c.status.Load()) != loginOK {
		logBridge.Printf("BT(NotLOGIN): % X\n", buf)
		return errors.New("not login ok")
	}

	c.mu.Lock()
	nc := c.conn
	c.mu.Unlock()

	if nc == nil {
		logBridge.Printf("BT(LOST): % X\n", buf)
		return nil
	}
	_, err := nc.Write(buf)
	if err != nil {
		logBridge.Printf("BT(ERR): % X\n", buf)
		c.disConnectServer()
		return err
	}
	logBridge.Printf("BT: % X\n", buf)
	return nil
}

func (c *Conn) connectServer() error {
	c.disConnectServer()
	conn, err := dialer.Dial("tcp", c.host)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.conn = conn
	c.mu.Unlock()
	c.status.Store(int32(tcpConnectOK))
	logBridge.Println("connect " + c.host + " OK")

	go c.recv()

	return nil
}

func (c *Conn) disConnectServer() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.conn == nil {
		return
	}
	if c.loginCancel != nil {
		c.loginCancel()
		c.loginCancel = nil
	}
	c.logout()
	c.status.Store(int32(unConnect))
	_ = c.conn.Close()
	c.conn = nil
	c.loginWaitCh = nil
	c.loginTime = time.Time{}
	c.heartTime = time.Time{}
}

func (c *Conn) login() error {
	var p []byte

	if c.ptype&zptl.PTL_1376_1 != 0 {
		p = zptl.Ptl1376_1BuildPacket(0, c.addr)
	} else if c.ptype&zptl.PTL_698_45 != 0 {
		p = zptl.Ptl698_45BuildPacket(0, c.addr)
	} else if c.ptype&zptl.PTL_NW != 0 {
		p = zptl.PtlNwBuildPacket(0, c.addr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c.mu.Lock()
	c.loginCancel = cancel
	nc := c.conn
	myCh := make(chan struct{}, 1)
	c.loginWaitCh = myCh
	c.mu.Unlock()

	defer func() {
		cancel()
		c.mu.Lock()
		c.loginCancel = nil
		c.mu.Unlock()
		c.clearLoginWait(myCh)
	}()

	if len(p) > 0 {
		if nc == nil {
			return errors.New("conn lost")
		}
		_, err := nc.Write(p)
		if err != nil {
			logBridge.Printf("BL(ERR): % X\n", p)
			return err
		}
		logBridge.Printf("BL: % X\n", p)
	}

	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()

	var err error
	select {
	case <-ctx.Done():
		err = errors.New("disconnected")
	case <-c.stopped:
		err = errors.New("stopped")
	case <-timer.C:
		err = errors.New("timeout")
	case <-myCh:
		c.status.Store(int32(loginOK))
		c.mu.Lock()
		c.loginTime = time.Now()
		c.mu.Unlock()
	}
	return err
}

func (c *Conn) clearLoginWait(ch chan struct{}) {
	c.mu.Lock()
	if c.loginWaitCh == ch {
		c.loginWaitCh = nil
	}
	c.mu.Unlock()
}

func (c *Conn) heartbeat() error {
	c.mu.Lock()
	nc := c.conn
	c.mu.Unlock()
	if nc == nil {
		return errors.New("conn lost")
	}

	var p []byte
	if c.ptype&zptl.PTL_1376_1 != 0 {
		p = zptl.Ptl1376_1BuildPacket(1, c.addr)
	} else if c.ptype&zptl.PTL_698_45 != 0 {
		p = zptl.Ptl698_45BuildPacket(1, c.addr)
	} else if c.ptype&zptl.PTL_NW != 0 {
		p = zptl.PtlNwBuildPacket(1, c.addr)
	}
	if len(p) > 0 {
		_, err := nc.Write(p)
		if err != nil {
			logBridge.Printf("BH(ERR): % X\n", p)
			return err
		}
		logBridge.Printf("BH: % X\n", p)
	}
	return nil
}

// packetType 判断报文类型
// @retval 0 : 其它报文
// @retval 1 : 登录/心跳/退出确认帧
func (c *Conn) packetType(ptype uint32, p []byte) uint8 {
	if len(p) < 4 {
		return 0
	}
	if ptype&zptl.PTL_1376_1 != 0 {
		return 0
	} else if ptype&zptl.PTL_698_45 != 0 {
		if p[3] == 0x01 {
			return 1
		}
		return 0
	} else if ptype&zptl.PTL_NW != 0 {
		return 0
	}
	return 0
}

func (c *Conn) logout() {
	if connStatus(c.status.Load()) != loginOK {
		return
	}
	nc := c.conn
	if nc == nil {
		return
	}
	var p []byte
	if c.ptype&zptl.PTL_1376_1 != 0 {
		p = zptl.Ptl1376_1BuildPacket(2, c.addr)
	} else if c.ptype&zptl.PTL_698_45 != 0 {
		p = zptl.Ptl698_45BuildPacket(2, c.addr)
	} else if c.ptype&zptl.PTL_NW != 0 {
		p = zptl.PtlNwBuildPacket(2, c.addr)
	}
	if len(p) == 0 {
		return
	}
	_ = nc.SetWriteDeadline(time.Now().Add(3 * time.Second))
	_, werr := nc.Write(p)
	_ = nc.SetWriteDeadline(time.Time{})
	if werr != nil {
		logBridge.Printf("logout write: %v\n", werr)
		return
	}
	logBridge.Printf("Blogout: % X\n", p)
}

func (c *Conn) signalLoginAck() {
	c.mu.Lock()
	ch := c.loginWaitCh
	c.mu.Unlock()
	if ch == nil {
		return
	}
	select {
	case ch <- struct{}{}:
	default:
	}
}

func (c *Conn) enqueueDownlink(data []byte) {
	if c.sendMsg == nil || c.isExit.Load() {
		return
	}
	payload := make([]byte, len(data))
	copy(payload, data)
	select {
	case c.downSem <- struct{}{}:
		go func() {
			defer func() { <-c.downSem }()
			c.sendMsg(payload)
		}()
	default:
		logBridge.Printf("BR: downlink saturated, drop len=%d\n", len(data))
	}
}

func cbRecvPacket(ptype uint32, data []byte, arg interface{}) {
	c, ok := arg.(*Conn)
	if !ok {
		return
	}
	logBridge.Printf("BR: % X\n", data)
	if c.packetType(ptype, data) == 1 {
		switch connStatus(c.status.Load()) {
		case loginOK:
			c.mu.Lock()
			c.heartTime = time.Now()
			c.mu.Unlock()
			atomic.StoreInt32(&c.heartUnAck, 0)
		case tcpConnectOK:
			c.signalLoginAck()
		}
		return
	}
	c.enqueueDownlink(data)
}

func (c *Conn) recv() {
	defer func() {
		if p := recover(); p != nil {
			logBridge.Printf("recv() panic recover! p: %v\n", p)
			debug.PrintStack()
		}
	}()
	ptlChk := zptl.NewChkfrm(c.ptype, 1000, cbRecvPacket, c)
	rbuf := make([]byte, 2048)

	for {
		c.mu.Lock()
		nc := c.conn
		c.mu.Unlock()
		if nc == nil {
			break
		}
		rlen, err := nc.Read(rbuf)
		if err != nil {
			logBridge.Printf("read err: %s\n", err)
			c.disConnectServer()
			break
		}
		ptlChk.Chkfrm(rbuf[0:rlen])
	}
}

func (c *Conn) serve() {
	errCntConnect := 0

	for {
		if !c.sleepInterruptible(time.Second) {
			c.disConnectServer()
			logBridge.Printf("[% X]exit", c.addr)
			return
		}
		if c.isExit.Load() {
			c.disConnectServer()
			logBridge.Printf("[% X]exit", c.addr)
			return
		}

		err := c.connectServer()
		if err != nil {
			if errCntConnect < 12 {
				errCntConnect++
			}
			backoff := time.Second * time.Duration(errCntConnect*10)
			if !c.sleepInterruptible(backoff) {
				c.disConnectServer()
				logBridge.Printf("[% X]exit", c.addr)
				return
			}
			continue
		}
		errCntConnect = 0

		err = c.login()
		if err != nil {
			c.disConnectServer()
			if !c.sleepInterruptible(10 * time.Second) {
				logBridge.Printf("[% X]exit", c.addr)
				return
			}
			continue
		}
		logBridge.Printf("[% X]login ok", c.addr)

		atomic.StoreInt32(&c.heartUnAck, 0)
		for {
			sec := int64(c.heartCycle / time.Second)
			for sec > 0 {
				step := int64(10)
				if sec < 10 {
					step = sec
					sec = 0
				} else {
					sec -= 10
				}
				if !c.sleepInterruptible(time.Duration(step) * time.Second) {
					c.disConnectServer()
					logBridge.Printf("[% X]exit", c.addr)
					return
				}
				if connStatus(c.status.Load()) != loginOK || c.isExit.Load() {
					break
				}
			}
			if connStatus(c.status.Load()) != loginOK || c.isExit.Load() {
				break
			}
			if atomic.AddInt32(&c.heartUnAck, 1) > 3 {
				break
			}
			_ = c.heartbeat()
		}

		c.disConnectServer()
		if c.isExit.Load() {
			logBridge.Printf("[% X]exit", c.addr)
			return
		}
		logBridge.Printf("[% X]logout", c.addr)
	}
}
