package main

import (
	"gfep/ziface"
	"gfep/zlog"
	"gfep/znet"
	"gfep/zptl"
)

//698规约路由
type PTL698_45Router struct {
	znet.BaseRouter
}

//698报文处理方法
func (this *PTL698_45Router) Handle(request ziface.IRequest) {

	zlog.Debug("Call 698 Router Handle")
	//先读取客户端的数据，再回写ping...ping...ping
	zlog.Debug("recv from client : msgID=", request.GetMsgID(), ", data=", string(request.GetData()))

	// err := request.GetConnection().SendBuffMsg(0, []byte("ping...ping...ping"))
	// if err != nil {
	// 	zlog.Error(err)
	// }
}

//创建连接的时候执行
func DoConnectionBegin(conn ziface.IConnection) {
	zlog.Debug("DoConnecionBegin is Called ... ")

	// //设置两个链接属性，在连接创建之后
	// zlog.Debug("Set conn Name, Home done!")
	// conn.SetProperty("Name", "Aceld")
	// conn.SetProperty("Home", "https://www.jianshu.com/u/35261429b7f1")

	// err := conn.SendMsg(2, []byte("DoConnection BEGIN..."))
	// if err != nil {
	// 	zlog.Error(err)
	// }
}

//连接断开的时候执行
func DoConnectionLost(conn ziface.IConnection) {
	//在连接销毁之前，查询conn的Name，Home属性
	// if name, err := conn.GetProperty("Name"); err == nil {
	// 	zlog.Error("Conn Property Name = ", name)
	// }

	// if home, err := conn.GetProperty("Home"); err == nil {
	// 	zlog.Error("Conn Property Home = ", home)
	// }

	zlog.Debug("DoConneciotnLost is Called ... ")
}

func main() {
	//创建一个server句柄
	s := znet.NewServer()

	//注册链接hook回调函数
	s.SetOnConnStart(DoConnectionBegin)
	s.SetOnConnStop(DoConnectionLost)

	//配置路由
	s.AddRouter(zptl.PTL_698_45, &PTL698_45Router{})

	//开启服务
	s.Serve()
}
