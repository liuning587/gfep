package zptl

import (
	"encoding/binary"
	"fmt"
)

func ptl1376_1IsVaild(buf []byte) int32 {
	if len(buf) < 1 {
		return 0
	}

	//第一字节必须为0x68
	if buf[0] != 0x68 {
		return -1
	}

	//固定报文头长度不足
	if len(buf) < 6 {
		return 0
	}

	//第六个字节必须为0x68
	if buf[5] != 0x68 {
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
	userDataLength := ((uint16(buf[2]) << 8) + uint16(buf[1])) >> 2
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

func getFn(buf []byte) uint16 {
	var fn uint16 = uint16(buf[1]) * 8

	for i := 0; i < 8; i++ {
		if ((buf[0] >> i) & 0x01) == 0x01 {
			fn += uint16(i) + 1
		}
	}
	return fn
}

// Ptl1376_1GetDir 获取报文传输方向,0:主站-->终端, 1:终端-->主站
func Ptl1376_1GetDir(buf []byte) int {
	if buf[6]&0x80 != 0 {
		return 1
	}
	return 0
}

// Ptl1376_1GetFrameType 获取报文类型
func Ptl1376_1GetFrameType(buf []byte) int {
	if len(buf) < 20 {
		return OTHER
	}

	if buf[12] == 0x02 { //链路连接管理（登录，心跳，退出登录）
		switch binary.BigEndian.Uint32(buf[14:18]) {
		case 0x00000100:
			return LINK_LOGIN
		case 0x00000200:
			return LINK_EXIT
		case 0x00000400:
			return LINK_HAERTBEAT
		default:
			break
		}
	} else if buf[12] == 0xFE {
		if buf[1] == 0x42 && buf[2] == 0x00 &&
			buf[7] == 0x00 && buf[8] == 0x00 && buf[9] == 0x00 && buf[10] == 0x00 &&
			buf[14] == 0x00 && buf[15] == 0x00 && buf[16] == 0x00 && buf[17] == 0x00 {
			return ONLINE
		}
	}
	return OTHER
}

// Ptl1376_1BuildReplyPacket 打包登陆、心跳回复包
// 登录REQ: 68 32 00 32 00 68 C9 21 45 03 00 00 02 74 00 00 01 00 A9 16
// 登录ACK: 68 4A 00 4A 00 68 0B 21 45 03 00 02 00 64 00 00 04 00 02 00 00 01 00 00 E1 16
// 心跳REQ: 68 4A 00 4A 00 68 C9 34 08 94 04 00 02 75 00 00 04 00 33 30 14 12 D2 20 93 16
// 心跳ACK: 68 4A 00 4A 00 68 0B 34 08 94 04 02 00 65 00 00 04 00 02 00 00 04 00 00 50 16
func Ptl1376_1BuildReplyPacket(in []byte, out []byte) int {
	out[0] = 0x68
	out[1] = 0x48 | (in[1] & 0x03)
	out[2] = 0x00
	out[3] = out[1]
	out[4] = 0x00
	out[5] = 0x68
	out[6] = 0x0B //CTRL

	out[7] = in[7]
	out[8] = in[8]
	out[9] = in[9]
	out[10] = in[10]

	out[11] = 0x02                   //in[11] MSA
	out[12] = 0x00                   //AFN
	out[13] = 0x60 | (in[13] & 0x0f) //SEQ

	offset := 14
	out[offset+0] = 0x00
	out[offset+1] = 0x00
	out[offset+2] = 0x04
	out[offset+3] = 0x00
	out[offset+4] = 0x02 //确认afn
	out[offset+5] = in[14]
	out[offset+6] = in[15]
	out[offset+7] = in[16]
	out[offset+8] = in[17]
	out[offset+9] = 0x00
	out[offset+10] = GetCs(out[6 : offset+10])
	out[offset+11] = 0x16

	return offset + 12
}

// Ptl1376_1AddrCmp 终端地址比较
func Ptl1376_1AddrCmp(addr []byte, buf []byte) bool {
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

// Ptl1376_1AddrGet 从报文中取出终端地址
func Ptl1376_1AddrGet(buf []byte) []byte {
	return buf[7:11]
}

// Ptl1376_1AddrStr 获取终端字符串
func Ptl1376_1AddrStr(addr []byte) string {
	if len(addr) == 4 {
		return fmt.Sprintf("%02X%02X-%02X%02X", addr[1], addr[0], addr[3], addr[2])
	}
	return ""
}

// Ptl1376_1MsaCmp 主站MSA地址比较
func Ptl1376_1MsaCmp(msa int, buf []byte) bool {
	return msa == int(buf[11])
}

// Ptl1376_1MsaGet 从报文中取出主站MSA地址
func Ptl1376_1MsaGet(buf []byte) int {
	return int(buf[11])
}

// Ptl1376_1IsMsaValid 判断主站发出的msa是否有效
func Ptl1376_1IsMsaValid(msa int) bool {
	return msa != 0
}

// Ptl1376_1BuildPacket 创建登录包
// 登录: 68 32 00 32 00 68 C9 21 45 03 00 00 02 74 00 00 01 00 A9 16
// 心跳: 68 4A 00 4A 00 68 C9 34 08 94 04 00 02 75 00 00 04 00 33 30 14 12 D2 20 93 16
func Ptl1376_1BuildPacket(tp uint8, tsa []byte) []byte {
	return []byte{}
}
