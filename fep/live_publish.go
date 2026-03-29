package fep

import (
	"gfep/utils"
	"gfep/web"
	"gfep/znet"
)

// liveStreamLinkMeta 按 connID 解析规约标签与终端/主站侧地址（用于 Web 实时过滤）。
func liveStreamLinkMeta(connID uint32) (protocol, addr, remoteTCP string) {
	if a, ok := regTmn376.addrForConn(connID); ok {
		return "376/1376-1", a, remoteTCPForConn(connID)
	}
	if a, ok := regTmn698.addrForConn(connID); ok {
		return "698-45", a, remoteTCPForConn(connID)
	}
	if a, ok := regTmnNw.addrForConn(connID); ok {
		return "NW", a, remoteTCPForConn(connID)
	}
	if msa, ok := regApp376.msaForConn(connID); ok {
		return "376-主站", msa, remoteTCPForConn(connID)
	}
	if msa, ok := regApp698.msaForConn(connID); ok {
		return "698-主站", msa, remoteTCPForConn(connID)
	}
	if msa, ok := regAppNw.msaForConn(connID); ok {
		return "Nw-主站", msa, remoteTCPForConn(connID)
	}
	d, ok := connDetailsOrEmpty(connID)
	if ok {
		return "", d.TermAddr, d.RemoteTCP
	}
	return "", "", ""
}

func remoteTCPForConn(connID uint32) string {
	d, ok := connDetailsOrEmpty(connID)
	if !ok {
		return ""
	}
	return d.RemoteTCP
}

func relayWebPacketLine(from, to, cat string, connID uint32, hexBody, emptyNote string) {
	if !utils.GlobalObject.LogWebEnabled {
		return
	}
	proto, addr, rtcp := liveStreamLinkMeta(connID)
	if addr == "" {
		if srv := utils.GlobalObject.TCPServer; srv != nil {
			if ic, err := srv.GetConnMgr().Get(connID); err == nil {
				if co, ok := ic.(*znet.Connection); ok {
					_, addr = co.FastGetRouting()
				}
			}
		}
	}
	body := hexBody
	if emptyNote != "" {
		body = emptyNote
	}
	web.PublishLiveJSON(struct {
		Kind      string `json:"kind"`
		From      string `json:"from"`
		To        string `json:"to"`
		Cat       string `json:"cat"`
		ConnID    uint32 `json:"connId"`
		Protocol  string `json:"protocol,omitempty"`
		Addr      string `json:"addr,omitempty"`
		RemoteTCP string `json:"remoteTcp,omitempty"`
		Hex       string `json:"hex,omitempty"`
	}{Kind: "pkt", From: from, To: to, Cat: cat, ConnID: connID, Protocol: proto, Addr: addr, RemoteTCP: rtcp, Hex: body})
}
