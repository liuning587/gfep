package zptl

func ptl1376_2IsVaild(buf []byte) int32 {
	if len(buf) < 1 {
		return 0
	}

	//第一字节必须为0x68
	if 0x68 != buf[0] {
		return -1
	}

	//长度不足
	if len(buf) < 3 {
		return 0
	}

	//判断长度域
	flen := (uint16(buf[2]) << 8) + uint16(buf[1])
	if (flen > PmaxPtlFrameLen) || (flen < 7) {
		return -1
	}

	//长度不足
	if len(buf) < int(flen-1) {
		return 0
	}

	//校验和
	if buf[flen-2] != GetCs(buf[3:flen-2]) {
		return -1
	}

	//长度不足
	if len(buf) < int(flen) {
		return 0
	}

	//判断最后字节0x16
	if buf[flen-1] != 0x16 {
		return -1
	}

	return int32(flen)
}
