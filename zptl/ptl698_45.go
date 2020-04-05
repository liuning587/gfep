package zptl

import (
	"fmt"
	"strings"
)

func ptl698_45HeadIsVaild(buf []byte) int32 {
	var fLen uint16
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

	fLen = (uint16(buf[2]&0x3f) << 8) + uint16(buf[1])
	if (fLen > PmaxPtlFrameLen-2) || (fLen < 7) {
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
	var flen uint16
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

	flen = (uint16(buf[2]&0x3f) << 8) + uint16(buf[1])
	if (flen > PmaxPtlFrameLen-2) || (flen < 7) {
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

	if len(buf) < int(flen+1) {
		return 0
	}

	//check fcs
	cs = Crc16Calculate(buf[1 : flen-1])
	if cs != ((uint16(buf[flen]) << 8) + uint16(buf[flen-1])) {
		return -1
	}

	if len(buf) < int(flen+2) {
		return 0
	}
	if 0x16 != buf[flen+1] {
		return -1
	}

	return int32(flen + 2)
}

//--------------------------------------------------------------------
//获取报文传输方向,0:主站-->终端, 1:终端-->主站
func Ptl698_45GetDir(buf []byte) int {
	if buf[3]&0x80 != 0 {
		return 1
	}
	return 0
}

//获取报文类型
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

//打包登陆、心跳回复包
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
	crc := Crc16Calculate(out[1:offset])
	out[offset+0] = byte((crc >> 0) & 0xff)
	out[offset+1] = byte((crc >> 8) & 0xff)
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
	for i := 0; i < 10; i++ {
		out[offset+i] = 0x00
	}

	//响应时间
	for i := 0; i < 10; i++ {
		out[offset+10+i] = out[offset+i]
	}
	offset += 20

	//长度区域
	out[1] = byte(((offset + 3 - 2) >> 0) & 0xff)
	out[2] = byte(((offset + 3 - 2) >> 8) & 0xff)

	crc = Crc16Calculate(out[1:offset])
	out[offset+0] = byte((crc >> 0) & 0xff)
	out[offset+1] = byte((crc >> 8) & 0xff)

	out[offset+2] = 0x16

	offset += 3

	return offset
}

//终端地址比较
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

//从报文中取出终端地址
func Ptl698_45AddrGet(buf []byte) []byte {
	return buf[4 : 4+(buf[4]&0x0f)+2]
}

//获取终端字符串
func Ptl698_45AddrStr(addr []byte) string {
	var sa = make([]string, 0)
	for _, v := range addr {
		sa = append(sa, fmt.Sprintf("%02X", v))
	}
	ss := strings.Join(sa, "")
	return ss
}

//主站MSA地址比较
func ptl698_45MsaCmp(msa int, buf []byte) bool {
	if msa == int(buf[6+buf[4]&0x0f]) {
		return true
	}
	return false
}

//从报文中取出主站MSA地址
func Ptl698_45MsaGet(buf []byte) int {
	return int(buf[6+buf[4]&0x0f])
}

//判断主站发出的msa是否有效
func Ptl698_45IsMsaValid(msa int) bool {
	if msa != 0 {
		return true
	}
	return false
}
