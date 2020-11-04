package zptl

func ptl645_07IsVaild(buf []byte) int32 {
	rlen := ptl645IsVaild(buf)
	if rlen > 0 {
		switch buf[8] & 0x1f {
		case 0x00: //00000: 保留
		case 0x03: //     : 安全情况
		case 0x08: //01000: 广播校时
		case 0x11: //10001: 读数据
		case 0x12: //10010: 读后续数据
		case 0x13: //10011: 读通信地址
		case 0x14: //10100: 写数据
		case 0x15: //10101: 写通信地址
		case 0x16: //10110: 冻结命令
		case 0x17: //10111: 更改通信速率
		case 0x18: //11000: 修改密码
		case 0x19: //11001: 最大需量清零
		case 0x1a: //11010: 电量清零
		case 0x1b: //11011: 事件清零
		case 0x1c: //11100: 跳闸、合闸允许、直接合闸、报警、报警解除、保电和保电解除
			return rlen

		default:
			return -1
		}
	}
	return rlen
}
