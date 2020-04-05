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
	zlog.Debug("recv from client:", zptl.Hex2Str(request.GetData()))

	if zptl.Ptl698_45GetDir(request.GetData()) == 0 {
		//from app
		//寻找匹配的终端连接，进行转发
	} else {
		//from 终端
		switch zptl.Ptl698_45GetFrameType(request.GetData()) {
		case zptl.LINK_LOGIN:
			zlog.Debug("终端登录", zptl.Ptl698_45AddrStr(zptl.Ptl698_45AddrGet(request.GetData())))
			reply := make([]byte, 128)
			len := zptl.Ptl698_45BuildReplyPacket(request.GetData(), reply)
			err := request.GetConnection().SendBuffMsg(reply[0:len])
			if err != nil {
				zlog.Error(err)
			}
			return
		case zptl.LINK_HAERTBEAT:
			zlog.Debug("终端心跳", zptl.Ptl698_45AddrStr(zptl.Ptl698_45AddrGet(request.GetData())))
			reply := make([]byte, 128)
			len := zptl.Ptl698_45BuildReplyPacket(request.GetData(), reply)
			err := request.GetConnection().SendBuffMsg(reply[0:len])
			if err != nil {
				zlog.Error(err)
			}
			return
		case zptl.LINK_EXIT:
			zlog.Debug("终端退出", zptl.Ptl698_45AddrStr(zptl.Ptl698_45AddrGet(request.GetData())))
			reply := make([]byte, 128)
			len := zptl.Ptl698_45BuildReplyPacket(request.GetData(), reply)
			err := request.GetConnection().SendBuffMsg(reply[0:len])
			if err != nil {
				zlog.Error(err)
			}
			return
		default:
			break
		}
		//寻找对应APP进行转发
	}
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
