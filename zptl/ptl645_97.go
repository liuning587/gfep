package zptl

func ptl645_97IsVaild(buf []byte) int32 {
	rlen := ptl645IsVaild(buf)
	if rlen > 0 {
		switch buf[8] & 0x1f {
		case 0x00: //00000: 保留
		case 0x01: //00001: 读数据
		case 0x02: //00010: 读后续数据
		case 0x03: //00011: 重读数据
		case 0x04: //00100: 写数据
		case 0x08: //01000: 广播校时
		case 0x0a: //01010: 写设备地址
		case 0x0c: //01100: 更改通信速率
		case 0x0f: //01111: 修改密码
		case 0x10: //10000: 最大需量清零
			return rlen

		default:
			return -1
		}
	}
	return rlen
}
