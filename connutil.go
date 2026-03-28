package main

import (
	"gfep/utils"
	"gfep/ziface"
	"gfep/znet"
	"log"
	"strconv"
	"sync"
	"time"
)

func asConn(c ziface.IConnection) *znet.Connection {
	co, _ := c.(*znet.Connection)
	return co
}

func touchRx(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastTouchRx(t)
		return
	}
	c.SetProperty("rtime", t)
}

func setRoutingAddr(c ziface.IConnection, addr string) {
	if co := asConn(c); co != nil {
		co.FastSetRoutingAddr(addr)
		return
	}
	c.SetProperty("addr", addr)
}

func setLtime(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastSetLtime(t)
		return
	}
	c.SetProperty("ltime", t)
}

func setHtime(c ziface.IConnection, t time.Time) {
	if co := asConn(c); co != nil {
		co.FastSetHtime(t)
		return
	}
	c.SetProperty("htime", t)
}

func setRoutingStatus(c ziface.IConnection, status int) {
	if co := asConn(c); co != nil {
		co.FastSetRoutingStatus(status)
		return
	}
	c.SetProperty("status", status)
}

// getConnStatus 读取业务状态；非 *znet.Connection 时回退 GetProperty。
// 与 ziface 其它实现并存时，status 解析失败返回 ok=false，调用方应 NeedStop。
func getConnStatus(c ziface.IConnection) (status int, ok bool) {
	if co := asConn(c); co != nil {
		s, _ := co.FastGetRouting()
		return s, true
	}
	v, err := c.GetProperty("status")
	if err != nil {
		return 0, false
	}
	s, tOK := v.(int)
	if !tOK {
		return 0, false
	}
	return s, true
}

// preAddrForTmnLogin 终端登录前已登记的地址（字符串）。
func preAddrForTmnLogin(c ziface.IConnection) string {
	if co := asConn(c); co != nil {
		_, a := co.FastGetRouting()
		return a
	}
	v, err := c.GetProperty("addr")
	if err != nil {
		return ""
	}
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return s
}

var msaStrPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 24)
		return &b
	},
}

// msaString 将 MSA 整型格式化为十进制字符串（与 strconv.Itoa 一致），复用缓冲降低分配。
func msaString(msa int) string {
	p := msaStrPool.Get().(*[]byte)
	b := *p
	b = strconv.AppendInt(b[:0], int64(msa), 10)
	s := string(b)
	*p = b
	msaStrPool.Put(p)
	return s
}

func logPkt(lg *log.Logger, prefix string, data []byte) {
	if lg == nil || !utils.GlobalObject.LogPacketHex {
		return
	}
	lg.Printf("%s: % X\n", prefix, data)
}
