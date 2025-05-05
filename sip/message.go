package sip

import (
	"bytes"
	"cmp"
	"encoding/hex"
	"fmt"
	"net"
	"sipclientgo/global"
	"sipclientgo/system"
	"slices"
	"strings"
)

type SipMessage struct {
	MsgType   global.MessageType
	StartLine *SipStartLine
	Headers   *SipHeaders
	Body      *MessageBody

	//all fields below are only set in incoming messages
	FromHeader string
	ToHeader   string
	PAIHeaders []string
	DivHeaders []string

	CallID    string
	FromTag   string
	ToTag     string
	ViaBranch string

	ViaUdpAddr *net.UDPAddr

	RCURI string
	RRURI string

	MaxFwds       int
	CSeqNum       uint32
	CSeqMethod    global.Method
	ContentLength uint16 //only set for incoming messages
}

func NewRequestMessage(md global.Method, up string) *SipMessage {
	sipmsg := &SipMessage{
		MsgType: global.REQUEST,
		StartLine: &SipStartLine{
			Method:    md,
			UriScheme: "sip",
			UserPart:  up,
		},
	}
	return sipmsg
}

func NewResponseMessage(sc int, rp string) *SipMessage {
	sipmsg := &SipMessage{MsgType: global.RESPONSE, StartLine: new(SipStartLine)}
	if 100 <= sc && sc <= 699 {
		sipmsg.StartLine.StatusCode = sc
		dfltsc := system.Str2Int[int](fmt.Sprintf("%d00", sc/100))
		sipmsg.StartLine.ReasonPhrase = cmp.Or(rp, global.DicResponse[sc], global.DicResponse[dfltsc])
	}
	return sipmsg
}

// ==========================================================================

func (sipmsg *SipMessage) getAddBodyParts() []int {
	hv := system.ASCIIToLower(sipmsg.Headers.ValueHeader(global.P_Add_BodyPart))
	hv = strings.ReplaceAll(hv, " ", "")
	partflags := strings.Split(hv, ",")
	var flags []int
	for k, v := range global.BodyAddParts {
		if slices.Contains(partflags, v) {
			flags = append(flags, k)
		}
	}
	return flags
}

func (sipmsg *SipMessage) AddRequestedBodyParts() {
	pflags := sipmsg.getAddBodyParts()
	if len(pflags) == 0 {
		return
	}
	// if sipmsg.Body
	msgbdy := sipmsg.Body
	hdrs := sipmsg.Headers
	if len(msgbdy.PartsContents) == 1 {
		frstbt := system.FirstKey(msgbdy.PartsContents)
		cntnthdrsmap := hdrs.ValuesWithHeaderPrefix("Content-", global.Content_Length.LowerCaseString())
		hdrs.DeleteHeadersWithPrefix("Content-")
		msgbdy.PartsContents[frstbt] = ContentPart{Headers: NewSHsFromMap(cntnthdrsmap), Bytes: msgbdy.PartsContents[frstbt].Bytes}
	}
	for _, pf := range pflags {
		switch pf {
		case global.AddXMLPIDFLO:
			bt := global.PIDFXML
			xml := `<?xml version="1.0" encoding="UTF-8"?><presence xmlns="urn:ietf:params:xml:ns:pidf" xmlns:gp="urn:ietf:params:xml:ns:pidf:geopriv10" xmlns:cl="urn:ietf:params:xml:ns:pidf:geopriv10:civicLoc" xmlns:btd="http://btd.orange-business.com" entity="pres:geotarget@btip.orange-business.com"><tuple id="sg89ae"><status><gp:geopriv><gp:location-info><cl:civicAddress><cl:country>FR</cl:country><cl:A2>35</cl:A2><cl:A3>CESSON SEVIGNE</cl:A3><cl:A6>DU CHENE GERMAIN</cl:A6><cl:HNO>9</cl:HNO><cl:STS>RUE</cl:STS><cl:PC>35510</cl:PC><cl:CITYCODE>99996</cl:CITYCODE></cl:civicAddress></gp:location-info><gp:usage-rules></gp:usage-rules></gp:geopriv></status></tuple></presence>`
			xmlbytes := []byte(xml)
			msgbdy.PartsContents[bt] = NewContentPart(bt, xmlbytes)
		case global.AddINDATA:
			bt := global.VndOrangeInData
			binbytes, _ := hex.DecodeString("77124700830e8307839069391718068a019288000d0a")
			sh := NewSipHeaders()
			sh.AddHeader(global.Content_Type, global.DicBodyContentType[bt])
			sh.AddHeader(global.Content_Transfer_Encoding, "binary")
			sh.AddHeader(global.Content_Disposition, "signal;handling=optional")
			msgbdy.PartsContents[bt] = ContentPart{Headers: sh, Bytes: binbytes}
		}
	}
}

func (sipmsg *SipMessage) KeepOnlyBodyPart(bt global.BodyType) bool {
	msgbdy := sipmsg.Body
	kys := system.Keys(msgbdy.PartsContents) //get all keys
	if len(kys) == 1 && kys[0] == bt {
		return true //to avoid removing Content-* headers while there is no Content headers inside the single body part
	}
	for _, ky := range kys {
		if ky == bt {
			continue
		}
		delete(msgbdy.PartsContents, ky) //remove other keys
	}
	if len(msgbdy.PartsContents) == 0 { //return if no remaining parts
		return false
	}
	cntprt := msgbdy.PartsContents[bt]
	smhdrs := sipmsg.Headers
	smhdrs.DeleteHeadersWithPrefix("Content-")            //remove all existing Content-* headers
	for _, hdr := range cntprt.Headers.GetHeaderNames() { //set Content-* headers from kept body part
		smhdrs.Set(hdr, cntprt.Headers.Value(hdr))
	}
	msgbdy.PartsContents[bt] = ContentPart{Bytes: cntprt.Bytes}
	return true
}

func (sipmsg *SipMessage) GetBodyPart(bt global.BodyType) ([]byte, bool) {
	cntnt, ok := sipmsg.Body.PartsContents[bt]
	return cntnt.Bytes, ok
}

func (sipmsg *SipMessage) GetSingleBody() (global.BodyType, []byte, bool) {
	if sipmsg.Body.WithNoBody() || sipmsg.Body.WithUnknownBodyPart() {
		return global.None, nil, false
	}
	if len(sipmsg.Body.PartsContents) > 1 {
		return global.None, nil, false
	}
	bt, cp := system.FirstKeyValue(sipmsg.Body.PartsContents)
	return bt, cp.Bytes, true
}

// ===========================================================================

func (sipmsg *SipMessage) IsOutOfDialgoue() bool {
	return sipmsg.ToTag == ""
}

func (sipmsg *SipMessage) GetRSeqFromRAck() (rSeq, cSeq uint32, ok bool) {
	rAck := sipmsg.Headers.ValueHeader(global.RAck)
	if rAck == "" {
		system.LogError(system.LTSIPStack, "Empty RAck header")
		ok = false
		return
	}
	mtch := global.DicFieldRegEx[global.RAckHeader].FindStringSubmatch(rAck)
	if mtch == nil { // Ensure we have both RSeq and CSeq from the match
		system.LogError(system.LTSIPStack, "Malformed RAck header")
		ok = false
		return
	}
	rSeq = system.Str2Uint[uint32](mtch[1])
	cSeq = system.Str2Uint[uint32](mtch[2])
	ok = true
	return
}

func (sipmsg *SipMessage) IsOptionSupportedOrRequired(opt string) bool {
	hdr := sipmsg.Headers.ValueHeader(global.Require)
	if strings.Contains(hdr, opt) {
		return true
	}
	hdr = sipmsg.Headers.ValueHeader(global.Supported)
	return strings.Contains(hdr, opt)
}

func (sipmsg *SipMessage) IsOptionSupported(o string) bool {
	hdr := sipmsg.Headers.ValueHeader(global.Supported)
	hdr = system.ASCIIToLower(hdr)
	return hdr != "" && strings.Contains(hdr, o)
}

func (sipmsg *SipMessage) IsOptionRequired(o string) bool {
	hdr := sipmsg.Headers.ValueHeader(global.Require)
	hdr = system.ASCIIToLower(hdr)
	return hdr != "" && strings.Contains(hdr, o)
}

func (sipmsg *SipMessage) IsMethodAllowed(m global.Method) bool {
	hdr := sipmsg.Headers.ValueHeader(global.Allow)
	hdr = system.ASCIIToLower(hdr)
	return hdr != "" && strings.Contains(hdr, system.ASCIIToLower(m.String()))
}

func (sipmsg *SipMessage) IsKnownRURIScheme() bool {
	for _, s := range global.UriSchemes {
		if s == sipmsg.StartLine.UriScheme {
			return true
		}
	}
	return false
}

// func (sipmsg *SipMessage) GetReferToRUIR() (string, error) {
// 	ok, values := sipmsg.Headers.ValuesHeader(Refer_To)
// 	if !ok {
// 		return "", errors.New("No Refer-To header")
// 	}
// 	if len(values) > 1 {
// 		return "", errors.New("Multiple Refer-To headers found")
// 	}
// 	value := values[0]
// 	if strings.Contains(ASCIIToLower(value), "replaces") {
// 		return "", errors.New("Refer-To with Replaces")
// 	}
// 	var mtch []string
// 	if !RMatch(value, URIFull, &mtch) {
// 		return "", errors.New("Badly formatted URI")
// 	}
// 	return mtch[1], nil
// }

// func (sipmsg *SipMessage) WithNoReferSubscription() bool {
// 	if sipmsg.Headers.DoesValueExistInHeader(Require.String(), "norefersub") {
// 		return true
// 	}
// 	if sipmsg.Headers.DoesValueExistInHeader(Supported.String(), "norefersub") {
// 		return true
// 	}
// 	if sipmsg.Headers.DoesValueExistInHeader(Refer_Sub.String(), "false") {
// 		return true
// 	}
// 	return false
// }

func (sipmsg *SipMessage) IsResponse() bool {
	return sipmsg.MsgType == global.RESPONSE
}

func (sipmsg *SipMessage) IsRequest() bool {
	return sipmsg.MsgType == global.REQUEST
}

func (sipmsg *SipMessage) GetMethod() global.Method {
	return sipmsg.StartLine.Method
}

func (sipmsg *SipMessage) GetStatusCode() int {
	return sipmsg.StartLine.StatusCode
}

func (sipmsg *SipMessage) GetRegistrationData() (contact, ext, ruri, ipport string, expiresInt int) {
	// TODO fix the Regex
	contact = sipmsg.Headers.ValueHeader(global.Contact)
	contact1 := strings.Replace(contact, "-", ";", 1) // Contact: <sip:12345-0x562f8a9e7390@172.20.40.132:5030>;expires=30;+sip.instance="<urn:uuid:da213fce-693c-3403-8455-a548a10ef970>"
	var mtch []string
	if global.RMatch(contact1, global.ContactHeader, &mtch) {
		ruri = mtch[0]
		ext = mtch[2]
		ipport = mtch[5]
	} else {
		expiresInt = -100 // bad contact
		return
	}
	if global.RMatch(contact, global.ExpiresParameter, &mtch) {
		expiresInt = system.Str2Int[int](mtch[1])
		return
	}
	expires := sipmsg.Headers.ValueHeader(global.Expires)
	if expires != "" {
		expiresInt = system.Str2Int[int](expires)
		return
	}
	expires = "3600"
	sipmsg.Headers.SetHeader(global.Expires, expires)
	expiresInt = system.Str2Int[int](expires)
	return
}

func (sipmsg *SipMessage) PrepareMessageBytes(ss *SipSession) {
	var bb bytes.Buffer
	var headers []string

	byteschan := make(chan []byte)

	go func(bc chan<- []byte) {
		var bb2 bytes.Buffer
		if sipmsg.Body.PartsContents == nil {
			sipmsg.Headers.SetHeader(global.Content_Type, "")
			sipmsg.Headers.SetHeader(global.MIME_Version, "")
		} else {
			bdyparts := sipmsg.Body.PartsContents
			if len(bdyparts) == 1 {
				k, v := system.FirstKeyValue(bdyparts)
				sipmsg.Headers.SetHeader(global.Content_Type, global.DicBodyContentType[k])
				sipmsg.Headers.SetHeader(global.MIME_Version, "")
				bb2.Write(v.Bytes)
			} else {
				sipmsg.Headers.SetHeader(global.Content_Type, fmt.Sprintf("multipart/mixed;boundary=%v", global.MultipartBoundary))
				sipmsg.Headers.SetHeader(global.MIME_Version, "1.0")
				isfirstline := true
				for _, ct := range bdyparts {
					if !isfirstline {
						bb2.WriteString("\r\n")
					}
					bb2.WriteString(fmt.Sprintf("--%v\r\n", global.MultipartBoundary))
					for _, h := range ct.Headers.GetHeaderNames() {
						_, values := ct.Headers.Values(h)
						for _, hv := range values {
							bb2.WriteString(fmt.Sprintf("%v: %v\r\n", global.HeaderCase(h), hv))
						}
					}
					bb2.WriteString("\r\n")
					bb2.Write(ct.Bytes)
					isfirstline = false
				}
				bb2.WriteString(fmt.Sprintf("\r\n--%v--\r\n", global.MultipartBoundary))
			}
		}
		bc <- bb2.Bytes()
	}(byteschan)

	//startline
	if sipmsg.IsRequest() {
		sl := sipmsg.StartLine
		bb.WriteString(fmt.Sprintf("%s %s %s\r\n", sl.Method.String(), sl.RUri, global.SipVersion))
		headers = global.DicRequestHeaders[sipmsg.StartLine.Method]
	} else {
		sl := sipmsg.StartLine
		bb.WriteString(fmt.Sprintf("%s %d %s\r\n", global.SipVersion, sl.StatusCode, sl.ReasonPhrase))
		headers = global.DicResponseHeaders[sipmsg.StartLine.StatusCode]
	}

	// var bodybytes []byte
	bodybytes := <-byteschan

	//body - build body type, length, multipart and related headers
	cntntlen := len(bodybytes)

	sipmsg.Headers.SetHeader(global.Content_Length, fmt.Sprintf("%v", cntntlen))

	//headers - build and write
	for _, h := range headers {
		_, values := sipmsg.Headers.Values(h)
		for _, hv := range values {
			if hv != "" {
				bb.WriteString(fmt.Sprintf("%v: %v\r\n", h, hv))
			}
		}
	}

	//P- headers build and write
	pHeaders := sipmsg.Headers.ValuesWithHeaderPrefix("P-")
	for h, hvs := range pHeaders {
		for _, hv := range hvs {
			if hv != "" {
				bb.WriteString(fmt.Sprintf("%v: %v\r\n", h, hv))
			}
		}
	}

	// write separator
	bb.WriteString("\r\n")

	// write body bytes
	bb.Write(bodybytes)

	//save generated bytes for retransmissions
	sipmsg.Body.MessageBytes = bb.Bytes()
}
