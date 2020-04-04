package ziface

/*
	报文检测
*/
type IProtocol interface {
	//报文检测, 返回合法报文数量
	Chkfrm(data []byte) int32

	//复位
	Reset()
}
