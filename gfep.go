package main

import (
	"gfep/utils"
	"gfep/ziface"
	"gfep/zlog"
	"gfep/znet"
	"gfep/zptl"
	"sync"
	"time"
)

type addrManager struct {
	conns map[string]ziface.IConnection //管理的连接信息
	lock  sync.RWMutex                  //读写连接的读写锁
}

const (
	connIdle   = 0
	connUnknow = 1
	connT698   = 2
	connT376   = 3
	connTNW    = 4
	connA698   = 5
	connA376   = 6
	connANW    = 7
)

//698规约路由
type PTL698_45Router struct {
	znet.BaseRouter
}

//698报文处理方法
func (this *PTL698_45Router) Handle(request ziface.IRequest) {
	zlog.Debug("recv:", zptl.Hex2Str(request.GetData()))
	conn := request.GetConnection()
	connStatus, err := conn.GetProperty("status")
	if err != nil {
		conn.Stop()
		return
	}
	zlog.Debug("connStatus:", connStatus)
	conn.SetProperty("rtime", time.Now()) //最近报文接收时间

	if zptl.Ptl698_45GetDir(request.GetData()) == 0 {
		//from app
		conn.SetProperty("status", connA698)
		//寻找匹配的终端连接，进行转发
	} else {
		//from 终端
		addrStr := zptl.Ptl698_45AddrStr(zptl.Ptl698_45AddrGet(request.GetData()))
		switch zptl.Ptl698_45GetFrameType(request.GetData()) {
		case zptl.LINK_LOGIN:
			if utils.GlobalObject.SupportCommTermianl != true {
				//todo: 查重
			}

			//todo: 检测重新登陆,  若地址变更需要更新addrConn表

			// conn.TcpServer.GetConnMgr()
			zlog.Debug("终端登录", addrStr)
			reply := make([]byte, 128)
			len := zptl.Ptl698_45BuildReplyPacket(request.GetData(), reply)
			err := conn.SendBuffMsg(reply[0:len])
			if err != nil {
				zlog.Error(err)
			} else {
				conn.SetProperty("ltime", time.Now())
				conn.SetProperty("status", connT698)
			}
			return
		case zptl.LINK_HAERTBEAT:
			if utils.GlobalObject.SupportReplyHeart {
				if connStatus != connT698 {
					zlog.Error("终端未登录就发心跳", addrStr)
					conn.Stop()
				} else {
					preAddrStr, err := conn.GetProperty("addr")
					if err == nil && preAddrStr == addrStr {
						//todo: 级联心跳时, 需判断级联地址是否存在
						zlog.Debug("终端心跳", addrStr)
						conn.SetProperty("htime", time.Now()) //更新心跳时间
						reply := make([]byte, 128)
						len := zptl.Ptl698_45BuildReplyPacket(request.GetData(), reply)
						err := conn.SendBuffMsg(reply[0:len])
						if err != nil {
							zlog.Error(err)
						}
					} else {
						zlog.Error("终端登录地址与心跳地址不匹配!", addrStr, addrStr)
						conn.Stop()
					}
				}
			}
			return
		case zptl.LINK_EXIT:
			if connStatus != connT698 {
				zlog.Error("终端未登录就想退出", addrStr)
			} else {
				zlog.Debug("终端退出", addrStr)
				reply := make([]byte, 128)
				len := zptl.Ptl698_45BuildReplyPacket(request.GetData(), reply)
				err := conn.SendMsg(reply[0:len])
				if err != nil {
					zlog.Error(err)
				}
			}
			conn.Stop()
			return
		default:
			break
		}
		//寻找对应APP进行转发
	}
}

//创建连接的时候执行
func DoConnectionBegin(conn ziface.IConnection) {
	conn.SetProperty("status", connIdle)  //默认状态
	conn.SetProperty("ctime", time.Now()) //连接时间
}

//连接断开的时候执行
func DoConnectionLost(conn ziface.IConnection) {
	//todo: 清空addrConn表
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
