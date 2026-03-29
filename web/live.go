package web

import (
	"encoding/json"
	"gfep/utils"
	"sync"
	"time"
)

// LiveEvent 推送到浏览器 SSE 的单条内容（line 可为 JSON 或纯文本）。
type LiveEvent struct {
	TS   time.Time `json:"ts"`
	Line string    `json:"line"`
}

const (
	liveBufSize   = 64
	liveMaxSub    = 64
)

var (
	liveMu   sync.Mutex
	liveSubs map[chan LiveEvent]struct{}
)

// InitLive 初始化订阅表（Listen 前调用）。
func InitLive() {
	liveMu.Lock()
	if liveSubs == nil {
		liveSubs = make(map[chan LiveEvent]struct{})
	}
	liveMu.Unlock()
}

func liveSubscribe() (ch chan LiveEvent, ok bool) {
	InitLive()
	liveMu.Lock()
	if len(liveSubs) >= liveMaxSub {
		liveMu.Unlock()
		return nil, false
	}
	ch = make(chan LiveEvent, liveBufSize)
	liveSubs[ch] = struct{}{}
	liveMu.Unlock()
	return ch, true
}

func liveUnsubscribe(ch chan LiveEvent) {
	if ch == nil {
		return
	}
	liveMu.Lock()
	_, was := liveSubs[ch]
	if was {
		delete(liveSubs, ch)
	}
	liveMu.Unlock()
	if was {
		close(ch)
	}
}

// PublishLive 非阻塞广播；慢客户端丢事件。
func PublishLive(ev LiveEvent) {
	if !utils.GlobalObject.LogWebEnabled {
		return
	}
	liveMu.Lock()
	chans := make([]chan LiveEvent, 0, len(liveSubs))
	for ch := range liveSubs {
		chans = append(chans, ch)
	}
	liveMu.Unlock()
	for _, ch := range chans {
		select {
		case ch <- ev:
		default:
		}
	}
}

// PublishLivef 推送一行 UTF-8 文本（如链路层日志）。
func PublishLivef(line string) {
	PublishLive(LiveEvent{TS: time.Now(), Line: line})
}

// PublishLiveJSON 将结构化对象序列化为单行 JSON（供按 protocol/addr 过滤）。
func PublishLiveJSON(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	PublishLive(LiveEvent{TS: time.Now(), Line: string(b)})
}
