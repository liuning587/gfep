package zptl

func ptl1376_1IsVaild(buf []byte) int32 {
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
