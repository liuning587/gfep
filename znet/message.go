package znet

import "sync"

// Message message
type Message struct {
	DataLen uint32 //消息的长度
	ID      uint32 //消息的ID
	Data    []byte //消息的内容
}

var msgPool = sync.Pool{
	New: func() any {
		return &Message{}
	},
}

// NewMsgPackage 创建一个Message消息包（从池中复用，由 DoMsgHandler 路径归还池）。
func NewMsgPackage(id uint32, data []byte) *Message {
	m := msgPool.Get().(*Message)
	m.ID = id
	m.DataLen = uint32(len(data))
	m.Data = data
	return m
}

func releaseMsg(m *Message) {
	if m == nil {
		return
	}
	m.Data = nil
	m.DataLen = 0
	m.ID = 0
	msgPool.Put(m)
}

// GetDataLen 获取消息数据段长度
func (msg *Message) GetDataLen() uint32 {
	return msg.DataLen
}

// GetMsgID 获取消息ID
func (msg *Message) GetMsgID() uint32 {
	return msg.ID
}

// GetData 获取消息内容
func (msg *Message) GetData() []byte {
	return msg.Data
}

// SetDataLen 设置消息数据段长度
func (msg *Message) SetDataLen(len uint32) {
	msg.DataLen = len
}

// SetMsgID 设计消息ID
func (msg *Message) SetMsgID(msgID uint32) {
	msg.ID = msgID
}

// SetData 设计消息内容
func (msg *Message) SetData(data []byte) {
	msg.Data = data
}
