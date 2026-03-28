package main

import (
	"fmt"
	"gfep/bridge"
	"gfep/timewriter"
	"gfep/utils"
	"gfep/ziface"
	"gfep/znet"
	"gfep/zptl"
	"log"
	"os"
	"runtime"
	"time"
	// _ "net/http/pprof"
)

type addrConnID struct {
	addrStr string
	connID  uint32
}

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
	conn.SetProperty("status", connIdle)  //默认状态
	conn.SetProperty("ctime", time.Now()) //连接时间
	// 级联/多表位与 SupportCas、SupportCasLink 等仍依赖 gfep_ptl 内 todo 与配置，断线清理见 DoConnectionLost。
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
				fmt.Printf("376[%d] %s, %d\n", i, t.addrStr, t.connID)
				i++
			}
			for _, t := range regTmn698.snapshot() {
				fmt.Printf("698[%d] %s, %d\n", i, t.addrStr, t.connID)
				i++
			}
			for _, t := range regTmnNw.snapshot() {
				fmt.Printf("Nw[%d] %s, %d\n", i, t.addrStr, t.connID)
				i++
			}
		case 2:
			i := 0
			for _, a := range regApp376.snapshot() {
				fmt.Printf("376[%d] %s, %d\n", i, a.addrStr, a.connID)
				i++
			}
			for _, a := range regApp698.snapshot() {
				fmt.Printf("698[%d] %s, %d\n", i, a.addrStr, a.connID)
				i++
			}
			for _, a := range regAppNw.snapshot() {
				fmt.Printf("Nw[%d] %s, %d\n", i, a.addrStr, a.connID)
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

func main() {
	// runtime.GOMAXPROCS(runtime.NumCPU())

	// go func() {
	// 	log.Println(http.ListenAndServe("localhost:9999", nil))
	// }()

	if runtime.GOOS != "linux" {
		go usrInput()
	}

	logInit()
	initPtlProfiles()
	initForwardPool()

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
