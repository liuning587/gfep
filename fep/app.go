package fep

import (
	"fmt"
	"gfep/bridge"
	"gfep/internal/netaddr"
	"gfep/timewriter"
	"gfep/utils"
	"gfep/ziface"
	"gfep/znet"
	"gfep/zptl"
	"log"
	"os"
	"runtime"
	"strings"
	"time"
)

var (
	regApp376 = newAppRegistry()
	regTmn376 = newTmnRegistry()
	regApp698 = newAppRegistry()
	regTmn698 = newTmnRegistry()
	regAppNw  = newAppRegistry()
	regTmnNw  = newTmnRegistry()

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

// Ptl1376_1Router 376规约路由（逻辑见 gfep_ptl.go 中 profile376）。
type Ptl1376_1Router struct {
	znet.BaseRouter
}

func (r *Ptl1376_1Router) Handle(request ziface.IRequest) {
	profile376.Handle(request)
}

// PTL698_45Router 698规约路由（逻辑见 gfep_ptl.go 中 profile698）。
type PTL698_45Router struct {
	znet.BaseRouter
}

func (r *PTL698_45Router) Handle(request ziface.IRequest) {
	profile698.Handle(request)
}

// PTLNWRouter NW规约路由（逻辑见 gfep_ptl.go 中 profileNw）。
type PTLNWRouter struct {
	znet.BaseRouter
}

func (r *PTLNWRouter) Handle(request ziface.IRequest) {
	profileNw.Handle(request)
}

// DoConnectionBegin 创建连接的时候执行
func DoConnectionBegin(conn ziface.IConnection) {
	conn.SetProperty("status", connIdle)
	conn.SetProperty("ctime", time.Now())
}

// DoConnectionLost 连接断开的时候执行
func DoConnectionLost(conn ziface.IConnection) {
	statusVal, err := conn.GetProperty("status")
	connStatus := connIdle
	if err != nil {
		log.Printf("gfep: DoConnectionLost: GetProperty(status) failed connID=%d: %v", conn.GetConnID(), err)
	} else if s, ok := statusVal.(int); ok {
		connStatus = s
	} else {
		log.Printf("gfep: DoConnectionLost: unexpected status type connID=%d: %T", conn.GetConnID(), statusVal)
	}

	switch connStatus {
	case connT376:
		regTmn376.removeConn(conn.GetConnID())

	case connA376:
		regApp376.removeConn(conn.GetConnID())

	case connT698:
		b, err := conn.GetProperty("bridge")
		if err == nil {
			if v, ok := b.(*bridge.Conn); ok {
				v.Stop()
			}
			conn.RemoveProperty("bridge")
		}
		regTmn698.removeConn(conn.GetConnID())

	case connA698:
		regApp698.removeConn(conn.GetConnID())

	case connTNW:
		regTmnNw.removeConn(conn.GetConnID())

	case connANW:
		regAppNw.removeConn(conn.GetConnID())

	default:
		break
	}
}

const usrConnTimeFmt = "2006-01-02 15:04:05.000"

func fmtUsrConnTime(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(usrConnTimeFmt)
}

// printOnlineConnDetail 打印单条在线连接（终端/后台列表用）。
func printOnlineConnDetail(tag string, seq int, addr string, connID uint32, isApp bool) {
	const labelW = 14
	line := func() { fmt.Println(strings.Repeat("-", 56)) }

	addrLabel := "通信地址"
	if isApp {
		addrLabel = "主站 MSA"
	}
	show698Report := !isApp && strings.HasPrefix(tag, "698")

	srv := utils.GlobalObject.TCPServer
	if srv == nil {
		fmt.Printf("%s #%d  conn=%d  %s=%s  (TCP 未就绪)\n", tag, seq, connID, addrLabel, addr)
		return
	}
	ic, err := srv.GetConnMgr().Get(connID)
	if err != nil {
		fmt.Printf("%s #%d  conn=%d  %s=%s  (已断开)\n", tag, seq, connID, addrLabel, addr)
		return
	}
	co, ok := ic.(*znet.Connection)
	if !ok {
		peer := "-"
		if ra := ic.RemoteAddr(); ra != nil {
			peer = netaddr.FormatTCP(ra)
		}
		fmt.Printf("%s #%d  conn=%d  %s=%s  peer=%s  (非 *znet.Connection)\n", tag, seq, connID, addrLabel, addr, peer)
		return
	}
	d := co.Details()

	line()
	fmt.Printf(" %s  #%d\n", tag, seq)
	fmt.Printf(" %-*s  %d\n", labelW, "连接 ID", connID)
	fmt.Printf(" %-*s  %s\n", labelW, addrLabel, addr)
	fmt.Printf(" %-*s  %s\n", labelW, "对端 TCP", d.RemoteTCP)
	fmt.Println()
	fmt.Printf(" %-*s  %s\n", labelW, "连接建立", fmtUsrConnTime(d.Ctime))
	if !isApp {
		fmt.Printf(" %-*s  %s\n", labelW, "登录", fmtUsrConnTime(d.Ltime))
		fmt.Printf(" %-*s  %s\n", labelW, "心跳", fmtUsrConnTime(d.Htime))
	}
	fmt.Printf(" %-*s  %s\n", labelW, "最近收帧", fmtUsrConnTime(d.Rtime))
	fmt.Printf(" %-*s  %s\n", labelW, "最近发送", fmtUsrConnTime(d.LastTxAt))
	if show698Report {
		fmt.Printf(" %-*s  %s\n", labelW, "上报 MSA=0", fmtUsrConnTime(d.LastReportAt))
	}
	fmt.Println()
	fmt.Printf(" %-*s  收 %d 帧 / %d B    发 %d 次 / %d B\n",
		labelW, "流量统计", d.RxFrames, d.RxFrameBytes, d.TxWrites, d.TxWriteBytes)
	line()
	fmt.Println()
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
		_, _ = fmt.Scanln(&menu)
		fmt.Println("Hi you input is", menu)
		switch menu {
		case 1:
			i := 0
			for _, t := range regTmn376.snapshot() {
				printOnlineConnDetail("376终端", i, t.addrStr, t.connID, false)
				i++
			}
			for _, t := range regTmn698.snapshot() {
				printOnlineConnDetail("698终端", i, t.addrStr, t.connID, false)
				i++
			}
			for _, t := range regTmnNw.snapshot() {
				printOnlineConnDetail("Nw终端", i, t.addrStr, t.connID, false)
				i++
			}
		case 2:
			i := 0
			for _, a := range regApp376.snapshot() {
				printOnlineConnDetail("376后台", i, a.addrStr, a.connID, true)
				i++
			}
			for _, a := range regApp698.snapshot() {
				printOnlineConnDetail("698后台", i, a.addrStr, a.connID, true)
				i++
			}
			for _, a := range regAppNw.snapshot() {
				printOnlineConnDetail("Nw后台", i, a.addrStr, a.connID, true)
				i++
			}
		case 3:
			fmt.Println(utils.GlobalObject.Version)
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
		fmt.Printf("%s", helper)
	}
}

func logInit() {
	log376 = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "376",
	}, "", log.LstdFlags|log.Lmicroseconds)

	log698 = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "698",
	}, "", log.LstdFlags|log.Lmicroseconds)

	logNw = log.New(&timewriter.TimeWriter{
		Dir:        utils.GlobalObject.LogDir,
		Compress:   true,
		ReserveDay: 30,
		ModuleName: "nw",
	}, "", log.LstdFlags|log.Lmicroseconds)
}

// Main 启动前置机 TCP 服务（由根目录 gfep.go 的 main 调用）。
func Main() {
	if runtime.GOOS != "linux" {
		go usrInput()
	}

	logInit()
	initPtlProfiles()
	initForwardPool()
	startLogWebIfEnabled()

	s := znet.NewServer()

	s.SetOnConnStart(DoConnectionBegin)
	s.SetOnConnStop(DoConnectionLost)

	s.AddRouter(zptl.PTL_1376_1, &Ptl1376_1Router{})
	s.AddRouter(zptl.PTL_698_45, &PTL698_45Router{})
	s.AddRouter(zptl.PTL_NW, &PTLNWRouter{})

	s.Serve()
}
