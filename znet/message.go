package znet

// Message message
type Message struct {
	DataLen uint32 //消息的长度
	Id      uint32 //消息的ID
	Data    []byte //消息的内容
}

// NewMsgPackage 创建一个Message消息包
func NewMsgPackage(id uint32, data []byte) *Message {
	return &Message{
		DataLen: uint32(len(data)),
		Id:      id,
		Data:    data,
	}
}

// GetDataLen 获取消息数据段长度
func (msg *Message) GetDataLen() uint32 {
	return msg.DataLen
}

// GetMsgId 获取消息ID
func (msg *Message) GetMsgId() uint32 {
	return msg.Id
}

// GetData 获取消息内容
func (msg *Message) GetData() []byte {
	return msg.Data
}

// SetDataLen 设置消息数据段长度
func (msg *Message) SetDataLen(len uint32) {
	msg.DataLen = len
}

// SetMsgId 设计消息ID
func (msg *Message) SetMsgId(msgID uint32) {
	msg.Id = msgID
}

// SetData 设计消息内容
func (msg *Message) SetData(data []byte) {
	msg.Data = data
}
