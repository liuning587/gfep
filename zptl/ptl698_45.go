package zptl

import (
	"fmt"
	"strings"
	"time"
)

func ptl698_45HeadIsVaild(buf []byte) int32 {
	var fLen int32
	var hLen uint16
	var cs uint16

	if len(buf) < 1 {
		return 0
	}

	if 0x68 != buf[0] {
		return -1
	}

	if len(buf) < 3 {
		return 0
	}

	fLen = (int32(buf[2]&0x3f) << 8) + int32(buf[1])
	if buf[2]&0x40 != 0 {
		fLen *= 1024
	}
	if (fLen > Pmax698PtlFrameLen-2) || (fLen < 7) {
		return -1
	}

	if len(buf) < 6 {
		return 0
	}

	hLen = 6 + uint16(buf[4]&0x0f) + 1

	if len(buf) < int(hLen+2) {
		return 0
	}

	//check hcs
	cs = Crc16Calculate(buf[1:hLen])
	if cs != ((uint16(buf[hLen+1]) << 8) + uint16(buf[hLen])) {
		return -1
	}

	return int32(hLen + 2)
}

func ptl698_45IsVaild(buf []byte) int32 {
	var fLen int32
	var hlen uint16
	var cs uint16

	if len(buf) < 1 {
		return 0
	}

	if 0x68 != buf[0] {
		return -1
	}

	if len(buf) < 3 {
		return 0
	}

	fLen = (int32(buf[2]&0x3f) << 8) + int32(buf[1])
	if buf[2]&0x40 != 0 {
		fLen *= 1024
	}
	if (fLen > Pmax698PtlFrameLen-2) || (fLen < 7) {
		return -1
	}

	if len(buf) < 6 {
		return 0
	}

	hlen = 6 + uint16(buf[4]&0x0f) + 1

	if len(buf) < int(hlen+2) {
		return 0
	}

	//check hcs
	cs = Crc16Calculate(buf[1:hlen])
	if cs != ((uint16(buf[hlen+1]) << 8) + uint16(buf[hlen])) {
		return -1
	}

	if len(buf) < int(fLen+1) {
		return 0
	}

	//check fcs
	cs = Crc16Calculate(buf[1 : fLen-1])
	if cs != ((uint16(buf[fLen]) << 8) + uint16(buf[fLen-1])) {
		return -1
	}

	if len(buf) < int(fLen+2) {
		return 0
	}
	if 0x16 != buf[fLen+1] {
		return -1
	}

	return int32(fLen + 2)
}

//--------------------------------------------------------------------

// Ptl698_45GetDir 获取报文传输方向,0:主站-->终端, 1:终端-->主站
func Ptl698_45GetDir(buf []byte) int {
	if buf[3]&0x80 != 0 {
		return 1
	}
	return 0
}

// Ptl698_45GetFrameType 获取报文类型
func Ptl698_45GetFrameType(buf []byte) int {
	if buf[3]&0x07 == 0x01 { //链路连接管理（登录，心跳，退出登录）
		switch buf[7+buf[4]+2+2] {
		case 0:
			return LINK_LOGIN
		case 1:
			return LINK_HAERTBEAT
		case 2:
			return LINK_EXIT
		default:
			break
		}
	}
	return OTHER
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
	millisecond := t.Nanosecond() / 1e6
	buf[8] = byte(millisecond >> 8)
	buf[9] = byte(millisecond)
}

// Ptl698_45BuildReplyPacket 打包登陆、心跳回复包
func Ptl698_45BuildReplyPacket(in []byte, out []byte) int {
	out[0] = 0x68
	out[1] = 0x00
	out[2] = 0x00
	out[3] = 0x01
	//服务器地址、客户机地址
	for i := 0; i < int(in[4]+3); i++ {
		out[4+i] = in[4+i]
	}
	offset := int(4 + in[4]&0xf + 3) //起始1、长度2、控制域1、地址
	offset += 2

	out[offset+0] = 0x81
	out[offset+1] = 0x00
	out[offset+2] = 0x00
	offset += 3

	//请求时间
	for i := 0; i < 10; i++ {
		out[offset+i] = in[offset+2+i]
	}
	offset += 10

	//收到时间: todo: 更具系统时间获取
	getDataTime(out[offset:])

	//响应时间
	for i := 0; i < 10; i++ {
		out[offset+10+i] = out[offset+i]
	}
	offset += 20

	//长度区域
	out[1] = byte(((offset + 3 - 2) >> 0) & 0xff)
	out[2] = byte(((offset + 3 - 2) >> 8) & 0xff)

	offsetHcs := int(4 + in[4]&0xf + 3) //起始1、长度2、控制域1、地址
	crc := Crc16Calculate(out[1:offsetHcs])
	out[offsetHcs+0] = byte((crc >> 0) & 0xff)
	out[offsetHcs+1] = byte((crc >> 8) & 0xff)

	crc = Crc16Calculate(out[1:offset])
	out[offset+0] = byte((crc >> 0) & 0xff)
	out[offset+1] = byte((crc >> 8) & 0xff)

	out[offset+2] = 0x16

	offset += 3

	return offset
}

// Ptl698_45AddrCmp 终端地址比较
func Ptl698_45AddrCmp(addr []byte, buf []byte) bool {
	if buf[5] == 0xaa && buf[4] == 15 {
		return true
	}
	for i := 0; i <= int(addr[0]&0x0f)+2; i++ {
		if addr[i] != buf[i] {
			return false
		}
	}

	return true
}

// Ptl698_45AddrGet 从报文中取出终端地址
func Ptl698_45AddrGet(buf []byte) []byte {
	return buf[4 : 4+(buf[4]&0x0f)+2]
}

// Ptl698_45AddrStr 获取终端字符串
func Ptl698_45AddrStr(addr []byte) string {
	var sa = make([]string, 0)
	for _, v := range addr {
		sa = append(sa, fmt.Sprintf("%02X", v))
	}
	ss := strings.Join(sa, "")
	return ss
}

// Ptl698_45MsaCmp 主站MSA地址比较
func Ptl698_45MsaCmp(msa int, buf []byte) bool {
	if msa == int(buf[6+buf[4]&0x0f]) {
		return true
	}
	return false
}

// Ptl698_45MsaGet 从报文中取出主站MSA地址
func Ptl698_45MsaGet(buf []byte) int {
	return int(buf[6+buf[4]&0x0f])
}

// Ptl698_45IsMsaValid 判断主站发出的msa是否有效
func Ptl698_45IsMsaValid(msa int) bool {
	if msa != 0 {
		return true
	}
	return false
}
