package sip

import (
	"errors"
	"fmt"
	. "sipclientgo/global"
	"sipclientgo/q850"
	"sipclientgo/sip/mode"
	"sipclientgo/sip/state"
	"sipclientgo/sip/status"
	"sipclientgo/system"
	"strconv"
	"strings"
)

func processPDU(payload []byte) (*SipMessage, []byte, error) {
	defer func() {
		if r := recover(); r != nil {
			// check if pdu is rqst >> send 400 with Warning header indicating what was wrong or unable to parse
			// or discard rqst if totally wrong
			// if pdu is rsps >> discard
			// in any case, log this pdu by saving its hex stream and why it was wrong
			system.LogCallStack(r)
		}
	}()

	var msgType MessageType
	var startLine SipStartLine

	sipmsg := new(SipMessage)
	msgmap := NewSHsPointer(false)

	var idx int
	var _dblCrLfIdx, _bodyStartIdx, lnIdx, cntntLength uint16

	_dblCrLfIdxInt := system.GetNextIndex(payload, "\r\n\r\n")

	if _dblCrLfIdxInt == -1 {
		//empty sip message
		return nil, nil, nil
	}
	// #nosec G115: Ignoring integer overflow conversion gosec error - payload is always under limit of uint16
	_dblCrLfIdx = uint16(_dblCrLfIdxInt)

	msglines := strings.Split(string(payload[:_dblCrLfIdx]), "\r\n")

	lnIdx = 0
	var matches []string
	//start line parsing
	if RMatch(msglines[lnIdx], RequestStartLinePattern, &matches) {
		msgType = REQUEST
		startLine.StatusCode = 0
		startLine.Method = MethodFromName(system.ASCIIToUpper(matches[1]))
		if startLine.Method == UNKNOWN {
			return sipmsg, nil, errors.New("invalid method for Request message")
		}
		startLine.RUri = matches[2]
		if startLine.Method == INVITE && RMatch(startLine.RUri, INVITERURI, &matches) {
			startLine.UriScheme = system.ASCIIToLower(matches[1])
			startLine.UserPart = system.DropVisualSeparators(matches[2])
			startLine.UserParameters = system.ParseParameters(matches[3])
			startLine.Password = strings.TrimLeft(matches[4], ":")
			startLine.HostPart = matches[5]
			startLine.UriParameters = system.ParseParameters(matches[6])
		}
	} else {
		if RMatch(msglines[lnIdx], ResponseStartLinePattern, &matches) {
			msgType = RESPONSE
			code := system.Str2Int[int](matches[2])
			if code < 100 || code > 699 {
				return nil, nil, errors.New("invalid code for Response message")
			}
			startLine.StatusCode = code
			startLine.ReasonPhrase = matches[3]
			startLine.UriParameters = system.ParseParameters(matches[4])
		} else {
			sipmsg.MsgType = INVALID
			return sipmsg, nil, errors.New("invalid message")
		}
	}
	sipmsg.MsgType = msgType
	sipmsg.StartLine = &startLine

	lnIdx += 1

	//headers parsing
	// #nosec G115: Ignoring integer overflow conversion gosec error - payload is always under limit of uint16
	for i := lnIdx; i < uint16(len(msglines)) && msglines[i] != ""; i++ {
		matches := DicFieldRegEx[FullHeader].FindStringSubmatch(msglines[i])
		if matches != nil {
			headerLC := system.ASCIIToLower(matches[1])
			value := matches[2]
			switch headerLC {
			case From.LowerCaseString():
				tag := DicFieldRegEx[Tag].FindStringSubmatch(value)
				if tag != nil {
					sipmsg.FromTag = tag[1]
				}
				sipmsg.FromHeader = value
			case To.LowerCaseString():
				tag := DicFieldRegEx[Tag].FindStringSubmatch(value)
				if tag != nil {
					sipmsg.ToTag = tag[1]
					if tag[1] != "" && startLine.Method == INVITE {
						startLine.Method = ReINVITE
					}
				}
				sipmsg.ToHeader = value
			case P_Asserted_Identity.LowerCaseString():
				sipmsg.PAIHeaders = append(sipmsg.PAIHeaders, value)
			case Diversion.LowerCaseString():
				sipmsg.DivHeaders = append(sipmsg.DivHeaders, value)
			case Call_ID.LowerCaseString():
				sipmsg.CallID = value
			case Max_Forwards.LowerCaseString():
				max, err := strconv.Atoi(value)
				if err != nil {
					system.LogError(system.LTSIPStack, fmt.Sprintf("Invalid Max-Forwards header - %v", err.Error()))
				} else if max < 0 || max > 255 {
					system.LogError(system.LTSIPStack, "Invalid Max-Forwards header - Too little/big")
				} else {
					sipmsg.MaxFwds = max
				}
			case Contact.LowerCaseString():
				rc := DicFieldRegEx[URIFull].FindStringSubmatch(value)
				if rc != nil {
					sipmsg.RCURI = rc[1]
				}
			case Record_Route.LowerCaseString():
				rc := DicFieldRegEx[URIFull].FindStringSubmatch(value)
				if rc != nil {
					sipmsg.RRURI = rc[1]
				}
			case CSeq.LowerCaseString():
				cseq := DicFieldRegEx[CSeqHeader].FindStringSubmatch(value)
				if cseq == nil {
					system.LogError(system.LTSIPStack, "Invalid CSeq header")
					return nil, nil, errors.New("invalid CSeq header")
				}
				sipmsg.CSeqNum = system.Str2Uint[uint32](cseq[1])
				sipmsg.CSeqMethod = MethodFromName(cseq[2])
				if startLine.StatusCode == 0 {
					r1 := startLine.Method.String()
					r2 := system.ASCIIToUpper(cseq[2])
					if r1 != r2 {
						system.LogError(system.LTSIPStack, fmt.Sprintf("Invalid Request Method: %v vs CSeq Method: %v", r1, r2))
						return nil, nil, errors.New("invalid CSeq header")
					}
				}
			case Via.LowerCaseString():
				via := DicFieldRegEx[ViaBranchPattern].FindStringSubmatch(value)
				if via != nil {
					if sipmsg.ViaBranch == "" {
						sipmsg.ViaBranch = via[1]
					}
					if !strings.HasPrefix(via[1], MagicCookie) {
						system.LogWarning(system.LTSIPStack, fmt.Sprintf("Received message [%v] having non-RFC3261 Via branch [%v]", startLine.Method.String(), via[1]))
						fmt.Println(string(payload[:_dblCrLfIdx]))
					}
					if len(via[1]) <= len(MagicCookie) {
						system.LogWarning(system.LTSIPStack, fmt.Sprintf("Received message [%v] having too short Via branch [%v]", startLine.Method.String(), via[1]))
					}
				}
			}
			msgmap.Add(headerLC, value)
		}
	}

	if ko, hdr := msgmap.AnyMandatoryHeadersMissing(startLine.Method); ko {
		system.LogError(system.LTBadSIPMessage, fmt.Sprintf("Missing mandatory header [%s]", hdr))
		return nil, nil, errors.New("missing mandatory header")
	}

	if msgmap.HeaderCount("CSeq") > 1 {
		system.LogError(system.LTBadSIPMessage, "Duplicate CSeq header")
		return nil, nil, errors.New("duplicate CSeq header")
	}

	if msgmap.HeaderCount("Content-Length") > 1 {
		system.LogError(system.LTBadSIPMessage, "Duplicate Content-Length header")
		return nil, nil, errors.New("duplicate Content-Length header")
	}

	_bodyStartIdx = _dblCrLfIdx + 4 //CrLf x 2

	//automatic deducing of content-length
	// #nosec G115: Ignoring integer overflow conversion gosec error - payload is always under limit of uint16
	cntntLength = uint16(len(payload)) - _bodyStartIdx
	sipmsg.ContentLength = cntntLength

	if ok, values := msgmap.ValuesHeader(Content_Length); ok {
		cntntLength = system.Str2Uint[uint16](values[0])
	} else {
		if ok, _ := msgmap.ValuesHeader(Content_Type); ok {
			msgmap.AddHeader(Content_Length, system.Uint16ToStr(cntntLength))
		} else {
			msgmap.AddHeader(Content_Length, "0")
		}
	}
	sipmsg.Headers = msgmap

	//body parsing
	if cntntLength == 0 {
		payload = payload[_bodyStartIdx:]
		sipmsg.Body = NewMessageBody(false)
		return sipmsg, payload, nil
	}
	// #nosec G115: Ignoring integer overflow conversion gosec error - payload is always under limit of uint16
	if uint16(len(payload)) < _bodyStartIdx+cntntLength {
		system.LogError(system.LTBadSIPMessage, "bad content-length or fragmented pdu")
		return nil, nil, errors.New("bad content-length or fragmented pdu")
	}
	// ---------------------------------
	var MB = NewMessageBody(true)

	var cntntTypeSections map[string]string
	ok, v := msgmap.ValuesHeader(Content_Type)
	if !ok {
		return nil, nil, errors.New("bad message - invalid body")
	}
	cntntTypeSections = system.CleanAndSplitHeader(v[0])
	if cntntTypeSections == nil {
		system.LogWarning(system.LTSIPStack, "Content-Type header is missing while Content-Length is non-zero - Message skipped")
		return nil, nil, errors.New("bad message - invalid body")
	}

	cntntType := system.ASCIIToLower(cntntTypeSections["!headerValue"])

	if !strings.Contains(cntntType, "multipart") {
		bt := GetBodyType(cntntType)
		if bt == Unknown {
			system.LogError(system.LTBadSIPMessage, "Unknown Content-Type header")
		} else {
			MB.PartsContents[bt] = ContentPart{Bytes: payload[_bodyStartIdx : _bodyStartIdx+cntntLength]}
		}
		payload = payload[_bodyStartIdx+cntntLength:]
	} else {
		payload = payload[_bodyStartIdx:]
		boundary := cntntTypeSections["boundary"]
		markBoundary := "--" + boundary
		endBoundary := "--" + boundary + "--\r\n"
		var idxEnd, partsCount int
		for {
			idx = system.GetNextIndex(payload, markBoundary)
			if idx == -1 || string(payload) == endBoundary {
				break
			}
			payload = payload[idx+len(markBoundary)+2:]
			idx = system.GetNextIndex(payload, "\r\n\r\n")
			idxEnd = system.GetNextIndex(payload, markBoundary)
			msglines = strings.Split(string(payload[:idx]), "\r\n")
			bt := None
			partHeaders := NewSipHeaders()
			for _, ln := range msglines {
				matches = DicFieldRegEx[FullHeader].FindStringSubmatch(ln)
				if matches != nil {
					h := matches[1]
					partHeaders.Add(h, matches[2])
					if Content_Type.Equals(h) {
						cntntType = matches[2]
						bt = GetBodyType(cntntType)
					}
				}
			}
			switch bt {
			case None:
				system.LogError(system.LTBadSIPMessage, "Missing body part Content-Type - skipped")
			case Unknown:
				system.LogError(system.LTBadSIPMessage, "Unknown Content-Type header - skipped")
			default:
				MB.PartsContents[bt] = ContentPart{
					Headers: partHeaders,
					Bytes:   payload[idx+4 : idxEnd-2], //start_after \r\n\r\n (body_start) = +4 and end_before \r\n = -2 (boundary_edge)
				}
			}
			payload = payload[idxEnd:]
			partsCount++
		}
		if len(MB.PartsContents) < partsCount {
			system.LogError(system.LTBadSIPMessage, "One or more body parts have been skipped")
		}
	}

	sipmsg.Body = MB

	return sipmsg, payload, nil
}

func sessionGetter(sipmsg *SipMessage, ue *UserEquipment) (*SipSession, NewSessionType) {
	defer func() {
		if r := recover(); r != nil {
			system.LogCallStack(r)
		}
	}()

	callID := sipmsg.CallID
	ss, ok := ue.SesMap.Load(callID)
	if ok {
		sipses := ss
		if sipses.IsDuplicateMessage(sipmsg) {
			return sipses, DuplicateMessage
		}
		return sipses, ValidRequest
	} else {
		if sipmsg.IsResponse() {
			return nil, Response
		}
		sipses := NewSIPSession(sipmsg)
		ue.SesMap.Store(callID, sipses)
		sipses.UserEquipment = ue
		if sipmsg.ToTag == "" {
			switch sipmsg.GetMethod() {
			case INVITE:
				sipses.Mode = mode.Multimedia
				sipses.IsPRACKSupported = sipmsg.IsOptionSupported("100rel")
				sipses.IsDelayedOfferCall = !sipmsg.Body.ContainsSDP()
				sipses.SetState(state.BeingEstablished)
				if !sipmsg.IsKnownRURIScheme() {
					return sipses, UnsupportedURIScheme
				}
				if sipmsg.Body.WithUnknownBodyPart() {
					return sipses, UnsupportedBody
				}
				if sipmsg.Headers.HeaderExists("Require") {
					return sipses, WithRequireHeader
				}
				if sipmsg.MaxFwds <= MinMaxFwds {
					return sipses, TooLowMaxForwards
				}
				return sipses, ValidRequest
			case MESSAGE:
				sipses.Mode = mode.Messaging
				return sipses, ValidRequest
			case SUBSCRIBE:
				sipses.Mode = mode.Subscription
				return sipses, ValidRequest
			case OPTIONS:
				sipses.Mode = mode.KeepAlive
				return sipses, ValidRequest
			case REGISTER:
				sipses.Mode = mode.Registration
				return sipses, ValidRequest
			case REFER, NOTIFY, UPDATE, PRACK, INFO, PUBLISH, NEGOTIATE:
				return sipses, InvalidRequest
			case ACK:
				return sipses, UnExpectedMessage
			default:
				return sipses, CallLegTransactionNotExist
			}
		}
		return sipses, CallLegTransactionNotExist
	}
}

func sipStack(sipmsg *SipMessage, ss *SipSession, newSesType NewSessionType) {
	defer func() {
		if r := recover(); r != nil {
			system.LogCallStack(r)
		}
	}()

	if ss == nil || newSesType == DuplicateMessage {
		return
	}
	ss.UpdateContactRecordRouteBody(sipmsg) //update -- split headers logic

	var trans *Transaction
	if sipmsg.IsRequest() {
		trans = ss.AddIncomingRequest(sipmsg, nil)
	} else {
		trans = ss.AddIncomingResponse(sipmsg)
	}

	if trans == nil {
		ss.DropMe()
		return
	}

	switch newSesType {
	case Response:
		return
	case UnExpectedMessage:
		ss.DropMe()
		return
	case TooLowMaxForwards:
		ss.RejectMe(trans, status.TooManyHops, q850.NoRCProvided, "INVITE with too low MF")
		return
	case WithRequireHeader:
		ss.RejectMe(trans, status.BadExtension, q850.NoRCProvided, "INVITE with Require header")
		return
	case UnsupportedURIScheme:
		ss.RejectMe(trans, status.UnsupportedURIScheme, q850.NoRCProvided, "URI scheme unsupported")
		return
	case UnsupportedBody:
		ss.RejectMe(trans, status.UnsupportedMediaType, q850.NoRCProvided, "Message body unsupported")
		return
	case ExceededCallRate:
		ss.RejectMe(trans, status.ServiceUnavailable, q850.NoCircuitChannelAvailable, "Call rate exceeded")
		return
	case InvalidRequest:
		ss.SetState(state.BeingFailed)
		ss.SendResponse(trans, 503, EmptyBody())
		ss.DropMe()
		return
	case CallLegTransactionNotExist:
		if sipmsg.StartLine.Method != ACK {
			ss.SetState(state.Dropped)
			ss.SendResponse(trans, 481, EmptyBody())
		}
		ss.DropMe()
		return
	}

	if sipmsg.IsRequest() {
		if sipmsg.Body.WithUnknownBodyPart() {
			ss.SendResponse(trans, status.UnsupportedMediaType, EmptyBody())
			return
		}
		switch method := sipmsg.GetMethod(); method {
		case INVITE:
			ss.logSessData(nil, nil)
			ss.SendResponse(trans, status.Trying, EmptyBody())
			ss.RouteRequestInternal(trans, sipmsg)
		case ReINVITE:
			ss.SendResponse(trans, 100, EmptyBody())
			if !ss.ChecknSetDialogueChanging(true) {
				ss.SendResponseDetailed(trans, NewResponsePackRFWarning(status.RequestPending, "", "Competing ReINVITE rejected"), EmptyBody())
				return
			}
			switch {
			case sipmsg.Body.WithNoBody():
				ss.SendResponseDetailed(trans, NewResponsePackSIPQ850Details(status.NotAcceptableHere, q850.BearerCapabilityNotImplemented, "Not supported delayed offer"), EmptyBody())
			case sipmsg.Body.ContainsSDP():
				sc, qc, wr := ss.buildSDPAnswer(sipmsg)
				if sc != 0 {
					ss.SendResponseDetailed(trans, NewResponsePackSIPQ850Details(sc, qc, wr), EmptyBody())
					return
				}
				ss.SendResponse(trans, status.OK, NewMessageSDPBody(ss.LocalSDP))
			default:
				ss.SendResponseDetailed(trans, NewResponsePackSIPQ850Details(status.ServiceUnavailable, q850.InterworkingUnspecified, "Not supported action"), EmptyBody())
			}
		case ACK:
			if trans.Method == INVITE {
				ss.FinalizeState()
				if !ss.IsEstablished() {
					ss.logSessData(nil, nil)
					ss.DropMe()
					return
				}
				ss.logSessData(utcNow(), nil)
				ss.StartMaxCallDuration()
				ss.StartInDialogueProbing()
				go ss.mediaReceiver()
			} else { //ReINVITE
				if trans.IsFinalResponsePositiveSYNC() {
					ss.ChecknSetDialogueChanging(false)
					// go ss.startRTPStreaming("ErsemAlb", false, true, true)
				}
			}
		case CANCEL:
			if !ss.IsBeingEstablished() {
				ss.SendResponseDetailed(trans, ResponsePack{StatusCode: 400, ReasonPhrase: "Incompatible Method With Session State"}, EmptyBody())
				return
			}
			ss.SetState(state.BeingCancelled)
			ss.SendResponse(trans, 200, EmptyBody())
			ss.SendResponseDetailed(nil, ResponsePack{StatusCode: 487, CustomHeaders: NewSHQ850OrSIP(487, "", "")}, EmptyBody())
			ss.logSessData(nil, utcNow())
		case BYE:
			if !ss.IsEstablished() {
				ss.SendResponseDetailed(trans, ResponsePack{StatusCode: 400, ReasonPhrase: "Incompatible Method With Session State"}, EmptyBody())
				return
			}
			ss.SetState(state.Cleared)
			ss.SendResponse(trans, status.OK, EmptyBody())
			ss.logSessData(nil, utcNow())
			ss.DropMe()
		case OPTIONS:
			if sipmsg.IsOutOfDialgoue() { // incoming probing
				ss.SetState(state.Probed)
				ss.SendResponse(trans, 200, EmptyBody())
				ss.DropMe()
				return
			}
			ss.SendResponse(trans, 200, EmptyBody())
		case UPDATE:
			switch {
			case sipmsg.Body.WithNoBody():
				ss.SendResponse(trans, 200, EmptyBody())
			case sipmsg.Body.ContainsSDP():
				sc, qc, wr := ss.buildSDPAnswer(sipmsg)
				if sc != 0 {
					ss.SendResponseDetailed(trans, NewResponsePackSIPQ850Details(sc, qc, wr), EmptyBody())
					return
				}
				ss.SendResponse(trans, status.OK, NewMessageSDPBody(ss.LocalSDP))
			default:
				ss.SendResponseDetailed(trans, NewResponsePackSIPQ850Details(status.ServiceUnavailable, q850.InterworkingUnspecified, "Not supported action"), EmptyBody())
			}
		case PRACK:
			ss.SendResponse(trans, status.OK, EmptyBody())
		case INFO:
			if sipmsg.Body.WithNoBody() {
				ss.SendResponse(trans, status.OK, EmptyBody())
				return
			}
			btype, bytes, ok := sipmsg.GetSingleBody()
			if !ok {
				ss.SendResponseDetailed(trans, NewResponsePackSIPQ850Details(status.UnsupportedMediaType, q850.RequestedFacilityNotImplemented, "Not supported action/event"), EmptyBody())
				return
			}
			switch btype {
			case DTMF, DTMFRelay:
				ss.parseDTMF(bytes, method, btype)
				ss.SendResponse(trans, status.OK, EmptyBody())
			}
		default: //REFER, REGISTER, SUBSCRIBE, MESSAGE, PUBLISH, NEGOTIATE
			ss.SetState(state.Dropped)
			ss.SendResponse(trans, status.MethodNotAllowed, EmptyBody())
			ss.DropMe()
		}
	} else {
		stsCode := sipmsg.StartLine.StatusCode
		if stsCode <= 199 && trans.Method != INVITE {
			return
		}
		switch {
		case 180 <= stsCode && stsCode <= 189:
		case stsCode <= 199:
		case stsCode <= 299:
			switch trans.Method {
			case INVITE:
				ss.FinalizeState()
				ss.SendRequest(ACK, trans, EmptyBody())
				ss.logSessData(utcNow(), nil)
			case REGISTER:
				ss.FinalizeState()
				ss.logRegData(sipmsg)
				ss.DropMe()
			case ReINVITE:
				ss.SendRequest(ACK, trans, EmptyBody())
				ss.logSessData(nil, nil)
			case INFO:
			case OPTIONS: //probing or keepalive
				if ss.Mode == mode.KeepAlive {
					ss.FinalizeState()
					ss.RemoteUserAgent.IsAlive = true
					ss.DropMe()
				}
			case BYE:
				ss.StopAllOutTransactions()
				ss.FinalizeState()
				ss.logSessData(nil, nil)
				ss.DropMe()
			}
		case stsCode <= 399:
		default: // 400-699
			switch trans.Method {
			case INVITE:
				if ss.IsBeingEstablished() {
					ss.SetState(state.Rejected)
				} else {
					ss.FinalizeState()
				}
				ss.SendRequest(ACK, trans, EmptyBody())
				ss.logSessData(nil, utcNow())
				ss.DropMe()
			case REGISTER:
				sipstate := ss.SetState(state.Failed)
				ss.logRegData(sipmsg)
				defer ss.DropMe()
				if wwwauth := sipmsg.Headers.ValueHeader(WWW_Authenticate); wwwauth != "" {
					if sipstate == state.BeingUnregistered {
						go UnregisterMe(ss.UserEquipment, wwwauth)
					} else {
						go RegisterMe(ss.UserEquipment, wwwauth)
					}
				}
			case OPTIONS: //probing or keepalive
				if ss.Mode == mode.KeepAlive {
					ss.FinalizeState()
					ss.DropMe()
				}
			}

		}
	}
}
