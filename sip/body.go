package sip

import (
	"sipclientgo/global"
)

type MessageBody struct {
	PartsContents map[global.BodyType]ContentPart //used to store incoming/outgoing body parts
	MessageBytes  []byte                          //used to store the generated body bytes for sending msgs
}

type ContentPart struct {
	Headers SipHeaders
	Bytes   []byte
}

func EmptyBody() MessageBody {
	var mb MessageBody
	return mb
}

func NewMessageBody(init bool) *MessageBody {
	if init {
		return &MessageBody{PartsContents: make(map[global.BodyType]ContentPart)}
	}
	return new(MessageBody)
}

func NewMessageSDPBody(sdpbytes []byte) MessageBody {
	// mb := MessageBody{PartsContents: make(map[BodyType]ContentPart)}
	// mb.PartsContents[SDP] = ContentPart{Bytes: sdpbytes}
	// return mb
	return MessageBody{PartsContents: map[global.BodyType]ContentPart{global.SDP: {Bytes: []byte(sdpbytes)}}}
}

func NewContentPart(bt global.BodyType, bytes []byte) ContentPart {
	var ct ContentPart
	ct.Bytes = bytes
	ct.Headers = NewSipHeaders()
	ct.Headers.AddHeader(global.Content_Type, global.DicBodyContentType[bt])
	return ct
}

// ===============================================================

func NewMSCXML(xml []byte) MessageBody {
	// if aspart {
	// 	hdrs := NewSipHeaders()
	// 	hdrs.AddHeader(Content_Length, DicBodyContentType[MSCXML])
	// 	return MessageBody{PartsContents: map[BodyType]ContentPart{MSCXML: {hdrs, []byte(xml)}}}
	// }
	return MessageBody{PartsContents: map[global.BodyType]ContentPart{global.MSCXML: {Bytes: xml}}}
}

// ===============================================================

func (messagebody *MessageBody) WithNoBody() bool {
	return messagebody.PartsContents == nil
}

func (messagebody *MessageBody) WithUnknownBodyPart() bool {
	if messagebody.WithNoBody() {
		return false
	}
	if len(messagebody.PartsContents) == 0 { // means PartsContents initialized but nothing added
		return true
	}
	for k := range messagebody.PartsContents {
		if k == global.Unknown {
			return true
		}
	}
	return false
}

func (messagebody *MessageBody) IsMultiPartBody() bool {
	if messagebody.WithNoBody() {
		return false
	}
	return len(messagebody.PartsContents) >= 2
}

func (messagebody *MessageBody) ContainsSDP() bool {
	if messagebody.WithNoBody() {
		return false
	}
	_, ok := messagebody.PartsContents[global.SDP]
	return ok
}

func (messagebody *MessageBody) IsJSON() bool {
	if messagebody.WithNoBody() {
		return false
	}
	_, ok := messagebody.PartsContents[global.AppJson]
	return ok
}

func (messagebody *MessageBody) ContentLength() int {
	return len(messagebody.MessageBytes)
}
