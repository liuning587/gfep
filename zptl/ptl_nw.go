package zptl

import (
	"encoding/binary"
	"fmt"
	"strings"
)

func ptlNwIsVaild(buf []byte) int32 {
	if len(buf) < 1 {
		return 0
	}

	//第一字节必须为0x68
	if 0x68 != buf[0] {
		return -1
	}

	//固定报文头长度不足
	if len(buf) < 6 {
		return 0
	}

	//第六个字节必须为0x68
	if 0x68 != buf[5] {
		return -1
	}

	//一帧报文必备项长度不足
	if len(buf) < 8 {
		return 0
	}

	//前后两个长度L不一致
	if (buf[1] != buf[3]) || (buf[2] != buf[4]) {
		return -1
	}

	//计算用户数据长度
	userDataLength := (uint16(buf[2]) << 8) + uint16(buf[1])
	if (userDataLength > (PmaxPtlFrameLen - 8)) || (userDataLength < 8) {
		return -1
	}

	//一帧报文必备项长度不足
	if len(buf) < int(userDataLength+7) {
		return 0
	}

	//校验和
	if buf[userDataLength+6] != GetCs(buf[6:userDataLength+6]) {
		return -1
	}

	//一帧报文必备项长度不足
	if len(buf) < int(userDataLength+8) {
		return 0
	}

	//判断最后字节0x16
	if buf[userDataLength+7] != 0x16 {
		return -1
	}

	return int32(userDataLength + 8)
}

//--------------------------------------------------------------------

// PtlNwGetDir 获取报文传输方向,0:主站-->终端, 1:终端-->主站
func PtlNwGetDir(buf []byte) int {
	if buf[6]&0x80 != 0 {
		return 1
	}
	return 0
}

// PtlNwGetFrameType 获取报文类型
func PtlNwGetFrameType(buf []byte) int {
	if len(buf) < 23 {
		return OTHER
	}

	if buf[14] == 0x02 { //链路连接管理（登录，心跳，退出登录）
		switch binary.LittleEndian.Uint32(buf[18:22]) {
		case 0xE0001000:
			return LINK_LOGIN
		case 0xE0001002:
			return LINK_EXIT
		case 0xE0001001:
			return LINK_HAERTBEAT
		default:
			break
		}
	}
	return OTHER
}

// PtlNwBuildReplyPacket 打包登陆、心跳回复包
// 68 12 00 12 00 68 C9 23 00 00 01 00 00 00 02 71 00 00 00 10 00 E0 00 01 51 16
// 68 11 00 11 00 68 0B 23 00 00 01 00 00 00 00 61 00 00 00 00 00 E0 00 70 16
func PtlNwBuildReplyPacket(in []byte, out []byte) int {
	out[0] = 0x68
	out[1] = 0x11
	out[2] = 0x00
	out[3] = 0x11
	out[4] = 0x00
	out[5] = 0x68
	out[6] = 0x0B //CTRL

	out[7] = in[7]
	out[8] = in[8]
	out[9] = in[9]
	out[10] = in[10]
	out[11] = in[11]
	out[12] = in[12]

	out[13] = 0x00                   //in[13] MSA
	out[14] = 0x00                   //AFN
	out[15] = 0x60 | (in[15] & 0x0f) //SEQ

	offset := 16
	out[offset+0] = 0x00
	out[offset+1] = 0x00
	out[offset+2] = 0x00
	out[offset+3] = 0x00
	out[offset+4] = 0x00
	out[offset+5] = 0xE0
	out[offset+6] = 0x00
	out[offset+7] = GetCs(out[6 : offset+7])
	out[offset+8] = 0x16

	return offset + 9
}

// PtlNwAddrCmp 终端地址比较
func PtlNwAddrCmp(addr []byte, buf []byte) bool {
	if len(addr) != len(buf) {
		return false
	}

	for i := 0; i <= len(addr); i++ {
		if addr[i] != buf[i] {
			return false
		}
	}

	return true
}

// PtlNwAddrGet 从报文中取出终端地址
func PtlNwAddrGet(buf []byte) []byte {
	return buf[7:13]
}

// PtlNwAddrStr 获取终端字符串
func PtlNwAddrStr(addr []byte) string {
	if len(addr) == 6 {
		return fmt.Sprintf("%02X%02X%02X-%02X%02X%02X", addr[2], addr[1], addr[0], addr[5], addr[4], addr[3])
	}
	var sa = make([]string, 0)
	for _, v := range addr {
		sa = append(sa, fmt.Sprintf("%02X", v))
	}
	ss := strings.Join(sa, "")
	return ss
}

// PtlNwMsaCmp 主站MSA地址比较
func PtlNwMsaCmp(msa int, buf []byte) bool {
	if msa == int(buf[13]) {
		return true
	}
	return false
}

// PtlNwMsaGet 从报文中取出主站MSA地址
func PtlNwMsaGet(buf []byte) int {
	return int(buf[13])
}

// PtlNwIsMsaValid 判断主站发出的msa是否有效
func PtlNwIsMsaValid(msa int) bool {
	if msa != 0 {
		return true
	}
	return false
}
