package main

import (
	"container/list"
	"fmt"
	"gfep/timewriter"
	"gfep/utils"
	"gfep/ziface"
	"gfep/znet"
	"gfep/zptl"
	"log"
	"os"
	"runtime"
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
	app376List *list.List
	app376Lock sync.RWMutex
	tmn376List *list.List
	tmn376Lock sync.RWMutex

	app698List *list.List
	app698Lock sync.RWMutex
	tmn698List *list.List
	tmn698Lock sync.RWMutex

	appNwList *list.List
	appNwLock sync.RWMutex
	tmnNwList *list.List
	tmnNwLock sync.RWMutex

	log376 *log.Logger
	log698 *log.Logger
	logNw  *log.Logger
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

// Ptl1376_1Router 376规约路由
type Ptl1376_1Router struct {
	znet.BaseRouter
}

// Handle 376报文处理方法
func (r *Ptl1376_1Router) Handle(request ziface.IRequest) {
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
	msaStr := strconv.Itoa(zptl.Ptl1376_1MsaGet(rData))
	tmnStr := zptl.Ptl1376_1AddrStr(zptl.Ptl1376_1AddrGet(rData))
	conn.SetProperty("rtime", time.Now()) //最近报文接收时间

	if zptl.Ptl1376_1GetDir(rData) == 0 {
		//from app
		if connStatus != connIdle && connStatus != connA376 {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connA376)
		log376.Printf("A: % X\n", rData)
		isNewApp := true
		app376Lock.Lock()
		for e := app376List.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				if a.addrStr != msaStr {
					app376List.Remove(e)
				} else {
					isNewApp = false
				}
				break
			}
		}
		if isNewApp {
			app376List.PushBack(addrConnID{msaStr, conn.GetConnID()})
			conn.SetProperty("addr", msaStr)
			log376.Println("后台登录", msaStr, "connID", conn.GetConnID())
		}
		app376Lock.Unlock()

		if zptl.Ptl1376_1GetFrameType(rData) == zptl.ONLINE {
			//todo: 处理app Online响应
			return
		}

		//寻找匹配的终端连接，进行转发
		tmn376Lock.RLock()
		for e := tmn376List.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			//1. 终端地址匹配要转发
			//2. 广播/通配地址需要转发
			if ok && (a.addrStr == tmnStr || strings.HasSuffix(tmnStr, "AA")) {
				err := conn.SendMsgByConnID(a.connID, rData)
				if err != nil {
					//todo: 异常处理
				}
			}
		}
		tmn376Lock.RUnlock()
	} else {
		//from 终端
		if connStatus != connIdle && connStatus != connT376 {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connT376)
		log376.Printf("T: % X\n", rData)
		switch zptl.Ptl1376_1GetFrameType(rData) {
		case zptl.LINK_LOGIN:
			if utils.GlobalObject.SupportCasLink {
				//todo: 处理级联终端登陆
			}

			preTmnStr, err := conn.GetProperty("addr")
			if err != nil || preTmnStr != tmnStr {
				isNewTmn := true
				tmn376Lock.Lock()
				if utils.GlobalObject.SupportCommTermianl != true {
					var next *list.Element
					for e := tmn376List.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						//todo: 尝试比较级联终端
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if ok && a.addrStr == tmnStr && a.connID != conn.GetConnID() {
							log376.Println("终端重复登录", tmnStr, "删除", a.connID)
							//todo: 清除级联
							tmn376List.Remove(e)
						}
					}
				} else {
					var next *list.Element
					for e := tmn376List.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if utils.GlobalObject.SupportCasLink != true {
							if ok && a.connID == conn.GetConnID() && a.addrStr != tmnStr {
								//todo: 有可能是联终端登录
								log376.Println("终端登录地址发生变更", tmnStr, "删除", a.connID)
								tmn376List.Remove(e)
							}
						}
					}
				}
				if isNewTmn {
					tmn376List.PushBack(addrConnID{tmnStr, conn.GetConnID()})
					log376.Println("终端登录", tmnStr, "connID", conn.GetConnID())
				} else {
					log376.Println("终端重新登录", tmnStr, "connID", conn.GetConnID())
				}
				tmn376Lock.Unlock()
			} else {
				log376.Println("终端重新登录", tmnStr, "connID", conn.GetConnID())
			}

			reply := make([]byte, 128, 128)
			len := zptl.Ptl1376_1BuildReplyPacket(rData, reply)
			err = conn.SendBuffMsg(reply[0:len])
			if err != nil {
				log376.Println(err)
			} else {
				conn.SetProperty("ltime", time.Now())
				conn.SetProperty("addr", tmnStr)
				log376.Printf("L: % X\n", reply[0:len])
			}
			return

		case zptl.LINK_HAERTBEAT:
			if utils.GlobalObject.SupportReplyHeart {
				if connStatus != connT376 {
					log376.Println("终端未登录就发心跳", tmnStr)
					conn.NeedStop()
				} else {
					preTmnStr, err := conn.GetProperty("addr")
					if err == nil && preTmnStr == tmnStr {
						//todo: 级联心跳时, 需判断级联地址是否存在
						log376.Println("终端心跳", tmnStr)
						conn.SetProperty("htime", time.Now()) //更新心跳时间
						reply := make([]byte, 128, 128)
						len := zptl.Ptl1376_1BuildReplyPacket(rData, reply)
						err := conn.SendBuffMsg(reply[0:len])
						if err != nil {
							log376.Println(err)
						} else {
							log376.Printf("H: % X", reply[0:len])
						}
					} else {
						log376.Println("终端登录地址与心跳地址不匹配!", preTmnStr, tmnStr)
						conn.NeedStop()
					}
				}
				return
			}
			break

		case zptl.LINK_EXIT:
			if connStatus != connT376 {
				log376.Println("终端未登录就想退出", tmnStr)
			} else {
				log376.Println("终端退出", tmnStr)
				reply := make([]byte, 128, 128)
				len := zptl.Ptl1376_1BuildReplyPacket(rData, reply)
				err := conn.SendMsg(reply[0:len])
				if err != nil {
					log376.Println(err)
				}
			}
			conn.NeedStop()
			return

		default:
			break
		}
		//寻找对应APP进行转发
		app376Lock.RLock()
		for e := app376List.Front(); e != nil; e = e.Next() {
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
		app376Lock.RUnlock()
	}
}

// PTL698_45Router 698规约路由
type PTL698_45Router struct {
	znet.BaseRouter
}

// Handle 698报文处理方法
func (r *PTL698_45Router) Handle(request ziface.IRequest) {
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
	msaStr := strconv.Itoa(zptl.Ptl698_45MsaGet(rData))
	tmnStr := zptl.Ptl698_45AddrStr(zptl.Ptl698_45AddrGet(rData))
	conn.SetProperty("rtime", time.Now()) //最近报文接收时间

	if zptl.Ptl698_45GetDir(rData) == 0 {
		//from app
		if connStatus != connIdle && connStatus != connA698 {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connA698)
		log698.Printf("A: % X\n", rData)
		isNewApp := true
		app698Lock.Lock()
		for e := app698List.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				if a.addrStr != msaStr {
					app698List.Remove(e)
				} else {
					isNewApp = false
				}
				break
			}
		}
		if isNewApp {
			app698List.PushBack(addrConnID{msaStr, conn.GetConnID()})
			conn.SetProperty("addr", msaStr)
			log698.Println("后台登录", msaStr, "connID", conn.GetConnID())
		}
		app698Lock.Unlock()

		if zptl.Ptl698_45GetFrameType(rData) == zptl.ONLINE {
			//todo: 处理app Online响应
			return
		}

		// log698.Println("后台登录", msaStr, "读取", tmnStr)
		//寻找匹配的终端连接，进行转发
		tmn698Lock.RLock()
		for e := tmn698List.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			//1. 终端地址匹配要转发
			//2. 广播/通配地址需要转发
			if ok && (a.addrStr == tmnStr || strings.HasSuffix(tmnStr, "AA")) {
				// log698.Println("后台", msaStr, "转发", tmnStr)
				err := conn.SendMsgByConnID(a.connID, rData)
				if err != nil {
					//todo: 异常处理
				}
			}
		}
		tmn698Lock.RUnlock()
	} else {
		//from 终端
		if connStatus != connIdle && connStatus != connT698 {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connT698)
		log698.Printf("T: % X\n", rData)
		switch zptl.Ptl698_45GetFrameType(rData) {
		case zptl.LINK_LOGIN:
			if utils.GlobalObject.SupportCasLink {
				//todo: 处理级联终端登陆
			}

			preTmnStr, err := conn.GetProperty("addr")
			if err != nil || preTmnStr != tmnStr {
				isNewTmn := true
				tmn698Lock.Lock()
				if utils.GlobalObject.SupportCommTermianl != true {
					var next *list.Element
					for e := tmn698List.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						//todo: 尝试比较级联终端
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if ok && a.addrStr == tmnStr && a.connID != conn.GetConnID() {
							log698.Println("终端重复登录", tmnStr, "删除", a.connID)
							//todo: 清除级联
							tmn698List.Remove(e)
						}
					}
				} else {
					var next *list.Element
					for e := tmn698List.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if utils.GlobalObject.SupportCasLink != true {
							if ok && a.connID == conn.GetConnID() && a.addrStr != tmnStr {
								//todo: 有可能是联终端登录
								log698.Println("终端登录地址发生变更", tmnStr, "删除", a.connID)
								tmn698List.Remove(e)
							}
						}
					}
				}
				if isNewTmn {
					tmn698List.PushBack(addrConnID{tmnStr, conn.GetConnID()})
					log698.Println("终端登录", tmnStr, "connID", conn.GetConnID())
				} else {
					log698.Println("终端重新登录", tmnStr, "connID", conn.GetConnID())
				}
				tmn698Lock.Unlock()
			} else {
				log698.Println("终端重新登录", tmnStr, "connID", conn.GetConnID())
			}

			reply := make([]byte, 128, 128)
			len := zptl.Ptl698_45BuildReplyPacket(rData, reply)
			err = conn.SendBuffMsg(reply[0:len])
			if err != nil {
				log698.Println(err)
			} else {
				conn.SetProperty("ltime", time.Now())
				conn.SetProperty("addr", tmnStr)
				log698.Printf("L: % X\n", reply[0:len])
			}
			return

		case zptl.LINK_HAERTBEAT:
			if utils.GlobalObject.SupportReplyHeart {
				if connStatus != connT698 {
					log698.Println("终端未登录就发心跳", tmnStr)
					conn.NeedStop()
				} else {
					preTmnStr, err := conn.GetProperty("addr")
					if err == nil && preTmnStr == tmnStr {
						//todo: 级联心跳时, 需判断级联地址是否存在
						log698.Println("终端心跳", tmnStr)
						conn.SetProperty("htime", time.Now()) //更新心跳时间
						reply := make([]byte, 128, 128)
						len := zptl.Ptl698_45BuildReplyPacket(rData, reply)
						err := conn.SendBuffMsg(reply[0:len])
						if err != nil {
							log698.Println(err)
						} else {
							log698.Printf("H: % X", reply[0:len])
						}
					} else {
						log698.Println("终端登录地址与心跳地址不匹配!", preTmnStr, tmnStr)
						conn.NeedStop()
					}
				}
				return
			}
			break

		case zptl.LINK_EXIT:
			if connStatus != connT698 {
				log698.Println("终端未登录就想退出", tmnStr)
			} else {
				log698.Println("终端退出", tmnStr)
				reply := make([]byte, 128, 128)
				len := zptl.Ptl698_45BuildReplyPacket(rData, reply)
				err := conn.SendMsg(reply[0:len])
				if err != nil {
					log698.Println(err)
				}
			}
			conn.NeedStop()
			return

		default:
			break
		}
		//寻找对应APP进行转发
		app698Lock.RLock()
		for e := app698List.Front(); e != nil; e = e.Next() {
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
		app698Lock.RUnlock()
	}
}

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
		logNw.Printf("A: % X\n", rData)
		isNewApp := true
		appNwLock.Lock()
		for e := appNwList.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				if a.addrStr != msaStr {
					appNwList.Remove(e)
				} else {
					isNewApp = false
				}
				break
			}
		}
		if isNewApp {
			appNwList.PushBack(addrConnID{msaStr, conn.GetConnID()})
			conn.SetProperty("addr", msaStr)
			logNw.Println("后台登录", msaStr, "connID", conn.GetConnID())
		}
		appNwLock.Unlock()

		if zptl.PtlNwGetFrameType(rData) == zptl.ONLINE {
			//todo: 处理app Online响应
			return
		}

		// logNw.Println("后台登录", msaStr, "读取", tmnStr)
		//寻找匹配的终端连接，进行转发
		tmnNwLock.RLock()
		for e := tmnNwList.Front(); e != nil; e = e.Next() {
			a, ok := (e.Value).(addrConnID)
			//1. 终端地址匹配要转发
			//2. 广播/通配地址需要转发
			if ok && (a.addrStr == tmnStr || strings.HasSuffix(tmnStr, "AA")) {
				// logNw.Println("后台", msaStr, "转发", tmnStr)
				err := conn.SendMsgByConnID(a.connID, rData)
				if err != nil {
					//todo: 异常处理
				}
			}
		}
		tmnNwLock.RUnlock()
	} else {
		//from 终端
		if connStatus != connIdle && connStatus != connTNW {
			conn.NeedStop()
			return
		}
		conn.SetProperty("status", connTNW)
		logNw.Printf("T: % X\n", rData)
		switch zptl.PtlNwGetFrameType(rData) {
		case zptl.LINK_LOGIN:
			if utils.GlobalObject.SupportCasLink {
				//todo: 处理级联终端登陆
			}

			preTmnStr, err := conn.GetProperty("addr")
			if err != nil || preTmnStr != tmnStr {
				isNewTmn := true
				tmnNwLock.Lock()
				if utils.GlobalObject.SupportCommTermianl != true {
					var next *list.Element
					for e := tmnNwList.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						//todo: 尝试比较级联终端
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if ok && a.addrStr == tmnStr && a.connID != conn.GetConnID() {
							logNw.Println("终端重复登录", tmnStr, "删除", a.connID)
							//todo: 清除级联
							tmnNwList.Remove(e)
						}
					}
				} else {
					var next *list.Element
					for e := tmnNwList.Front(); e != nil; e = next {
						next = e.Next()
						a, ok := (e.Value).(addrConnID)
						if ok && a.addrStr == tmnStr && a.connID == conn.GetConnID() {
							isNewTmn = false
							break
						}
						if utils.GlobalObject.SupportCasLink != true {
							if ok && a.connID == conn.GetConnID() && a.addrStr != tmnStr {
								//todo: 有可能是联终端登录
								logNw.Println("终端登录地址发生变更", tmnStr, "删除", a.connID)
								tmnNwList.Remove(e)
							}
						}
					}
				}
				if isNewTmn {
					tmnNwList.PushBack(addrConnID{tmnStr, conn.GetConnID()})
					logNw.Println("终端登录", tmnStr, "connID", conn.GetConnID())
				} else {
					logNw.Println("终端重新登录", tmnStr, "connID", conn.GetConnID())
				}
				tmnNwLock.Unlock()
			} else {
				logNw.Println("终端重新登录", tmnStr, "connID", conn.GetConnID())
			}

			reply := make([]byte, 128, 128)
			len := zptl.PtlNwBuildReplyPacket(rData, reply)
			err = conn.SendBuffMsg(reply[0:len])
			if err != nil {
				logNw.Println(err)
			} else {
				conn.SetProperty("ltime", time.Now())
				conn.SetProperty("addr", tmnStr)
				logNw.Printf("L: % X\n", reply[0:len])
			}
			return

		case zptl.LINK_HAERTBEAT:
			if utils.GlobalObject.SupportReplyHeart {
				if connStatus != connTNW {
					logNw.Println("终端未登录就发心跳", tmnStr)
					conn.NeedStop()
				} else {
					preTmnStr, err := conn.GetProperty("addr")
					if err == nil && preTmnStr == tmnStr {
						//todo: 级联心跳时, 需判断级联地址是否存在
						logNw.Println("终端心跳", tmnStr)
						conn.SetProperty("htime", time.Now()) //更新心跳时间
						reply := make([]byte, 128, 128)
						len := zptl.PtlNwBuildReplyPacket(rData, reply)
						err := conn.SendBuffMsg(reply[0:len])
						if err != nil {
							logNw.Println(err)
						} else {
							logNw.Printf("H: % X", reply[0:len])
						}
					} else {
						logNw.Println("终端登录地址与心跳地址不匹配!", preTmnStr, tmnStr)
						conn.NeedStop()
					}
				}
				return
			}
			break

		case zptl.LINK_EXIT:
			if connStatus != connTNW {
				logNw.Println("终端未登录就想退出", tmnStr)
			} else {
				logNw.Println("终端退出", tmnStr)
				reply := make([]byte, 128, 128)
				len := zptl.PtlNwBuildReplyPacket(rData, reply)
				err := conn.SendMsg(reply[0:len])
				if err != nil {
					logNw.Println(err)
				}
			}
			conn.NeedStop()
			return

		default:
			break
		}
		//寻找对应APP进行转发
		appNwLock.RLock()
		for e := appNwList.Front(); e != nil; e = e.Next() {
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
		appNwLock.RUnlock()
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
	case connT376:
		tmn376Lock.Lock()
		var next *list.Element
		for e := tmn376List.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				tmn376List.Remove(e)
				if utils.GlobalObject.SupportCas != true {
					break
				}
			}
		}
		tmn376Lock.Unlock()
		break

	case connA376:
		app376Lock.Lock()
		var next *list.Element
		for e := app376List.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				app376List.Remove(e)
			}
		}
		app376Lock.Unlock()
		break

	case connT698:
		tmn698Lock.Lock()
		var next *list.Element
		for e := tmn698List.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				tmn698List.Remove(e)
				if utils.GlobalObject.SupportCas != true {
					break
				}
			}
		}
		tmn698Lock.Unlock()
		break

	case connA698:
		app698Lock.Lock()
		var next *list.Element
		for e := app698List.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				app698List.Remove(e)
			}
		}
		app698Lock.Unlock()
		break

	case connTNW:
		tmnNwLock.Lock()
		var next *list.Element
		for e := tmnNwList.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				tmnNwList.Remove(e)
				if utils.GlobalObject.SupportCas != true {
					break
				}
			}
		}
		tmnNwLock.Unlock()
		break

	case connANW:
		appNwLock.Lock()
		var next *list.Element
		for e := appNwList.Front(); e != nil; e = next {
			next = e.Next()
			a, ok := (e.Value).(addrConnID)
			if ok && a.connID == conn.GetConnID() {
				appNwList.Remove(e)
			}
		}
		appNwLock.Unlock()
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
			var i int
			var next *list.Element

			tmn376Lock.RLock()
			for e := tmn376List.Front(); e != nil; e = next {
				next = e.Next()
				t, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("376[%d] %s, %d\n", i, t.addrStr, t.connID)
					i++
				}
			}
			tmn376Lock.RUnlock()

			tmn698Lock.RLock()
			for e := tmn698List.Front(); e != nil; e = next {
				next = e.Next()
				t, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("698[%d] %s, %d\n", i, t.addrStr, t.connID)
					i++
				}
			}
			tmn698Lock.RUnlock()

			tmnNwLock.RLock()
			for e := tmnNwList.Front(); e != nil; e = next {
				next = e.Next()
				t, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("Nw[%d] %s, %d\n", i, t.addrStr, t.connID)
					i++
				}
			}
			tmnNwLock.RUnlock()
		case 2:
			var i int
			var next *list.Element

			app376Lock.RLock()
			for e := app376List.Front(); e != nil; e = next {
				next = e.Next()
				a, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("376[%d] %s, %d\n", i, a.addrStr, a.connID)
					i++
				}
			}
			app376Lock.RUnlock()

			app698Lock.RLock()
			for e := app698List.Front(); e != nil; e = next {
				next = e.Next()
				a, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("698[%d] %s, %d\n", i, a.addrStr, a.connID)
					i++
				}
			}
			app698Lock.RUnlock()

			appNwLock.RLock()
			for e := appNwList.Front(); e != nil; e = next {
				next = e.Next()
				a, ok := (e.Value).(addrConnID)
				if ok {
					fmt.Printf("Nw[%d] %s, %d\n", i, a.addrStr, a.connID)
					i++
				}
			}
			appNwLock.RUnlock()
		case 3:
			fmt.Println("V0.0.1")
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

func logInit() {
	log376 = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "376",
	}, "", log.LstdFlags)

	log698 = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "698",
	}, "", log.LstdFlags)

	logNw = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "nw",
	}, "", log.LstdFlags)
}

func main() {
	// runtime.GOMAXPROCS(runtime.NumCPU())

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:9999", nil))
	// }()

	if runtime.GOOS != "linux" {
		go usrInput()
	}

	app698List = list.New()
	tmn698List = list.New()

	app376List = list.New()
	tmn376List = list.New()

	appNwList = list.New()
	tmnNwList = list.New()
	logInit()

	//创建一个server句柄
	s := znet.NewServer()

	//注册链接hook回调函数
	s.SetOnConnStart(DoConnectionBegin)
	s.SetOnConnStop(DoConnectionLost)

	//配置路由
	s.AddRouter(zptl.PTL_1376_1, &Ptl1376_1Router{})
	s.AddRouter(zptl.PTL_698_45, &PTL698_45Router{})
	s.AddRouter(zptl.PTL_NW, &PTLNWRouter{})

	//开启服务
	s.Serve()
}
