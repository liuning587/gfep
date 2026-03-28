package zptl

// 进入业务 Handle 前最小长度（保守值，避免 Ptl*Get* 在未完整帧上越界）。
// 与 ptl1376_1 / ptl698_45 / ptl_nw 中字段访问范围对齐，实际完整帧仍由 Chkfrm/IsValid 保证。
const (
	MinHandlerParseLen1376 = 20
	MinHandlerParseLen698  = 32
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
