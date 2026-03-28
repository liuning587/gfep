package main

import (
	"gfep/bridge"
	"gfep/utils"
	"gfep/ziface"
	"gfep/zptl"
	"log"
	"strings"
	"time"
)

// ptlProfile 376 / 698 / NW 共用处理骨架。
// 级联(SupportCasLink)、APP Online 响应等待办项仍依赖配置与现场扩展，此处保留原 todo 语义。
type ptlProfile struct {
	ptype  uint32
	regApp *appRegistry
	regTmn *tmnRegistry
	log    *log.Logger
	connA  int
	connT  int

	broadcast func(tmn string) bool

	msaGet       func([]byte) int
	getDir       func([]byte) int
	getFrameType func([]byte) int
	addrGet      func([]byte) []byte
	addrStr      func([]byte) string
	buildReply   func(in, out []byte) int

	extras698      bool
	isReport       func([]byte) bool
	buildReportAck func(in, out []byte) int
}

var profile376, profile698, profileNw ptlProfile

func terminalRxCat(p *ptlProfile, rData []byte) string {
	switch p.getFrameType(rData) {
	case zptl.LINK_LOGIN, zptl.LINK_EXIT, zptl.LINK_HAERTBEAT:
		return logCatLink
	default:
		if p.isReport != nil && p.isReport(rData) {
			return logCatReport
		}
		return logCatFwd
	}
}

func appRxCat(p *ptlProfile, rData []byte) string {
	if p.getFrameType(rData) == zptl.ONLINE {
		return logCatLink
	}
	return logCatFwd
}

func initPtlProfiles() {
	profile376 = ptlProfile{
		ptype:        zptl.PTL_1376_1,
		regApp:       regApp376,
		regTmn:       regTmn376,
		log:          log376,
		connA:        connA376,
		connT:        connT376,
		broadcast:    func(t string) bool { return strings.HasSuffix(t, "FF") },
		msaGet:       zptl.Ptl1376_1MsaGet,
		getDir:       zptl.Ptl1376_1GetDir,
		getFrameType: zptl.Ptl1376_1GetFrameType,
		addrGet:      zptl.Ptl1376_1AddrGet,
		addrStr:      zptl.Ptl1376_1AddrStr,
		buildReply:   zptl.Ptl1376_1BuildReplyPacket,
	}
	profile698 = ptlProfile{
		ptype:          zptl.PTL_698_45,
		regApp:         regApp698,
		regTmn:         regTmn698,
		log:            log698,
		connA:          connA698,
		connT:          connT698,
		broadcast:      func(t string) bool { return strings.HasSuffix(t, "AA") },
		msaGet:         zptl.Ptl698_45MsaGet,
		getDir:         zptl.Ptl698_45GetDir,
		getFrameType:   zptl.Ptl698_45GetFrameType,
		addrGet:        zptl.Ptl698_45AddrGet,
		addrStr:        zptl.Ptl698_45AddrStr,
		buildReply:     zptl.Ptl698_45BuildReplyPacket,
		extras698:      true,
		isReport:       zptl.Ptl698_45IsReport,
		buildReportAck: zptl.Ptl698_45BuildReportAckPacket,
	}
	profileNw = ptlProfile{
		ptype:        zptl.PTL_NW,
		regApp:       regAppNw,
		regTmn:       regTmnNw,
		log:          logNw,
		connA:        connANW,
		connT:        connTNW,
		broadcast:    func(t string) bool { return strings.HasSuffix(t, "FF") },
		msaGet:       zptl.PtlNwMsaGet,
		getDir:       zptl.PtlNwGetDir,
		getFrameType: zptl.PtlNwGetFrameType,
		addrGet:      zptl.PtlNwAddrGet,
		addrStr:      zptl.PtlNwAddrStr,
		buildReply:   zptl.PtlNwBuildReplyPacket,
	}
}

func (p *ptlProfile) Handle(request ziface.IRequest) {
	conn := request.GetConnection()
	if conn.IsStop() {
		return
	}
	rData := request.GetData()
	if !zptl.HandlerParseLenOK(p.ptype, rData) {
		return
	}
	connStatus, ok := getConnStatus(conn)
	if !ok {
		// 与 IConnection 其它实现保持行为：无法解析 status 则断开
		conn.NeedStop()
		return
	}

	now := time.Now()
	msaStr := msaString(p.msaGet(rData))
	tmnStr := p.addrStr(p.addrGet(rData))

	touchRx(conn, now)

	if p.getDir(rData) == 0 {
		p.handleFromApp(conn, connStatus, rData, msaStr, tmnStr)
		return
	}
	p.handleFromTerminal(conn, connStatus, rData, msaStr, tmnStr, now)
}

func (p *ptlProfile) handleFromApp(conn ziface.IConnection, connStatus int, rData []byte, msaStr, tmnStr string) {
	if connStatus != connIdle && connStatus != p.connA {
		conn.NeedStop()
		return
	}
	setRoutingStatus(conn, p.connA)
	logPktLine(p.log, "APP", "FEP", appRxCat(p, rData), conn.GetConnID(), rData)

	isNewApp := p.regApp.registerOrUpdate(conn.GetConnID(), msaStr)
	if isNewApp {
		setRoutingAddr(conn, msaStr)
		if p.log != nil {
			p.log.Printf("[APP->FEP][%s] app registered MSA=%s conn=%d\n", logCatLink, msaStr, conn.GetConnID())
		}
	}

	if p.getFrameType(rData) == zptl.ONLINE {
		// todo: 处理 app Online 响应
		return
	}

	targets := p.regTmn.forwardTargetsAppToTmn(tmnStr, p.broadcast(tmnStr))
	for _, id := range targets {
		logPktLine(p.log, "FEP", "DCU", logCatFwd, id, rData)
		submitForward(conn, id, rData)
	}
}

func (p *ptlProfile) handleFromTerminal(conn ziface.IConnection, connStatus int, rData []byte, msaStr, tmnStr string, now time.Time) {
	if connStatus != connIdle && connStatus != p.connT {
		conn.NeedStop()
		return
	}
	setRoutingStatus(conn, p.connT)
	logPktLine(p.log, "DCU", "FEP", terminalRxCat(p, rData), conn.GetConnID(), rData)

	// 698 上报且报文中主站 MSA=0（与 forward 桥接语义一致）
	if p.isReport != nil && p.isReport(rData) && p.msaGet(rData) == 0 {
		setLastReportAt(conn, now)
	}

	switch p.getFrameType(rData) {
	case zptl.LINK_LOGIN:
		if utils.GlobalObject.SupportCasLink {
			// todo: 级联终端登录（与 SupportCas / 多表位配合）
		}

		preAddr := preAddrForTmnLogin(conn)
		if preAddr != tmnStr {
			isNewTmn, evicted, resetBridgeCur := p.regTmn.Login(conn.GetConnID(), tmnStr, utils.GlobalObject.SupportCommTermianl, utils.GlobalObject.SupportCasLink)
			for _, id := range evicted {
				if p.log != nil {
					p.log.Printf("[DCU->FEP][%s] duplicate terminal login addr=%s evict conn=%d\n", logCatLink, tmnStr, id)
				}
				if p.extras698 {
					stopBridgeForConnID(utils.GlobalObject.TCPServer, id)
				}
			}
			if resetBridgeCur {
				if p.log != nil {
					p.log.Printf("[DCU->FEP][%s] login addr changed addr=%s reset bridge conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
				}
				if p.extras698 {
					stopBridgeForConnID(utils.GlobalObject.TCPServer, conn.GetConnID())
				}
			}
			if isNewTmn {
				if p.extras698 {
					p.tryStartBridge698(conn, rData)
				}
				if p.log != nil {
					p.log.Printf("[DCU->FEP][%s] terminal login addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
				}
			} else {
				if p.log != nil {
					p.log.Printf("[DCU->FEP][%s] terminal re-login addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
				}
			}
		} else {
			if p.log != nil {
				p.log.Printf("[DCU->FEP][%s] terminal re-login addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
			}
		}

		var reply [128]byte
		plen := p.buildReply(rData, reply[:])
		if err := conn.SendBuffMsg(reply[:plen]); err != nil {
			if p.log != nil {
				p.log.Printf("[FEP->DCU][%s] login reply send failed conn=%d: %v\n", logCatLink, conn.GetConnID(), err)
			}
		} else {
			setLtime(conn, time.Now())
			setRoutingAddr(conn, tmnStr)
			if p.log != nil {
				logPktLine(p.log, "FEP", "DCU", logCatLink, conn.GetConnID(), reply[:plen])
			}
		}
		return

	case zptl.LINK_HAERTBEAT:
		if utils.GlobalObject.SupportReplyHeart {
			if connStatus != p.connT {
				if p.log != nil {
					p.log.Printf("[DCU->FEP][%s] heartbeat before login addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
				}
				conn.NeedStop()
			} else {
				preAddr := preAddrForTmnLogin(conn)
				if preAddr == tmnStr {
					// todo: 级联心跳时判断级联地址是否存在
					if p.log != nil {
						p.log.Printf("[DCU->FEP][%s] heartbeat addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
					}
					setHtime(conn, time.Now())
					var reply [128]byte
					plen := p.buildReply(rData, reply[:])
					if err := conn.SendBuffMsg(reply[:plen]); err != nil {
						if p.log != nil {
							p.log.Printf("[FEP->DCU][%s] heartbeat reply send failed conn=%d: %v\n", logCatLink, conn.GetConnID(), err)
						}
					} else {
						if p.log != nil {
							logPktLine(p.log, "FEP", "DCU", logCatLink, conn.GetConnID(), reply[:plen])
						}
					}
				} else {
					if p.log != nil {
						p.log.Printf("[DCU->FEP][%s] heartbeat addr mismatch registered=%s frame=%s conn=%d\n", logCatLink, preAddr, tmnStr, conn.GetConnID())
					}
					conn.NeedStop()
				}
			}
			return
		}

	case zptl.LINK_EXIT:
		if connStatus != p.connT {
			if p.log != nil {
				p.log.Printf("[DCU->FEP][%s] logout before login addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
			}
		} else {
			if p.log != nil {
				p.log.Printf("[DCU->FEP][%s] terminal logout addr=%s conn=%d\n", logCatLink, tmnStr, conn.GetConnID())
			}
			var reply [128]byte
			plen := p.buildReply(rData, reply[:])
			if err := conn.SendMsg(reply[:plen]); err != nil {
				if p.log != nil {
					p.log.Printf("[FEP->DCU][%s] logout reply send failed conn=%d: %v\n", logCatLink, conn.GetConnID(), err)
				}
			} else if p.log != nil && plen > 0 {
				logPktLine(p.log, "FEP", "DCU", logCatLink, conn.GetConnID(), reply[:plen])
			}
		}
		conn.NeedStop()
		return

	default:
		break
	}

	if p.extras698 && utils.GlobalObject.SupportReplyReport && p.isReport != nil && p.buildReportAck != nil && p.isReport(rData) {
		var reply [512]byte
		plen := p.buildReportAck(rData, reply[:])
		if err := conn.SendBuffMsg(reply[:plen]); err != nil {
			if p.log != nil {
				p.log.Printf("[FEP->DCU][%s] report ack send failed conn=%d: %v\n", logCatRptAck, conn.GetConnID(), err)
			}
		} else if p.log != nil && plen > 0 {
			logPktLine(p.log, "FEP", "DCU", logCatRptAck, conn.GetConnID(), reply[:plen])
		}
	}

	targets := p.regApp.forwardTargets(msaStr)
	isMatch := len(targets) > 0
	for _, id := range targets {
		logPktLine(p.log, "FEP", "APP", logCatFwd, id, rData)
		submitForward(conn, id, rData)
	}

	if p.extras698 && (msaStr == "0" || !isMatch) {
		b, err := conn.GetProperty("bridge")
		if err == nil {
			if v, ok := b.(*bridge.Conn); ok {
				_ = v.Send(rData)
			}
		}
	}
}

func (p *ptlProfile) tryStartBridge698(conn ziface.IConnection, rData []byte) {
	host := utils.GlobalObject.BridgeHost698
	if len(host) == 0 || host[0] == '0' {
		return
	}
	if ob, err := conn.GetProperty("bridge"); err == nil {
		if v, ok := ob.(*bridge.Conn); ok {
			v.Stop()
		}
	}
	tsa := zptl.Ptl698_45AddrGet(rData)
	b := bridge.NewConn(host, tsa[1:], zptl.PTL_698_45, time.Minute, func(data []byte) {
		if err := conn.SendBuffMsg(data); err != nil {
			if p.log != nil {
				p.log.Printf("[FEP->DCU][%s] bridge downlink send failed conn=%d: %v\n", logCatFwd, conn.GetConnID(), err)
			}
		} else if p.log != nil {
			logPktLine(p.log, "FEP", "DCU", logCatFwd, conn.GetConnID(), data)
		}
	})
	b.Start()
	conn.SetProperty("bridge", b)
	if p.log != nil {
		p.log.Printf("[FEP->BRG][%s] dial host=%s TSA=% X dcuConn=%d\n", logCatFwd, host, tsa, conn.GetConnID())
	}
}
