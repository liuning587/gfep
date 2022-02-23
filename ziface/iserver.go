package ziface

// IServer 定义服务器接口
type IServer interface {
	//Start 启动服务器方法
	Start()
	//Stop 停止服务器方法
	Stop()
	//Serve 开启业务服务方法
	Serve()
	//AddRouter 路由功能：给当前服务注册一个路由业务方法，供客户端链接处理使用
	AddRouter(msgID uint32, router IRouter)
	//GetConnMgr 得到链接管理
	GetConnMgr() IConnManager
	//SetOnConnStart 设置该Server的连接创建时Hook函数
	SetOnConnStart(func(IConnection))
	//SetOnConnStop 设置该Server的连接断开时的Hook函数
	SetOnConnStop(func(IConnection))
	//CallOnConnStart 调用连接OnConnStart Hook函数
	CallOnConnStart(conn IConnection)
	//CallOnConnStop 调用连接OnConnStop Hook函数
	CallOnConnStop(conn IConnection)
}
