package main

import (
	"container/list"
	"fmt"
	"gfep/utils"
	"gfep/ziface"
	"gfep/zlog"
	"gfep/znet"
	"gfep/zptl"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
	// _ "net/http/pprof"
)

type addrConnID struct {
	addrStr string
	connID  uint32
}

var (
	appList *list.List
	appLock sync.RWMutex
	tmnList *list.List
	tmnLock sync.RWMutex
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

// PTLNWRouter NW规约路由
type PTLNWRouter struct {
	znet.BaseRouter
}

// Handle nw报文处理方法
func (r *PTLNWRouter) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	if conn.IsStop() {
		return
	}
	connStatus, err := conn.GetProperty("status")
	if err != nil {
		conn.NeedStop()
		return
	}
	rData := request.GetData()
	msaStr := strconv.Itoa(zptl.PtlNwMsaGet(rData))
	tmnStr := zptl.PtlNwAddrStr(zptl.PtlNwAddrGet(rData))
	conn.SetProperty("rtime", time.Now()) //最近报文接收时间

	if zptl.PtlNwGetDir(rData) == 0 {
		//from app
		if connStatus != connIdle && connStatus != connANW {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connANW)
		zlog.Debugf("A: % X\n", rData)
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

		if zptl.PtlNwGetFrameType(rData) == zptl.ONLINE {
			//todo: 处理app Online响应
			return
		}

		// zlog.Debug("后台登录", msaStr, "读取", tmnStr)
		//寻找匹配的终端连接，进行转发
		tmnLock.RLock()
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
		tmnLock.RUnlock()
	} else {
		//from 终端
		if connStatus != connIdle && connStatus != connTNW {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connTNW)
		zlog.Debugf("T: % X\n", rData)
		switch zptl.PtlNwGetFrameType(rData) {
		case zptl.LINK_LOGIN:
			if utils.GlobalObject.SupportCasLink {
				//todo: 处理级联终端登陆
			}

			preTmnStr, err := conn.GetProperty("addr")
			if err != nil || preTmnStr != tmnStr {
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
							break
						}
						if ok && a.addrStr == tmnStr && a.connID != conn.GetConnID() {
							zlog.Debug("终端重复登录", tmnStr, "删除", a.connID)
							//todo: 清除级联
							tmnList.Remove(e)
						}
					}
				} else {
					var next *list.Element
					for e := tmnList.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if utils.GlobalObject.SupportCasLink != true {
							if ok && a.connID == conn.GetConnID() && a.addrStr != tmnStr {
								//todo: 有可能是联终端登录
								zlog.Debug("终端登录地址发生变更", tmnStr, "删除", a.connID)
								tmnList.Remove(e)
							}
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
			} else {
				zlog.Debug("终端重新登录", tmnStr, "connID", conn.GetConnID())
			}

			reply := make([]byte, 128, 128)
			len := zptl.PtlNwBuildReplyPacket(rData, reply)
			err = conn.SendBuffMsg(reply[0:len])
			if err != nil {
				zlog.Error(err)
			} else {
				conn.SetProperty("ltime", time.Now())
				conn.SetProperty("addr", tmnStr)
				zlog.Debugf("L: % X\n", reply[0:len])
			}
			return

		case zptl.LINK_HAERTBEAT:
			if utils.GlobalObject.SupportReplyHeart {
				if connStatus != connTNW {
					zlog.Error("终端未登录就发心跳", tmnStr)
					conn.NeedStop()
				} else {
					preTmnStr, err := conn.GetProperty("addr")
					if err == nil && preTmnStr == tmnStr {
						//todo: 级联心跳时, 需判断级联地址是否存在
						zlog.Debug("终端心跳", tmnStr)
						conn.SetProperty("htime", time.Now()) //更新心跳时间
						reply := make([]byte, 128, 128)
						len := zptl.PtlNwBuildReplyPacket(rData, reply)
						err := conn.SendBuffMsg(reply[0:len])
						if err != nil {
							zlog.Error(err)
						} else {
							zlog.Debugf("H: % X", reply[0:len])
						}
					} else {
						zlog.Error("终端登录地址与心跳地址不匹配!", preTmnStr, tmnStr)
						conn.NeedStop()
					}
				}
				return
			}
			break

		case zptl.LINK_EXIT:
			if connStatus != connTNW {
				zlog.Error("终端未登录就想退出", tmnStr)
			} else {
				zlog.Debug("终端退出", tmnStr)
				reply := make([]byte, 128, 128)
				len := zptl.PtlNwBuildReplyPacket(rData, reply)
				err := conn.SendMsg(reply[0:len])
				if err != nil {
					zlog.Error(err)
				}
			}
			conn.NeedStop()
			return

		default:
			break
		}
		//寻找对应APP进行转发
		appLock.RLock()
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
		appLock.RUnlock()
	}
}

// DoConnectionBegin 创建连接的时候执行
func DoConnectionBegin(conn ziface.IConnection) {
	conn.SetProperty("status", connIdle)  //默认状态
	conn.SetProperty("ctime", time.Now()) //连接时间
}

// DoConnectionLost 连接断开的时候执行
func DoConnectionLost(conn ziface.IConnection) {
	connStatus, err := conn.GetProperty("status")
	if err != nil {
		panic("connStatus != err")
	}

	switch connStatus {
	case connTNW:
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

	case connANW:
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

func usrInput() {
	helper := `~~~~~~~~~~~~~~~~~~~
1. 显示在线终端列表
2. 显示在线后台列表
3. 显示版本信息
4. 设置调试级别
5. 屏蔽心跳
6. 剔除终端
7. 尝试升级
8. 退出
~~~~~~~~~~~~~~~~~~~
:`
	var menu int

	for {
		menu = 0
		fmt.Scanln(&menu)
		fmt.Println("Hi you input is", menu)
		switch menu {
		case 1:
			tmnLock.RLock()
			var i int
			var next *list.Element
			for e := tmnList.Front(); e != nil; e = next {
				next = e.Next()
				t, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("%d %s, %d\n", i, t.addrStr, t.connID)
					i++
				}
			}
			tmnLock.RUnlock()
		case 2:
			appLock.RLock()
			var i int
			var next *list.Element
			for e := appList.Front(); e != nil; e = next {
				next = e.Next()
				a, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("%d %s, %d\n", i, a.addrStr, a.connID)
					i++
				}
			}
			appLock.RUnlock()
		case 4:
			fmt.Println("功能未实现!")
		case 5:
			fmt.Println("功能未实现!")
		case 6:
			fmt.Println("功能未实现!")
		case 7:
			fmt.Println("功能未实现!")
		case 8:
			os.Exit(0)
		}
		fmt.Printf(helper)
	}
}

func main() {
	// runtime.GOMAXPROCS(runtime.NumCPU())

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:9999", nil))
	// }()

	// zlog.SetLogFile("./log", "gfep.log")
	// zlog.OpenDebug()
	// zlog.ResetFlags(zlog.BitDefault | zlog.BitMicroSeconds)
	// zlog.CloseDebug()
	// go usrInput()

	appList = list.New()
	tmnList = list.New()

	//创建一个server句柄
	s := znet.NewServer()

	//注册链接hook回调函数
	s.SetOnConnStart(DoConnectionBegin)
	s.SetOnConnStop(DoConnectionLost)

	//配置路由
	s.AddRouter(zptl.PTL_NW, &PTLNWRouter{})

	//开启服务
	s.Serve()
}
