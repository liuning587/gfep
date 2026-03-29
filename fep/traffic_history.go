package fep

import (
	"strconv"
	"sync"
	"time"

	"gfep/web"
)

func u64sub(a, b uint64) uint64 {
	if a >= b {
		return a - b
	}
	return 0
}

// sumTerminalConnBytes 当前所有终端 TCP 连接的收/发字节累计（与在线终端列表数据源一致）。
func sumTerminalConnBytes() (rx, tx uint64) {
	type item struct {
		ac addrConnID
	}
	var items []item
	for _, t := range regTmn376.snapshot() {
		items = append(items, item{t})
	}
	for _, t := range regTmn698.snapshot() {
		items = append(items, item{t})
	}
	for _, t := range regTmnNw.snapshot() {
		items = append(items, item{t})
	}
	for _, it := range items {
		d, ok := connDetailsOrEmpty(it.ac.connID)
		if !ok {
			continue
		}
		rx += d.RxFrameBytes
		tx += d.TxWriteBytes
	}
	return rx, tx
}

// sumAppConnBytes 当前所有主站/APP 连接的收/发字节累计。
func sumAppConnBytes() (rx, tx uint64) {
	type item struct {
		ac addrConnID
	}
	var items []item
	for _, a := range regApp376.snapshot() {
		items = append(items, item{a})
	}
	for _, a := range regApp698.snapshot() {
		items = append(items, item{a})
	}
	for _, a := range regAppNw.snapshot() {
		items = append(items, item{a})
	}
	for _, it := range items {
		d, ok := connDetailsOrEmpty(it.ac.connID)
		if !ok {
			continue
		}
		rx += d.RxFrameBytes
		tx += d.TxWriteBytes
	}
	return rx, tx
}

const trafficMinBuckets = 15

var (
	trafficHistMu   sync.Mutex
	termMinRing     []uint64
	appMinRing      []uint64
	prevTRx         uint64
	prevTTx         uint64
	prevARx         uint64
	prevATx         uint64
	trafficHavePrev bool
)

func pushRing15(ring []uint64, v uint64) []uint64 {
	ring = append(ring, v)
	if len(ring) > trafficMinBuckets {
		ring = ring[len(ring)-trafficMinBuckets:]
	}
	return ring
}

func pad15Ring(ring []uint64) []uint64 {
	out := make([]uint64, trafficMinBuckets)
	n := len(ring)
	if n >= trafficMinBuckets {
		copy(out, ring[n-trafficMinBuckets:])
	} else if n > 0 {
		copy(out[trafficMinBuckets-n:], ring)
	}
	return out
}

func trafficHistoryTick() {
	tRx, tTx := sumTerminalConnBytes()
	aRx, aTx := sumAppConnBytes()
	trafficHistMu.Lock()
	defer trafficHistMu.Unlock()
	if trafficHavePrev {
		dt := u64sub(tRx, prevTRx) + u64sub(tTx, prevTTx)
		da := u64sub(aRx, prevARx) + u64sub(aTx, prevATx)
		termMinRing = pushRing15(termMinRing, dt)
		appMinRing = pushRing15(appMinRing, da)
	}
	prevTRx, prevTTx, prevARx, prevATx = tRx, tTx, aRx, aTx
	trafficHavePrev = true
}

// fepWebTrafficSnapshot 供总览：当前累计流量 + 最近 15 个完整分钟的每分钟总字节（终端 / 主站各一条序列）。
func fepWebTrafficSnapshot() web.TrafficStatus {
	tRx, tTx := sumTerminalConnBytes()
	aRx, aTx := sumAppConnBytes()
	trafficHistMu.Lock()
	defer trafficHistMu.Unlock()
	return web.TrafficStatus{
		TerminalTotalRx:     strconv.FormatUint(tRx, 10),
		TerminalTotalTx:     strconv.FormatUint(tTx, 10),
		AppTotalRx:          strconv.FormatUint(aRx, 10),
		AppTotalTx:          strconv.FormatUint(aTx, 10),
		TerminalBytesPerMin: pad15Ring(termMinRing),
		AppBytesPerMin:      pad15Ring(appMinRing),
	}
}

// startTrafficHistorySampler 每分钟采样一次，用于总览近 15 分钟流量曲线（Web 启用时由 logweb 启动）。
func startTrafficHistorySampler() {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			trafficHistoryTick()
		}
	}()
}
