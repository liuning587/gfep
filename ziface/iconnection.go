package ziface

import "net"

// IConnection 定义连接接口
type IConnection interface {
	//Start 启动连接，让当前连接开始工作
	Start()
	//Stop 停止连接，结束当前连接状态M
	Stop()
	IsStop() bool
	NeedStop()

	//GetTCPConnection 从当前连接获取原始的socket TCPConn
	GetTCPConnection() *net.TCPConn
	//GetConnID 获取当前连接ID
	GetConnID() uint32
	//RemoteAddr 获取远程客户端地址信息
	RemoteAddr() net.Addr

	//SendMsg 直接将数据发送数据给远程的TCP客户端(无缓冲)
	SendMsg(data []byte) error
	//SendBuffMsg 直接将数据发送给远程的TCP客户端(有缓冲)
	SendBuffMsg(data []byte) error
	//SendMsgByConnID 将数据发送到指定connID客户端
	SendMsgByConnID(connID uint32, data []byte) error

	//SetProperty 设置链接属性
	SetProperty(key string, value interface{})
	//GetProperty 获取链接属性
	GetProperty(key string) (interface{}, error)
	//RemoveProperty 移除链接属性
	RemoveProperty(key string)
}
