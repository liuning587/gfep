package zptl

// 进入业务 Handle 前最小长度（避免 Ptl*Get* 在未完整帧上越界）。
// 698：Ptl698_45AddrGet / Ptl698_45MsaGet 在地址域低 4 位为 0x0f 时最大读到 buf[21]，故取 22。
// Ptl698_45GetFrameType（链路管理分支）与 Ptl698_45IsReport 在包内自带下标长度校验，短帧不会越界。
// 若把此处提到 27 会误杀合法短下行（如总长 25）；完整帧仍由 Chkfrm/IsValid 保证。
const (
	MinHandlerParseLen1376 = 20
	MinHandlerParseLen698  = 22
	MinHandlerParseLenNw   = 23
)

// HandlerParseLenOK 若长度不足则业务层应丢弃本帧并勿调用解析函数。
func HandlerParseLenOK(ptype uint32, data []byte) bool {
	n := len(data)
	switch {
	case ptype&PTL_1376_1 != 0:
		return n >= MinHandlerParseLen1376
	case ptype&PTL_698_45 != 0:
		return n >= MinHandlerParseLen698
	case ptype&PTL_NW != 0:
		return n >= MinHandlerParseLenNw
	default:
		return n >= MinHandlerParseLen698
	}
}
