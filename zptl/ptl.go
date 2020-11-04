package zptl

const (
	PTL_1376_1      = 0x0001 /**< 1376.1协议 */
	PTL_1376_2      = 0x0002 /**< 1376.2协议 */
	PTL_645_97      = 0x0004 /**< 645-97协议 */
	PTL_645_07      = 0x0008 /**< 645-07协议 */
	PTL_698_45      = 0x0010 /**< 698.45协议 */
	PTL_NW          = 0x0020 /**< 南网2013协议 */
	PTL_NWM         = 0x0040 /**< 南网2013协议(密) */
	PTL_SSAL        = 0x0080 /**< 国网安全应用层协议 */
	PTL_ALL         = 0x00ff /**< 以上任意协议 */
	PTL_RAW         = 0x0100 /**< 原始报文 */
	PTL_698_45_HEAD = 0x0200 /**< 698头HCS结尾 */
	PTL_64507_1     = 0x1008 /**< 单相64507*/
	PTL_UNKNOW      = 0xffff /**< 未知 */
)

const (
	LINK_LOGIN     = 0
	LINK_EXIT      = 1
	LINK_HAERTBEAT = 2
	OTHER          = 3
	FUNCERROR      = 4
	ONLINE         = 5
)

//常量
const (
	PmaxPtlFrameLen = 2200 //最大报文长度
)

type ptlChkTab struct {
	ptype   uint32
	isVaild func([]byte) int32
}

var thePtlChkTab = [...]ptlChkTab{
	{PTL_698_45, ptl698_45IsVaild},
	{PTL_NW, ptlNwIsVaild},
	{PTL_NWM, ptlNwmIsVaild},
	{PTL_645_07, ptl645_07IsVaild},
	{PTL_645_97, ptl645_97IsVaild},
	{PTL_1376_1, ptl1376_1IsVaild},
	{PTL_1376_2, ptl1376_2IsVaild},
	//todo: other plt
}

// GetType 获取报文类型
func GetType(data []byte) uint32 {
	for i := 0; i < len(thePtlChkTab); i++ {
		if 0 < thePtlChkTab[i].isVaild(data) {
			return thePtlChkTab[i].ptype
		}
	}

	return PTL_UNKNOW
}

// GetLen 获取报文长度
func GetLen(data []byte) int32 {
	for i := 0; i < len(thePtlChkTab); i++ {
		rlen := thePtlChkTab[i].isVaild(data)
		if 0 < rlen {
			return rlen
		}
	}
	return 0
}

// IsVaild 判断报文释放合法
func IsVaild(ptype uint32, data []byte) (int32, uint32) {
	for i := 0; i < len(thePtlChkTab); i++ {
		if ptype&thePtlChkTab[i].ptype != 0 {
			rlen := thePtlChkTab[i].isVaild(data)
			if 0 <= rlen {
				return rlen, thePtlChkTab[i].ptype
			}
		}
	}

	return -1, PTL_UNKNOW
}

//判断是否为协议首字节
func isFirstByte(ptype uint32, head byte) bool {
	if head == 0x68 {
		return true
	}

	if ptype == PTL_NWM && head == 0x88 {
		return true
	}

	if ptype == PTL_SSAL && head == 0x98 {
		return true
	}

	return false
}

//findFirstByte 获取指定协议报文首字节偏移
func findFirstByte(ptype uint32, data []byte) int32 {
	var i int32
	var dlen int32 = int32(len(data))

	if ptype&(PTL_NWM|PTL_SSAL) != 0 {
		for i = 0; i < dlen; i++ {
			if data[i] == 0x68 || data[i] == 0x88 || data[i] == 0x98 {
				break
			}
		}
	} else if ptype&PTL_NWM != 0 {
		for i = 0; i < dlen; i++ {
			if data[i] == 0x68 || data[i] == 0x88 {
				break
			}
		}
	} else if ptype&PTL_SSAL != 0 {
		for i = 0; i < dlen; i++ {
			if data[i] == 0x68 || data[i] == 0x98 {
				break
			}
		}
	} else {
		for i = 0; i < dlen; i++ {
			if data[i] == 0x68 {
				break
			}
		}
	}

	if i < dlen {
		return i
	}
	return -1
}

//Check 从输入缓存中找出首条合法报文
func Check(ptype uint32, data []byte) (int32, int32, uint32) {
	var pos int32 = 0
	inlen := int32(len(data))

	for inlen > 0 {
		offset := findFirstByte(ptype, data[pos:])
		if offset < 0 {
			return -1, 0, PTL_UNKNOW //头部(68,88,98)都找不到, 直接退出
		}
		pos += offset
		inlen -= offset
		if inlen == 1 {
			return -1, 0, PTL_UNKNOW //最后1个字节是(68,88,98)
		}

		rlen, ptype := IsVaild(ptype, data[pos:])
		if rlen >= 0 {
			return pos, rlen, ptype
		}
		pos++
		inlen--
	}

	return -1, 0, PTL_UNKNOW
}
