package zptl

// 88 01 00 25 68 1D 00 1D 00 68 4B 23 00 00 11 11 21 49 0D 74 02 01 00 FF 01
// 00 20 19 03 07 00 00 20 19 03 07 14 00 00 18 16 77

func ptlNwmIsVaild(buf []byte) int32 {
	if len(buf) < 1 {
		return 0
	}

	//第一字节必须为0x88
	if 0x88 != buf[0] {
		return -1
	}

	//固定报文头长度不足
	if len(buf) < 2 {
		return 0
	}

	//第二个字节必须为0x01
	if 0x01 != buf[1] {
		return -1
	}

	//一帧报文必备项长度不足
	if len(buf) < 4 {
		return 0
	}

	//计算用户数据长度
	userDataLength := (uint16(buf[2]) << 8) + uint16(buf[3])
	if (userDataLength > 2048) || (userDataLength < 7) {
		return -1
	}

	//报文长度不足
	if len(buf) < int(userDataLength+4+1) {
		return 0
	}

	//判断最后字节0x77
	if buf[userDataLength+4] != 0x77 {
		return -1
	}

	return int32(userDataLength + 5)
}
