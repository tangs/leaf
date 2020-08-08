package protobuf_custom

import (
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/tangs/leaf/chanrpc"
	"github.com/tangs/leaf/log"
	"math"
	"reflect"
)

// -------------------------
// (| session id )| serial id | message id | protobuf message |
// isInnerProto 为 true时，有session id(fixed uint32)
// serial id:var int
// message id:var int
// -------------------------
type Processor struct {
	littleEndian bool
	msgInfo      map[uint32]*MsgInfo
	msgID        map[reflect.Type]uint32
	isInnerProto bool
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

func NewProcessor(isInnerProto bool) *Processor {
	p := new(Processor)
	p.littleEndian = false
	p.msgID = make(map[reflect.Type]uint32)
	p.msgInfo = make(map[uint32]*MsgInfo)
	p.isInnerProto = isInnerProto
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
	p.msgInfo[id] = i
	p.msgID[msgType] = id
	log.Debug("register:%v, %v", msgType, id)
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
	msgType := reflect.TypeOf(msg1.Message)
	//log.Debug("msg id:%v, %v, %v", msg1.MessageId, msg1, msgType)
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

	//log.Debug("Unmarshal:%v", data)
	messageBase := &MessageBase{}
	idx := 0
	if !p.isInnerProto {
		messageBase.SessionId = binary.LittleEndian.Uint32(data)
		idx += 4
		if !messageBase.IsInnerMessage() {
			log.Debug(fmt.Sprintf("session id: %d", messageBase.SessionId))
		}
	}

	if messageBase.IsInnerMessage() {
		strLen, len1 := binary.Uvarint(data[idx:])
		if len1 <= 0 {
			return nil, errors.New("read string len fail")
		}
		idx += len1
		endIdx := idx + int(strLen)
		//log.Debug("read cmd, idx: %d, len: %d, %v", idx, strLen, data[idx:endIdx])
		msg := &InnerMessage{}
		msg.Cmd = string(data[idx:endIdx])
		msg.Bytes = make([]byte, endIdx - idx)
		copy(msg.Bytes, data[endIdx:])
		messageBase.Message = msg
		messageBase.MessageId = MessageIdInnerMessage
		//log.Debug("unmarshal inner message.cmd:%s", msg.Cmd)
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

		log.Debug(fmt.Sprintf("serial id: %d, meessage id:%d",  serialId, msgId))

		if i == nil {
			return nil, errors.New(fmt.Sprintf("cant't fiad message info by msg id:%v", msgId))
		}

		if i.msgRawHandler != nil {
			return MsgRaw{uint16(msgId), data[idx:]}, nil
		} else {
			msg := reflect.New(i.msgType.Elem()).Interface()
			err := proto.UnmarshalMerge(data[idx:], msg.(proto.Message))
			messageBase.Message = msg
			return messageBase, err
		}
	}
}

// goroutine safe
func (p *Processor) Marshal(msg interface{}) ([][]byte, error) {
	if msgBase, ok := msg.(*MessageBase); ok {
		//if !msgBase.IsInnerMessage() {
		//	log.Debug("Marshal data:%v", msg)
		//}
		idx := 0
		header := make([]byte, 256)
		if !p.isInnerProto {
			binary.LittleEndian.PutUint32(header, msgBase.SessionId)
			idx += 4
		}
		if msgBase.IsInnerMessage() {
			msg := msgBase.Message.(*InnerMessage)
			idx += binary.PutUvarint(header[idx:], uint64(len(msg.Cmd)))
			idx += copy(header[idx:], msg.Cmd)

			//log.Debug("Marshal inner msg:%v, %v, %v", msg.Cmd, header[:idx], msg.Bytes)
			return [][]byte{header[:idx], msg.Bytes}, nil
		} else {
			idx += binary.PutVarint(header[idx:], int64(msgBase.SerialId))
			idx += binary.PutVarint(header[idx:], int64(msgBase.MessageId))

			data, err := proto.Marshal(msgBase.Message.(proto.Message))
			//log.Debug("Marshal:%v, %v", header[:idx], data)
			return [][]byte{header[:idx], data}, err
		}
	}

	return nil, errors.New("must *MessageBase object")
}

// goroutine safe
func (p *Processor) Range(f func(id uint16, t reflect.Type)) {
	for id, i := range p.msgInfo {
		f(uint16(id), i.msgType)
	}
}
