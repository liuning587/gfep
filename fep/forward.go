package fep

import (
	"expvar"
	"gfep/bridge"
	"gfep/internal/logx"
	"gfep/utils"
	"gfep/ziface"
	"sync"
)

// ForwardQueueDrops 转发任务队列满时丢弃次数（可通过 expvar 或 /debug/vars 查看）。
var ForwardQueueDrops = expvar.NewInt("gfep_forward_queue_drops")

type fwdJob struct {
	sender ziface.IConnection
	dst    uint32
	data   []byte
}

var (
	fwdCh   chan fwdJob
	fwdOnce sync.Once
)

func initForwardPool() {
	fwdOnce.Do(func() {
		n := utils.GlobalObject.ForwardWorkers
		if n <= 0 {
			n = 32
		}
		q := utils.GlobalObject.ForwardQueueLen
		if q <= 0 {
			q = 16384
		}
		fwdCh = make(chan fwdJob, q)
		for i := 0; i < n; i++ {
			go forwardWorker()
		}
	})
}

func forwardWorker() {
	for j := range fwdCh {
		_ = j.sender.SendMsgByConnID(j.dst, j.data)
	}
}

// submitForward 异步转发（拷贝 payload）；队列满则丢弃并记日志。
func submitForward(sender ziface.IConnection, dst uint32, data []byte) {
	initForwardPool()
	p := make([]byte, len(data))
	copy(p, data)
	select {
	case fwdCh <- fwdJob{sender: sender, dst: dst, data: p}:
	default:
		ForwardQueueDrops.Add(1)
		logx.Warnf("forward queue full, drop dst=%d len=%d", dst, len(data))
	}
}

func stopBridgeForConnID(srv ziface.IServer, connID uint32) {
	if srv == nil {
		return
	}
	c, err := srv.GetConnMgr().Get(connID)
	if err != nil {
		return
	}
	b, err := c.GetProperty("bridge")
	if err != nil {
		return
	}
	if v, ok := b.(*bridge.Conn); ok {
		v.Stop()
	}
	c.RemoveProperty("bridge")
}
