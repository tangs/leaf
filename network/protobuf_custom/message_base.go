package protobuf_custom

type InnerMessage struct {
	cmd string
	bytes []byte
}

//const SessionIdInnerProto uint32 = 0xfffffffe
const SessionIdInnerMessage uint32 = 0xffffffff


type MessageBase struct {
	MessageId int32
	SerialId int32
	SessionId uint32
	Message interface{}
	innerMessage *InnerMessage
}

//func (msg *MessageBase) IsInnerProto() bool {
//	return msg.SessionId == SessionIdInnerProto
//}

func (msg *MessageBase) IsInnerMessage() bool {
	return msg.SessionId == SessionIdInnerMessage
}
