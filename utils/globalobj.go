package utils

import (
	"encoding/json"
	"gfep/ziface"
	"gfep/zlog"
	"log"
	"os"
)

// GlobalObj 存储一切有关Zinx框架的全局参数，供其他模块使用一些参数也可以通过 用户根据 zinx.json来配置
type GlobalObj struct {
	/*
		Server
	*/
	TCPServer     ziface.IServer //当前Zinx的全局Server对象
	Host          string         //监听地址：IPv4 如 0.0.0.0；IPv6 如 :: 或 ::1（勿再套方括号，除非整段 [::]）
	TCPPort       int            //当前服务器主机监听端口号
	TCPNetwork    string         //监听网络：tcp（推荐，支持 IPv4/IPv6 解析）、tcp4、tcp6；空等价 tcp
	BridgeHost698 string         //698桥接主机IP
	Name          string         //当前服务器名称

	/*
		Zinx
	*/
	Version          string //当前Zinx版本号
	MaxPacketSize    uint32 //都需数据包的最大值
	MaxConn          int    //当前服务器主机允许的最大链接个数
	WorkerPoolSize   uint32 //业务工作Worker池的数量
	MaxWorkerTaskLen uint32 //业务工作Worker对应负责的任务队列最大任务存储数量
	MaxMsgChanLen    uint32 //SendBuffMsg发送消息的缓冲最大长度

	/*
		config file path
	*/
	ConfFilePath string

	/*
		logger
	*/
	LogDir        string //日志所在文件夹 默认"./log"
	LogFile       string //日志文件名称   默认""  --如果没有设置日志文件，打印信息将打印至stderr
	LogWebEnabled bool   //是否启用 HTTP 列举/下载 LogDir 下日志（默认关闭，勿对公网裸奔）
	LogWebHost    string //日志 Web 监听 IP，空则 0.0.0.0；仅当 LogWebEnabled 时有效
	LogWebPort    int    //日志 Web 监听端口，<=0 时用 20084；仅当 LogWebEnabled 时有效
	LogDebugClose bool   //是否关闭Debug日志级别调试信息 默认false  -- 默认打开debug信息
	LogConnTrace  bool   //是否打印每条连接的 Accept/Add/Remove 等跟踪日志（高并发请关闭）
	LogNetVerbose bool   //是否打印 Worker 启动、路由注册等网络框架详细日志（默认关闭）
	LogPacketHex  bool   //是否对每条 A:/T: 报文打十六进制日志（高 QPS 请关闭）
	// LogForwardEgressHex 为 true 时，异步转发再各打一行 [FEP->DCU]/[FEP->APP]（与入站 FORWARD 帧相同 hex，默认 false 避免一轮请求打四条）
	LogForwardEgressHex bool

	/*
		Fep
	*/
	ForwardWorkers      int  //异步转发 worker 数量，<=0 时用默认 32
	ForwardQueueLen     int  //转发任务队列长度，<=0 时用默认 16384
	Timeout             int  //TCP连接超时时间(单位:分钟)
	SupportCompress     bool //是否支持加密(南网使用)
	SupportCas          bool //是否级联(南网使用)
	SupportCasLink      bool //是否级联终端登陆、心跳(南网使用)
	SupportCommTermianl bool //是否支持终端重复登陆(Y/N)
	SupportReplyHeart   bool //是否支持前置机维护心跳(Y/N)
	SupportReplyReport  bool //是否支持前置机确认上报(Y/N)
}

// GlobalObject 定义一个全局的对象
var GlobalObject *GlobalObj

// PathExists 判断一个文件是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// Reload 读取用户的配置文件
func (g *GlobalObj) Reload() {

	if confFileExists, _ := PathExists(g.ConfFilePath); !confFileExists {
		//fmt.Println("Config File ", g.ConfFilePath , " is not exist!!")
		return
	}

	data, err := os.ReadFile(g.ConfFilePath)
	if err != nil {
		log.Printf("gfep: read config %s: %v", g.ConfFilePath, err)
		return
	}
	loaded := *g
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("gfep: parse config %s: %v", g.ConfFilePath, err)
		return
	}
	*g = loaded

	//Logger 设置
	if g.LogFile != "" {
		zlog.SetLogFile(g.LogDir, g.LogFile)
	}
	if g.LogDebugClose {
		zlog.CloseDebug()
	}
}

// 提供init方法，默认加载
func init() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	pwd, err := os.Getwd()
	if err != nil {
		pwd = "."
	}
	//初始化GlobalObject变量，设置一些默认值
	GlobalObject = &GlobalObj{
		Name:                "gfep",
		Version:             "V0.3",
		TCPPort:             20083,
		Host:                "0.0.0.0",
		TCPNetwork:          "tcp",
		BridgeHost698:       "", //0.0.0.0:0
		MaxConn:             50000,
		MaxPacketSize:       2200,
		ConfFilePath:        pwd + "/conf/gfep.json",
		WorkerPoolSize:      256,
		MaxWorkerTaskLen:    1024,
		MaxMsgChanLen:       8,
		LogDir:              pwd + "/log",
		LogFile:             "",
		LogWebEnabled:       false,
		LogWebHost:          "0.0.0.0",
		LogWebPort:          20084,
		LogDebugClose:       false,
		LogConnTrace:        false,
		LogNetVerbose:       false,
		LogPacketHex:        false,
		LogForwardEgressHex: false,
		ForwardWorkers:      32,
		ForwardQueueLen:     16384,

		Timeout:             30,
		SupportCompress:     false,
		SupportCas:          false,
		SupportCasLink:      false,
		SupportCommTermianl: true,
		SupportReplyHeart:   true,
		SupportReplyReport:  false,
	}

	//从配置文件中加载一些用户配置的参数
	GlobalObject.Reload()
}
