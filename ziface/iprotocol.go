package ziface

// IProtocol 报文检测
type IProtocol interface {
	//Chkfrm 报文检测, 返回合法报文数量
	Chkfrm(data []byte) int32

	//Reset 复位
	Reset()
}
