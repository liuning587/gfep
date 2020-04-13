package znet

import (
	"errors"
	"fmt"
	"gfep/utils"
	"gfep/ziface"
	"gfep/zptl"
	"net"
	"sync"
)

type Connection struct {
	//当前Conn属于哪个Server
	TcpServer ziface.IServer
	//当前连接的socket TCP套接字
	Conn *net.TCPConn
	//当前连接的ID 也可以称作为SessionID，ID全局唯一
	ConnID uint32
	//当前连接的关闭状态
	isClosed bool
	//消息管理MsgId和对应处理方法的消息管理模块
	MsgHandler ziface.IMsgHandle
	//告知该链接已经退出/停止的channel
	ExitBuffChan chan bool
	//无缓冲管道，用于读、写两个goroutine之间的消息通信
	msgChan chan []byte
	//有缓冲管道，用于读、写两个goroutine之间的消息通信
	msgBuffChan chan []byte

	//链接属性
	property map[string]interface{}
	//保护链接属性修改的锁
	propertyLock sync.RWMutex

	//报文检测
	ptlChk *zptl.PtlChkfrm
	// //当前状态
	// status int

	// //终端/主站地址字符串
	// addr string
	// //最近一次报文接收时间
	// rtime time.Time
	// //登录时间
	// ltime time.Time
	// //心跳时间
	// htime time.Time
	// 级联终端信息
}

//创建连接的方法
func NewConntion(server ziface.IServer, conn *net.TCPConn, connID uint32, msgHandler ziface.IMsgHandle) *Connection {
	//初始化Conn属性
	c := &Connection{
		TcpServer:    server,
		Conn:         conn,
		ConnID:       connID,
		isClosed:     false,
		MsgHandler:   msgHandler,
		ExitBuffChan: make(chan bool, 1),
		msgChan:      make(chan []byte),
		msgBuffChan:  make(chan []byte, utils.GlobalObject.MaxMsgChanLen),
		property:     make(map[string]interface{}),
		ptlChk:       nil,
	}

	//将新创建的Conn添加到链接管理中
	c.TcpServer.GetConnMgr().Add(c)
	return c
}

/*
	写消息Goroutine， 用户将数据发送给客户端
*/
func (c *Connection) StartWriter() {
	fmt.Println("[Writer Goroutine is running]")
	defer fmt.Println(c.RemoteAddr().String(), "[conn Writer exit!]")

	for {
		select {
		case data := <-c.msgChan:
			//有数据要写给客户端
			if _, err := c.Conn.Write(data); err != nil {
				fmt.Println("Send Data error:, ", err, " Conn Writer exit")
				return
			}
			//fmt.Printf("Send data succ! data = %+v\n", data)
		case data, ok := <-c.msgBuffChan:
			if ok {
				//有数据要写给客户端
				if _, err := c.Conn.Write(data); err != nil {
					fmt.Println("Send Buff Data error:, ", err, " Conn Writer exit")
					return
				}
			} else {
				fmt.Println("msgBuffChan is Closed")
				break
			}
		case <-c.ExitBuffChan:
			return
		}
	}
}

func cbRecvPacket(ptype uint32, data []byte, arg interface{}) {
	c, ok := arg.(*Connection)
	if ok {
		//得到当前客户端请求的Request数据
		req := Request{
			conn: c,
			msg:  NewMsgPackage(ptype, data),
		}

		if utils.GlobalObject.WorkerPoolSize > 0 {
			//已经启动工作池机制，将消息交给Worker处理
			c.MsgHandler.SendMsgToTaskQueue(&req)
		} else {
			//从绑定好的消息和对应的处理方法中执行对应的Handle方法
			go c.MsgHandler.DoMsgHandler(&req)
		}
	} else {
		fmt.Println("arg is not Connection")
	}
}

/*
	读消息Goroutine，用于从客户端中读取数据
*/
func (c *Connection) StartReader() {
	fmt.Println("[Reader Goroutine is running]")
	defer fmt.Println(c.RemoteAddr().String(), "[conn Reader exit!]")
	defer c.Stop()

	c.ptlChk = zptl.NewPtlChkfrm(zptl.PTL_698_45, 1000, cbRecvPacket, c)
	defer c.ptlChk.Reset()
	rbuf := make([]byte, zptl.PmaxPtlFrameLen/2, zptl.PmaxPtlFrameLen/2)

	for {
		rlen, err := c.Conn.Read(rbuf)
		if err != nil {
			break
		}
		c.ptlChk.Chkfrm(rbuf[0:rlen])
	}
}

//启动连接，让当前连接开始工作
func (c *Connection) Start() {
	//1 开启用户从客户端读取数据流程的Goroutine
	go c.StartReader()
	//2 开启用于写回客户端数据流程的Goroutine
	go c.StartWriter()
	//按照用户传递进来的创建连接时需要处理的业务，执行钩子方法
	c.TcpServer.CallOnConnStart(c)
}

//停止连接，结束当前连接状态M
func (c *Connection) Stop() {
	fmt.Println("Conn Stop()...ConnID = ", c.ConnID)
	//如果当前链接已经关闭
	if c.isClosed == true {
		return
	}
	c.isClosed = true

	//如果用户注册了该链接的关闭回调业务，那么在此刻应该显示调用
	c.TcpServer.CallOnConnStop(c)

	// 关闭socket链接
	c.Conn.Close()
	//关闭Writer
	c.ExitBuffChan <- true

	//将链接从连接管理器中删除
	c.TcpServer.GetConnMgr().Remove(c)

	//关闭该链接全部管道
	close(c.ExitBuffChan)
	close(c.msgBuffChan)
}

//从当前连接获取原始的socket TCPConn
func (c *Connection) GetTCPConnection() *net.TCPConn {
	return c.Conn
}

//获取当前连接ID
func (c *Connection) GetConnID() uint32 {
	return c.ConnID
}

//获取远程客户端地址信息
func (c *Connection) RemoteAddr() net.Addr {
	return c.Conn.RemoteAddr()
}

//直接将Message数据发送数据给远程的TCP客户端
func (c *Connection) SendMsg(data []byte) error {
	if c.isClosed == true {
		return errors.New("Connection closed when send msg")
	}

	//写回客户端
	c.msgChan <- data

	return nil
}

func (c *Connection) SendBuffMsg(data []byte) error {
	if c.isClosed == true {
		return errors.New("Connection closed when send buff msg")
	}

	//写回客户端
	c.msgBuffChan <- data

	return nil
}

//直接将Message数据发送数据给远程的TCP客户端
func (c *Connection) SendMsgByConnID(connID uint32, data []byte) error {

	conn, err := c.TcpServer.GetConnMgr().Get(connID)
	if err != nil {
		return errors.New("Connection closed when send msg")
	}

	return conn.SendBuffMsg(data)
}

//设置链接属性
func (c *Connection) SetProperty(key string, value interface{}) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	c.property[key] = value
}

//获取链接属性
func (c *Connection) GetProperty(key string) (interface{}, error) {
	c.propertyLock.RLock()
	defer c.propertyLock.RUnlock()

	if value, ok := c.property[key]; ok {
		return value, nil
	} else {
		return nil, errors.New("no property found")
	}
}

//移除链接属性
func (c *Connection) RemoveProperty(key string) {
	c.propertyLock.Lock()
	defer c.propertyLock.Unlock()

	delete(c.property, key)
}
