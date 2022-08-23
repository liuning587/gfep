package bridge

import (
	"errors"
	"gfep/timewriter"
	"gfep/utils"
	"gfep/zptl"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

var (
	logBridge *log.Logger
)

// init bridge
func init() {
	logBridge = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "bridge",
	}, "", log.LstdFlags)
}

type connStatus int

const (
	_            connStatus = iota
	unConnect               //未连接
	tcpConnectOK            //已连接
	loginOK                 //已登录
	exitRequest             //请求退出
)

//SendMsgHandler 消息回调
type SendMsgHandler func([]byte)

// Conn 桥接信息
type Conn struct {
	host          string         // 桥接主站IP:Port
	addr          []byte         // 终端地址
	ptype         uint32         // 协议类型
	heartCycle    time.Duration  // 心跳周期
	sendMsg       SendMsgHandler // 消息回调
	cStatus       connStatus     // 当前在线状态
	loginTime     time.Time      // 登录时间
	heartTime     time.Time      // 心跳时间
	heartUnAckCnt int32          // 未确认心跳数量
	conn          net.Conn       // socket
	loginAckSig   chan struct{}  // 登录确认信号
	lock          sync.Mutex     // 登录确认信号锁
	isExitRequest bool           //是否退出
	// chan
}

// NewConn 新建桥接连接
func NewConn(host string, addr []byte, ptype uint32, heart time.Duration, sendMsg SendMsgHandler) *Conn {
	return &Conn{
		host:          host,
		addr:          addr,
		ptype:         ptype,
		heartCycle:    heart,
		sendMsg:       sendMsg,
		cStatus:       unConnect,
		isExitRequest: false,
	}
}

// Start 启动桥接
func (c *Conn) Start() {
	go c.serve()
}

// Stop 终止桥接
func (c *Conn) Stop() {
	c.isExitRequest = true
	c.cStatus = exitRequest
	c.disConnectServer()
}

// Send 发送报文
func (c *Conn) Send(buf []byte) error {
	if c.cStatus != loginOK {
		logBridge.Printf("BT(NotLOGIN): % X\n", buf)
		return errors.New("not login ok")
	}

	if c.conn != nil {
		_, err := c.conn.Write(buf)
		if err != nil {
			logBridge.Printf("BT(ERR): % X\n", buf)
			c.disConnectServer()
			return err
		}
		logBridge.Printf("BT: % X\n", buf)
	} else {
		logBridge.Printf("BT(LOST): % X\n", buf)
	}

	return nil
}

func (c *Conn) connectServer() error {
	c.disConnectServer()
	conn, err := net.Dial("tcp", c.host)
	if err != nil {
		return err
	}
	c.conn = conn
	c.cStatus = tcpConnectOK
	logBridge.Println("connect " + c.host + " OK")

	go c.recv()

	return nil
}

func (c *Conn) disConnectServer() {
	if c.conn != nil {
		c.logout()
		c.cStatus = unConnect
		if c.conn != nil {
			_ = c.conn.Close()
			c.conn = nil
		}
		c.loginTime = time.Time{}
		c.heartTime = time.Time{}
	}
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
	if len(p) > 0 {
		_, err := c.conn.Write(p)
		if err != nil {
			logBridge.Printf("BL(ERR): % X\n", p)
			return err
		}
		logBridge.Printf("BL: % X\n", p)
	}

	c.lock.Lock()
	c.loginAckSig = make(chan struct{})
	c.lock.Unlock()

	var err error
	select {
	case <-time.After(time.Second * 5):
		err = errors.New("timeout")
	case <-c.loginAckSig:
		c.cStatus = loginOK
		c.loginTime = time.Now()
	}

	c.lock.Lock()
	close(c.loginAckSig)
	c.loginAckSig = nil
	c.lock.Unlock()

	return err
}

func (c *Conn) heartbeat() error {
	var p []byte

	if c.conn == nil {
		return errors.New("conn lost")
	}

	if c.ptype&zptl.PTL_1376_1 != 0 {
		p = zptl.Ptl1376_1BuildPacket(1, c.addr)
	} else if c.ptype&zptl.PTL_698_45 != 0 {
		p = zptl.Ptl698_45BuildPacket(1, c.addr)
	} else if c.ptype&zptl.PTL_NW != 0 {
		p = zptl.PtlNwBuildPacket(1, c.addr)
	}
	if len(p) > 0 {
		_, err := c.conn.Write(p)
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
	if c.cStatus != loginOK {
		return
	}
	//todo: 发送退出帧
}

func cbRecvPacket(ptype uint32, data []byte, arg interface{}) {
	c, ok := arg.(*Conn)
	if ok {
		logBridge.Printf("BR: % X\n", data)
		//解析报文类型, 若为登录、心跳确认帧直接截获
		if c.packetType(ptype, data) == 1 {
			if c.cStatus == loginOK {
				c.heartTime = time.Now()
				atomic.StoreInt32(&c.heartUnAckCnt, 0)
			} else {
				if c.cStatus == tcpConnectOK {
					c.lock.Lock()
					if c.loginAckSig != nil {
						c.loginAckSig <- struct{}{}
					}
					c.lock.Unlock()
				}
			}
		} else {
			c.sendMsg(data) // 把数据发送给终端
		}
	}
}

func (c *Conn) recv() {
	ptlChk := zptl.NewChkfrm(c.ptype, 1000, cbRecvPacket, c)
	rbuf := make([]byte, 2048)

	for {
		rlen, err := c.conn.Read(rbuf)
		if err != nil {
			logBridge.Printf("read err: %s\n", err)
			c.disConnectServer()
			break
		}
		// logBridge.Printf("BRAW: % X\n", rbuf[0:rlen])
		ptlChk.Chkfrm(rbuf[0:rlen])
	}
}

func (c *Conn) serve() {
	errCntConnect := 0 //连接错误次数

	for {
		<-time.After(time.Second)

		// 1. tcp connect
		err := c.connectServer()
		if err != nil {
			if errCntConnect < 12 { //120s
				errCntConnect++
			}
			<-time.After(time.Second * time.Duration(errCntConnect*10))
			continue
		}
		errCntConnect = 0

		// 2. login
		err = c.login()
		if err != nil {
			c.disConnectServer()
			<-time.After(time.Second * 10)
			continue
		}
		logBridge.Printf("[% X]login ok", c.addr)

		// 3. hearbeat
		atomic.StoreInt32(&c.heartUnAckCnt, 0)
		for {
			sec := c.heartCycle / time.Second
			for ; sec > 0; sec -= 10 {
				<-time.After(time.Second * 10)
				if c.cStatus != loginOK || c.isExitRequest {
					break //掉线后立即重连
				}
			}
			if c.cStatus != loginOK || c.isExitRequest {
				break
			}
			if atomic.AddInt32(&c.heartUnAckCnt, 1) > 3 {
				break //3次心跳错误
			}
			_ = c.heartbeat()
		}

		//4. 断开连接
		c.disConnectServer()
		if c.isExitRequest {
			logBridge.Printf("[% X]exit", c.addr)
			return
		}
		logBridge.Printf("[% X]logout", c.addr)
	}
}
