package protobuf_custom

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/name5566/leaf/chanrpc"
	"github.com/name5566/leaf/log"
	"math"
	"reflect"
)

// -------------------------
// | id | protobuf message |
// -------------------------
type Processor struct {
	littleEndian bool
	msgInfo      map[uint32]*MsgInfo
	msgID        map[reflect.Type]uint32
}

type MsgInfo struct {
	msgType       reflect.Type
	msgRouter     *chanrpc.Server
	msgHandler    MsgHandler
	msgRawHandler MsgHandler
}

type MsgHandler func([]interface{})

type MsgRaw struct {
	msgID      uint16
	msgRawData []byte
}

func NewProcessor() *Processor {
	p := new(Processor)
	p.littleEndian = false
	p.msgID = make(map[reflect.Type]uint32)
	p.msgInfo = make(map[uint32]*MsgInfo)
	return p
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) Register(msg proto.Message, id uint32) uint32 {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Fatal("protobuf message pointer required")
	}
	if _, ok := p.msgID[msgType]; ok {
		log.Fatal("message %s is already registered", msgType)
	}
	if len(p.msgInfo) >= math.MaxUint16 {
		log.Fatal("too many protobuf messages (max = %v)", math.MaxUint16)
	}

	i := new(MsgInfo)
	i.msgType = msgType
	//p.msgInfo = append(p.msgInfo, i)
	//id := uint16(len(p.msgInfo) - 1)
	p.msgInfo[id] = i
	p.msgID[msgType] = id
	fmt.Println("register:", msgType, id)
	return id
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetRouter(msg proto.Message, msgRouter *chanrpc.Server) {
	msgType := reflect.TypeOf(msg)
	id, ok := p.msgID[msgType]
	if !ok {
		log.Fatal("message %s not registered5", msgType)
	}

	p.msgInfo[id].msgRouter = msgRouter
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetHandler(msg proto.Message, msgHandler MsgHandler) {
	msgType := reflect.TypeOf(msg)
	id, ok := p.msgID[msgType]
	if !ok {
		log.Fatal("message %s not registered4", msgType)
	}

	p.msgInfo[id].msgHandler = msgHandler
}

// It's dangerous to call the method on routing or marshaling (unmarshaling)
func (p *Processor) SetRawHandler(id uint16, msgRawHandler MsgHandler) {
	if id >= uint16(len(p.msgInfo)) {
		log.Fatal("message id %v not registered3", id)
	}

	p.msgInfo[uint32(id)].msgRawHandler = msgRawHandler
}

// goroutine safe
func (p *Processor) Route(msg interface{}, userData interface{}) error {
	// raw
	if msgRaw, ok := msg.(MsgRaw); ok {
		if msgRaw.msgID >= uint16(len(p.msgInfo)) {
			return fmt.Errorf("message id %v not registered2", msgRaw.msgID)
		}
		i := p.msgInfo[uint32(msgRaw.msgID)]
		if i.msgRawHandler != nil {
			i.msgRawHandler([]interface{}{msgRaw.msgID, msgRaw.msgRawData, userData})
		}
		return nil
	}

	// protobuf
	msg1 := msg.(*MessageBase)
	fmt.Println("msg id:", msg1.MessageId, msg1)
	msgType := reflect.TypeOf(msg1.Message)
	id, ok := p.msgID[msgType]
	if !ok {
		return fmt.Errorf("message %s not registered1", msgType)
	}
	i := p.msgInfo[id]
	if i.msgHandler != nil {
		i.msgHandler([]interface{}{msg, userData})
	}
	if i.msgRouter != nil {
		i.msgRouter.Go(msgType, msg, userData)
	}
	return nil
}

// goroutine safe
func (p *Processor) Unmarshal(data []byte) (interface{}, error) {
	if len(data) < 4 {
		return nil, errors.New("message data too short")
	}

	fmt.Println("data:", data)
	messageBase := &MessageBase{}
	idx := 0
	//if !messageBase.IsInnerProto() {
	//	messageBase.SessionId = binary.LittleEndian.Uint32(data)
	//	idx += 4
	//}
	messageBase.MessageId = -1
	log.Debug(fmt.Sprintf("session id: %d", messageBase.SessionId))

	if messageBase.IsInnerMessage() {
		strLen, len1 := binary.Uvarint(data[idx:])
		if len1 <= 0 {
			return nil, errors.New("read string len fail")
		}
		idx += len1
		endIdx := idx + int(strLen)
		messageBase.innerMessage.cmd = string(data[idx:endIdx])
		messageBase.innerMessage.bytes = make([]byte, endIdx - idx)
		messageBase.MessageId = -1
		copy(messageBase.innerMessage.bytes, data[endIdx:])
		log.Debug("unmarshal inner message.cmd:%s", messageBase.innerMessage.cmd)
		return messageBase, nil
	} else {
		serialId, len1 := binary.Varint(data[idx:])
		if len1 <= 0 {
			return nil, errors.New(fmt.Sprintf("read serialId fail, id:%d", serialId))
		}
		idx += len1

		msgId, len1 := binary.Varint(data[idx:])
		if len1 <= 0 {
			return nil, errors.New(fmt.Sprintf("read msgId fail, id:%d", msgId))
		}
		idx += len1

		messageBase.SerialId = int32(serialId)
		messageBase.MessageId = int32(msgId)
		i := p.msgInfo[uint32(msgId)]
		fmt.Println("i:", i)

		log.Debug(fmt.Sprintf("serial id: %d, meessage id:%d", serialId, msgId))

		if i.msgRawHandler != nil {
			return MsgRaw{uint16(msgId), data[idx:]}, nil
		} else {
			msg := reflect.New(i.msgType.Elem()).Interface()
			err := proto.UnmarshalMerge(data[idx:], msg.(proto.Message))
			//msg1 := msg.(proto.Message)
			msgType := reflect.TypeOf(msg)
			fmt.Println("type1", msgType)
			messageBase.Message = msg
			return messageBase, err
		}
	}
	//// id
	//var id uint16
	//if p.littleEndian {
	//	id = binary.LittleEndian.Uint16(data)
	//} else {
	//	id = binary.BigEndian.Uint16(data)
	//}
	//if id >= uint16(len(p.msgInfo)) {
	//	return nil, fmt.Errorf("message id %v not registered", id)
	//}
	//
	//// msg
	//i := p.msgInfo[id]
	//if i.msgRawHandler != nil {
	//	return MsgRaw{id, data[2:]}, nil
	//} else {
	//	msg := reflect.New(i.msgType.Elem()).Interface()
	//	return msg, proto.UnmarshalMerge(data[2:], msg.(proto.Message))
	//}
}

// goroutine safe
func (p *Processor) Marshal(msg interface{}) ([][]byte, error) {

	if msgBase, ok := msg.(*MessageBase); ok {
		idx := 0
		if msgBase.IsInnerMessage() {
			header := make([]byte, 256)
			idx += binary.PutUvarint(header[idx:], uint64(len(msgBase.innerMessage.cmd)))
			idx += copy(header[idx:], msgBase.innerMessage.cmd)

			return [][]byte{header, msgBase.innerMessage.bytes}, nil
		} else {
			header := make([]byte, 64)
			if !msgBase.IsInnerProto() {
				binary.LittleEndian.PutUint32(header, msgBase.SessionId)
				idx += 4
			}
			idx += binary.PutVarint(header[idx:], int64(msgBase.SerialId))
			idx += binary.PutVarint(header[idx:], int64(msgBase.MessageId))

			data, err := proto.Marshal(msgBase.Message.(proto.Message))
			return [][]byte{header[:idx], data}, err
		}
	}

	return nil, errors.New("Is not a *MessageBase instance")
}

// goroutine safe
func (p *Processor) Range(f func(id uint16, t reflect.Type)) {
	for id, i := range p.msgInfo {
		f(uint16(id), i.msgType)
	}
}
