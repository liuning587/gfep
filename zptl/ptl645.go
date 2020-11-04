package zptl

func ptl645IsVaild(buf []byte) int32 {
	if len(buf) < 1 {
		return 0
	}

	//第一字节必须为0x68
	if 0x68 != buf[0] {
		return -1
	}

	//长度不足
	if len(buf) < 8 {
		return 0
	}

	//第八个字节必须为0x68
	if 0x68 != buf[7] {
		return -1
	}

	//长度不足
	if len(buf) < 10 {
		return 0
	}

	//长度不足
	if len(buf) < int(buf[9]+11) {
		return 0
	}

	//校验和
	if buf[buf[9]+10] != GetCs(buf[:buf[9]+10]) {
		return -1
	}

	if len(buf) < int(buf[9]+12) {
		return 0
	}

	//判断最后字节0x16
	if buf[buf[9]+11] != 0x16 {
		return -1
	}

	return int32(buf[9] + 12)
}
