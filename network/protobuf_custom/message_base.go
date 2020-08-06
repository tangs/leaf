package protobuf_custom

type InnerMessage struct {
	Cmd string
	Bytes []byte
}

func (x *InnerMessage) Reset() {
	x.Cmd = ""
	x.Bytes = nil
}

func (x *InnerMessage) String() string {
	return "InnerMessage"
}

func (*InnerMessage) ProtoMessage() {}

const SessionIdInnerMessage uint32 = 0xffffffff
const MessageIdInnerMessage int32 = 0x7fffffff


type MessageBase struct {
	MessageId int32
	SerialId int32
	GatewayId uint32
	SessionId uint32
	Message interface{}
}

//func (msg *MessageBase) IsInnerProto() bool {
//	return msg.SessionId == SessionIdInnerProto
//}

func (msg *MessageBase) IsInnerMessage() bool {
	return msg.SessionId == SessionIdInnerMessage
}
