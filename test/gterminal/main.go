package main

import (
	"bytes"
	"encoding/hex"
	"flag"
	"fmt"
	"net"
	"os"
	"sync"
	"time"

	"gfep/zptl"
)

// 主站下行抄读电压（示例）；地址域为 0x05 + 6 字节通信地址，其后为固定 APDU 前缀。
var voltageReadMark = []byte{
	0x52, 0xE0, 0xD7, 0x05, 0x01, 0x3F, 0x20, 0x00, 0x02, 0x00, 0x00,
}

// 终端上行应答模板（总长 38，L=0x24）；仅地址与 CRC 需按实际终端重写。
var voltageRespTpl = []byte{
	0x68, 0x24, 0x00, 0xC3, 0x05, 0x27, 0x03, 0x00, 0x00, 0x00, 0x26,
	0x52, 0x4B, 0x7E, 0x85, 0x01, 0x3F, 0x20, 0x00, 0x02, 0x00, 0x01, 0x01, 0x03,
	0x12, 0x08, 0x9C, 0x12, 0x08, 0x98, 0x12, 0x08, 0x99, 0x00, 0x00,
	0x15, 0x6E, 0x16,
}

// 常见主站/工具在 0x4F 后连续 AA（16/24 等）再跟 11 字节固定前缀（与规范「52+HCS+8 字节用户数据」并列存在）。
var timeBroadcastWireMark = []byte{
	0x52, 0xDA, 0x7D, 0x05, 0x01, 0x01, 0x40, 0x00, 0x02, 0x00, 0x00,
}

// 规范 L=0x21：紧跟 52 之后为 2 字节 HCS，再 8 字节用户数据。
var timeBroadcastStrictUser = []byte{0xDA, 0x7D, 0x05, 0x01, 0x01, 0x40, 0x00, 0x02}

// 广播读时钟应答模板（总长 35，L=0x21）；仅将 [5:11] 换为本终端通信地址并重算 HCS/FCS。
var timeReadRespTpl = []byte{
	0x68, 0x21, 0x00, 0xC3, 0x05, 0x01, 0x00, 0x00, 0x00, 0x00, 0x94,
	0x52, 0x62, 0x4B, 0x85, 0x01, 0x01, 0x40, 0x00, 0x02, 0x00, 0x01,
	0x1C, 0x07, 0xEB, 0x04, 0x0B, 0x07, 0x11, 0x10, 0x00, 0x00,
	0x3B, 0x43, 0x16,
}

func parseAddr(s string) ([]byte, error) {
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, err
	}
	if len(b) != 6 {
		return nil, fmt.Errorf("need 12 hex chars (6 bytes), got %d bytes", len(b))
	}
	return b, nil
}

func addrForInstance(base []byte, cNo int, unique bool) []byte {
	a := make([]byte, 6)
	copy(a, base)
	if unique {
		a[4] = byte((cNo >> 8) & 0xff)
		a[5] = byte(cNo & 0xff)
	}
	return a
}

func recalc698CRCs(frame []byte) error {
	if len(frame) < 12 {
		return fmt.Errorf("frame too short")
	}
	fLen := int(frame[1]) | int(frame[2]&0x3f)<<8
	if frame[2]&0x40 != 0 {
		return fmt.Errorf("extended length not supported")
	}
	if len(frame) < fLen+2 {
		return fmt.Errorf("buffer shorter than L+2")
	}
	hlen := 6 + int(frame[4]&0x0f) + 1
	if hlen+2 > len(frame) {
		return fmt.Errorf("bad hlen")
	}
	cs := zptl.Crc16Calculate(frame[1:hlen])
	frame[hlen] = byte(cs)
	frame[hlen+1] = byte(cs >> 8)
	cs = zptl.Crc16Calculate(frame[1 : fLen-1])
	frame[fLen-1] = byte(cs)
	frame[fLen] = byte(cs >> 8)
	return nil
}

// buildLoginFrame 与原先 32 字节登录结构一致，仅将通信地址写入 [5:11)。
func buildLoginFrame(addr []byte) ([]byte, error) {
	if len(addr) != 6 {
		return nil, fmt.Errorf("addr len %d", len(addr))
	}
	sbuf := make([]byte, 32)
	sbuf[0] = 0x68
	sbuf[1] = 0x1E
	sbuf[2] = 0x00
	sbuf[3] = 0x81
	sbuf[4] = 0x05
	copy(sbuf[5:11], addr)
	sbuf[11] = 0x00
	cs := zptl.Crc16Calculate(sbuf[1:12])
	sbuf[12] = byte(cs)
	sbuf[13] = byte(cs >> 8)
	sbuf[14] = 0x01
	sbuf[15] = 0x00
	sbuf[16] = 0x00
	sbuf[17] = 0x00
	sbuf[18] = 0x3C
	getDataTime(sbuf[19:29])
	cs = zptl.Crc16Calculate(sbuf[1:29])
	sbuf[29] = byte(cs)
	sbuf[30] = byte(cs >> 8)
	sbuf[31] = 0x16
	return sbuf, nil
}

func getDataTime(buf []byte) {
	t := time.Now().Local()
	year := uint16(t.Year())
	buf[0] = byte(year >> 8)
	buf[1] = byte(year)
	buf[2] = byte(t.Month())
	buf[3] = byte(t.Day())
	buf[4] = byte(t.Weekday())
	buf[5] = byte(t.Hour())
	buf[6] = byte(t.Minute())
	buf[7] = byte(t.Second())
	ms := t.Nanosecond() / 1e6
	buf[8] = byte(ms >> 8)
	buf[9] = byte(ms)
}

func isVoltageReadRequest(frame, commAddr []byte) bool {
	if len(commAddr) != 6 || len(frame) < 11+len(voltageReadMark) {
		return false
	}
	if zptl.Ptl698_45GetDir(frame) != 0 {
		return false
	}
	if frame[4] != 0x05 {
		return false
	}
	if !bytes.Equal(frame[5:11], commAddr) {
		return false
	}
	return bytes.Equal(frame[11:11+len(voltageReadMark)], voltageReadMark)
}

// broadcastTimeReadTotalLen 从 buf[0] 起识别「广播读时钟」并返回整帧字节数；不校验 HCS/FCS。
func broadcastTimeReadTotalLen(buf []byte) int {
	minNeed := 5 + 1 + 2 + len(timeBroadcastStrictUser) + 2 + 1
	if len(buf) < minNeed {
		return 0
	}
	if buf[0] != 0x68 || buf[1] != 0x21 || buf[2] != 0x00 || buf[3] != 0x43 || buf[4] != 0x4F {
		return 0
	}
	if zptl.Ptl698_45GetDir(buf) != 0 {
		return 0
	}
	p := 5
	for p < len(buf) && buf[p] == 0xAA {
		p++
	}
	if p == 5 {
		return 0
	}
	// A) 工具常见：AA 后直接 11 字节 wireMark + FCS + 16（总长随 AA 个数变，如 43 字节）
	if p+len(timeBroadcastWireMark)+2+1 <= len(buf) &&
		bytes.Equal(buf[p:p+len(timeBroadcastWireMark)], timeBroadcastWireMark) &&
		buf[p+len(timeBroadcastWireMark)+2] == 0x16 {
		return p + len(timeBroadcastWireMark) + 2 + 1
	}
	// B) 规范布局：52 + HCS(2) + 8 字节用户数据 + FCS(2) + 16（总长 5 + nAA + 14）
	if p+1+2+len(timeBroadcastStrictUser)+2+1 <= len(buf) &&
		buf[p] == 0x52 &&
		bytes.Equal(buf[p+3:p+3+len(timeBroadcastStrictUser)], timeBroadcastStrictUser) &&
		buf[p+1+2+len(timeBroadcastStrictUser)+2] == 0x16 {
		return p + 1 + 2 + len(timeBroadcastStrictUser) + 2 + 1
	}
	return 0
}

func isBroadcastTimeReadLayout(frame []byte) bool {
	want := broadcastTimeReadTotalLen(frame)
	return want > 0 && want == len(frame)
}

func buildVoltageResponse(commAddr []byte) ([]byte, error) {
	out := make([]byte, len(voltageRespTpl))
	copy(out, voltageRespTpl)
	copy(out[5:11], commAddr)
	if err := recalc698CRCs(out); err != nil {
		return nil, err
	}
	return out, nil
}

func buildTimeBroadcastResponse(commAddr []byte) ([]byte, error) {
	out := make([]byte, len(timeReadRespTpl))
	copy(out, timeReadRespTpl)
	copy(out[5:11], commAddr)
	if err := recalc698CRCs(out); err != nil {
		return nil, err
	}
	return out, nil
}

// drainProtocolFrames 先按 zptl 完整校验拆帧；对「广播读时钟」在 CRC 错误时仍按 35 字节结构拆出（与常见错误拼帧工具兼容）。
func drainProtocolFrames(buf []byte, onFrame func([]byte)) []byte {
	for len(buf) > 0 {
		if buf[0] != 0x68 {
			buf = buf[1:]
			continue
		}
		n := zptl.Ptl698_45CompleteFrameLen(buf)
		if n > 0 {
			fl := int(n)
			frame := append([]byte(nil), buf[:fl]...)
			buf = buf[fl:]
			onFrame(frame)
			continue
		}
		bl := broadcastTimeReadTotalLen(buf)
		if bl > 0 {
			frame := append([]byte(nil), buf[:bl]...)
			buf = buf[bl:]
			onFrame(frame)
			continue
		}
		if n == 0 {
			break
		}
		buf = buf[1:]
	}
	return buf
}

func runTerminal(cNo int, server string, commAddr []byte, verbose bool) {
	defer wg.Done()

	conn, err := net.Dial("tcp", server)
	if err != nil {
		fmt.Fprintf(os.Stderr, "dial %s #%d: %v\n", server, cNo, err)
		return
	}
	defer conn.Close()

	login, err := buildLoginFrame(commAddr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "login frame #%d: %v\n", cNo, err)
		return
	}
	if _, err := conn.Write(login); err != nil {
		fmt.Fprintf(os.Stderr, "login write #%d: %v\n", cNo, err)
		return
	}
	if verbose {
		fmt.Printf("[%d] login sent addr=% X % X\n", cNo, commAddr, login)
	}

	var buf []byte
	tmp := make([]byte, 2048)
	for {
		n, err := conn.Read(tmp)
		if err != nil {
			if verbose {
				fmt.Printf("[%d] read end: %v\n", cNo, err)
			}
			return
		}
		buf = append(buf, tmp[:n]...)
		buf = drainProtocolFrames(buf, func(frame []byte) {
			if verbose {
				fmt.Printf("[%d] rx frame % X\n", cNo, frame)
			}
			if isVoltageReadRequest(frame, commAddr) {
				resp, err := buildVoltageResponse(commAddr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[%d] voltage resp build: %v\n", cNo, err)
					return
				}
				if _, err := conn.Write(resp); err != nil {
					fmt.Fprintf(os.Stderr, "[%d] voltage resp write: %v\n", cNo, err)
					return
				}
				if verbose {
					fmt.Printf("[%d] voltage reply % X\n", cNo, resp)
				}
			} else if isBroadcastTimeReadLayout(frame) {
				resp, err := buildTimeBroadcastResponse(commAddr)
				if err != nil {
					fmt.Fprintf(os.Stderr, "[%d] broadcast time resp build: %v\n", cNo, err)
					return
				}
				if _, err := conn.Write(resp); err != nil {
					fmt.Fprintf(os.Stderr, "[%d] broadcast time resp write: %v\n", cNo, err)
					return
				}
				if verbose {
					fmt.Printf("[%d] broadcast time reply % X\n", cNo, resp)
				}
			}
		})
		if len(buf) > zptl.PmaxPtlFrameLen*4 {
			buf = buf[len(buf)-zptl.PmaxPtlFrameLen*2:]
		}
	}
}

var wg sync.WaitGroup

func main() {
	server := flag.String("server", "127.0.0.1:20083", "FEP address host:port")
	nConn := flag.Int("n", 1, "concurrent simulated terminals")
	addrHex := flag.String("addr", "270300000026", "6-byte comm addr as 12 hex chars")
	unique := flag.Bool("unique", false, "vary last 2 addr bytes per instance (cNo) for scale-out sim")
	verbose := flag.Bool("v", false, "verbose hex log")
	flag.Parse()

	baseAddr, err := parseAddr(*addrHex)
	if err != nil {
		fmt.Fprintln(os.Stderr, "gfep: -addr:", err)
		os.Exit(1)
	}

	fmt.Printf("gfep gterminal: server=%s n=%d addr=%s unique=%v (698 voltage + broadcast time read)\n",
		*server, *nConn, *addrHex, *unique)

	for i := 0; i < *nConn; i++ {
		cNo := i
		comm := addrForInstance(baseAddr, cNo, *unique)
		wg.Add(1)
		go runTerminal(cNo, *server, comm, *verbose)
	}
	wg.Wait()
}
