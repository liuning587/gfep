package main

import (
	"container/list"
	"gfep/utils"
	"gfep/ziface"
	"gfep/zlog"
	"gfep/znet"
	"gfep/zptl"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// type addrManager struct {
// 	conns map[string][]ziface.IConnection //管理的连接信息
// 	lock  sync.RWMutex                    //读写连接的读写锁
// }

type addrConnID struct {
	addrStr string
	connID  uint32
}

var (
	appList *list.List
	appLock sync.Mutex
	tmnList *list.List
	tmnLock sync.Mutex
)

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
	conn := request.GetConnection()
	connStatus, err := conn.GetProperty("status")
	if err != nil {
		conn.Stop()
		return
	}
	rData := request.GetData()
	msaStr := strconv.Itoa(zptl.Ptl698_45MsaGet(rData))
	tmnStr := zptl.Ptl698_45AddrStr(zptl.Ptl698_45AddrGet(rData))
	conn.SetProperty("rtime", time.Now()) //最近报文接收时间

	if zptl.Ptl698_45GetDir(rData) == 0 {
		zlog.Debug("A:", zptl.Hex2Str(rData))
		//from app
		conn.SetProperty("status", connA698)
		isNewApp := true
		appLock.Lock()
		for e := appList.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			if ok && a.addrStr == msaStr {
				a.connID = conn.GetConnID()
				isNewApp = false
				break
			}
		}
		if isNewApp {
			appList.PushBack(addrConnID{msaStr, conn.GetConnID()})
			conn.SetProperty("addr", msaStr)
			zlog.Debug("后台登录", msaStr, "connID", conn.GetConnID())
		}
		appLock.Unlock()

		if zptl.Ptl698_45GetFrameType(rData) == zptl.ONLINE {
			//todo: 处理app Online响应
			return
		}

		// zlog.Debug("后台登录", msaStr, "读取", tmnStr)
		//寻找匹配的终端连接，进行转发
		tmnLock.Lock()
		for e := tmnList.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			//1. 终端地址匹配要转发
			//2. 广播/通配地址需要转发
			if ok && (a.addrStr == tmnStr || strings.HasSuffix(tmnStr, "AA")) {
				// zlog.Debug("后台", msaStr, "转发", tmnStr)
				err := conn.SendMsgByConnID(a.connID, rData)
				if err != nil {
					//todo: 异常处理
				}
			}
		}
		tmnLock.Unlock()
	} else {
		//from 终端
		zlog.Debug("T:", zptl.Hex2Str(rData))
		switch zptl.Ptl698_45GetFrameType(rData) {
		case zptl.LINK_LOGIN:
			if utils.GlobalObject.SupportCasLink {
				//todo: 处理级联终端登陆
			}

			isNewTmn := true
			tmnLock.Lock()
			if utils.GlobalObject.SupportCommTermianl != true {
				var next *list.Element
				for e := tmnList.Front(); e != nil; e = next {
					next = e.Next()
					a, ok := (e.Value).(addrConnID)
					//todo: 尝试比较级联终端
					if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
						isNewTmn = false
					}
					if ok && a.addrStr == tmnStr && a.connID != conn.GetConnID() {
						zlog.Debug("终端重复登录", tmnStr, "删除", a.connID)
						//todo: 清除级联
						tmnList.Remove(e)
					}
				}
			} else {
				for e := tmnList.Front(); e != nil; e = e.Next() {
					a, ok := (e.Value).(addrConnID)
					if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
						isNewTmn = false
						break
					}
				}
			}
			if isNewTmn {
				tmnList.PushBack(addrConnID{tmnStr, conn.GetConnID()})
				zlog.Debug("终端登录", tmnStr, "connID", conn.GetConnID())
			} else {
				zlog.Debug("终端重新登录", tmnStr, "connID", conn.GetConnID())
			}
			tmnLock.Unlock()

			reply := make([]byte, 128, 128)
			len := zptl.Ptl698_45BuildReplyPacket(rData, reply)
			err := conn.SendBuffMsg(reply[0:len])
			if err != nil {
				zlog.Error(err)
			} else {
				conn.SetProperty("ltime", time.Now())
				conn.SetProperty("status", connT698)
				conn.SetProperty("addr", tmnStr)
				zlog.Debug("L:", zptl.Hex2Str(reply[0:len]))
			}
			return
		case zptl.LINK_HAERTBEAT:
			if utils.GlobalObject.SupportReplyHeart {
				if connStatus != connT698 {
					zlog.Error("终端未登录就发心跳", tmnStr)
					conn.Stop()
				} else {
					preTmnStr, err := conn.GetProperty("addr")
					if err == nil && preTmnStr == tmnStr {
						//todo: 级联心跳时, 需判断级联地址是否存在
						zlog.Debug("终端心跳", tmnStr)
						conn.SetProperty("htime", time.Now()) //更新心跳时间
						reply := make([]byte, 128, 128)
						len := zptl.Ptl698_45BuildReplyPacket(rData, reply)
						err := conn.SendBuffMsg(reply[0:len])
						if err != nil {
							zlog.Error(err)
						} else {
							zlog.Debug("H:", zptl.Hex2Str(reply[0:len]))
						}
					} else {
						zlog.Error("终端登录地址与心跳地址不匹配!", preTmnStr, tmnStr)
						conn.Stop()
					}
				}
				return
			}
			break
		case zptl.LINK_EXIT:
			if connStatus != connT698 {
				zlog.Error("终端未登录就想退出", tmnStr)
			} else {
				zlog.Debug("终端退出", tmnStr)
				reply := make([]byte, 128, 128)
				len := zptl.Ptl698_45BuildReplyPacket(rData, reply)
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
		appLock.Lock()
		for e := appList.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			//1. 终端主动上报msa==0,所有后台都转发
			//2. 后台msa为匹配要转发
			if ok && (msaStr == "0" || a.addrStr == msaStr) {
				err := conn.SendMsgByConnID(a.connID, rData)
				if err != nil {
					//todo: 异常处理
				}
			}
		}
		appLock.Unlock()
	}
}

//创建连接的时候执行
func DoConnectionBegin(conn ziface.IConnection) {
	conn.SetProperty("status", connIdle)  //默认状态
	conn.SetProperty("ctime", time.Now()) //连接时间
}

//连接断开的时候执行
func DoConnectionLost(conn ziface.IConnection) {
	connStatus, err := conn.GetProperty("status")
	if err != nil {
		return
	}

	switch connStatus {
	case connT698:
		tmnLock.Lock()
		var next *list.Element
		for e := tmnList.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				tmnList.Remove(e)
				if utils.GlobalObject.SupportCas != true {
					break
				}
			}
		}
		tmnLock.Unlock()
		break

	case connA698:
		appLock.Lock()
		var next *list.Element
		for e := appList.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				appList.Remove(e)
			}
		}
		appLock.Unlock()
		break

	default:
		break
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	appList = list.New()
	tmnList = list.New()

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
