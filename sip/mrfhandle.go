package sip

import (
	"encoding/binary"
	"fmt"
	"math"
	"regexp"
	"sipclientgo/dtmf"
	. "sipclientgo/global"
	"sipclientgo/guid"
	"sipclientgo/q850"
	"sipclientgo/rtp"
	"sipclientgo/sdp"
	"sipclientgo/sip/state"
	"sipclientgo/sip/status"

	"net"
	"sipclientgo/system"
	"slices"
	"strings"
	"time"
)

func (ss *SipSession) RouteRequestInternal(trans *Transaction, sipmsg1 *SipMessage) {
	defer func() {
		if r := recover(); r != nil {
			system.LogCallStack(r)
		}
	}()

	upart := sipmsg1.StartLine.UserPart

	if !sipmsg1.Body.ContainsSDP() {
		ss.RejectMe(trans, status.NotAcceptableHere, q850.BearerCapabilityNotImplemented, "Not supported SDP or delayed offer")
		return
	}

	repo, ok := MRFRepos.GetMRFRepo(upart)
	if !ok {
		ss.RejectMe(trans, status.NotFound, q850.UnallocatedNumber, "MRF Repository not found")
		return
	}

	ss.MRFRepo = repo

	ss.answerMRF(trans, sipmsg1)
}

// ============================================================================
// MRF methods
func (ss *SipSession) buildSDPAnswer(sipmsg *SipMessage) (sipcode, q850code int, warn string) {
	sdpbytes, _ := sipmsg.GetBodyPart(SDP)
	sdpses, err := sdp.Parse(sdpbytes)
	if err != nil {
		sipcode = status.UnsupportedMediaType
		q850code = q850.BearerCapabilityNotImplemented
		warn = "Not supported SDP"
		return
	}
	var media *sdp.Media
	var conn *sdp.Connection = sdpses.Connection
	var audioFormat *sdp.Format
	var dtmfFormat *sdp.Format
	for i := 0; i < len(sdpses.Media); i++ {
		media = sdpses.Media[i]
		if media.Type != sdp.Audio || media.Port == 0 || media.Proto != sdp.RtpAvp || (conn == nil && len(media.Connection) == 0) { //|| media.Mode != sdp.SendRecv
			continue
		}
		for j := 0; j < len(media.Connection); j++ {
			connection := media.Connection[j]
			if connection.Type != sdp.TypeIPv4 || connection.Network != sdp.NetworkInternet { //connection.Address == "0.0.0.0"
				continue
			}
			conn = connection
			break
		}
		for k := 0; k < len(media.Format); k++ {
			frmt := media.Format[k]
			if frmt.Channels != 1 || frmt.ClockRate != 8000 || !slices.Contains(sdp.SupportedCodecs, frmt.Payload) {
				continue
			}
			audioFormat = frmt
			break
		}
		for k := 0; k < len(media.Format); k++ {
			frmt := media.Format[k]
			if frmt.Name == sdp.TelephoneEvents {
				dtmfFormat = frmt
				break
			}
		}
		media.Chosen = true
		break
	}

	if conn == nil {
		sipcode = status.NotAcceptableHere
		q850code = q850.MandatoryInformationElementIsMissing
		warn = "No media connection found"
		return
	}

	if media == nil {
		sipcode = status.NotAcceptableHere
		q850code = q850.BearerCapabilityNotAvailable
		warn = "No SDP audio offer found"
		return
	}

	if audioFormat == nil {
		sipcode = status.NotAcceptableHere
		q850code = q850.IncompatibleDestination
		warn = "No common audio codec found"
		return
	}

	if system.Str2Int[int](sdpses.GetEffectivePTime()) != PacketizationTime {
		sipcode = status.NotAcceptableHere
		q850code = q850.BearerCapabilityNotImplemented
		warn = "Packetization other than 20ms not supported"
		return
	}

	rmedia, err := system.BuildUDPAddr(conn.Address, media.Port)
	if err != nil {
		sipcode = status.NotAcceptableHere
		q850code = q850.ChannelUnacceptable
		warn = "Unable to parse received connection IPv4"
		return
	}

	ss.RemoteMedia = rmedia
	ss.IsCallHeld = sdpses.IsCallHeld()

	// TODO need to handle CANCEL (put some delay before answering?)
	if ss.MediaListener == nil {
		ss.MediaListener = MediaPorts.ReserveSocket()
	}
	if ss.MediaListener == nil {
		sipcode = status.NotAcceptableHere
		q850code = q850.ResourceUnavailableUnspecified
		warn = "Media pool depleted"
		return
	}

	mySDP := &sdp.Session{
		Origin: &sdp.Origin{
			Username:       "mt",
			SessionID:      ss.SDPSessionID,
			SessionVersion: ss.SDPSessionVersion,
			Network:        sdp.NetworkInternet,
			Type:           sdp.TypeIPv4,
			Address:        ClientIPv4.String(),
		},
		Name: "MRF",
		// Information: "A Seminar on the session description protocol",
		// URI:         "http://www.example.com/seminars/sdp.pdf",
		// Email:       []string{"j.doe@example.com (Jane Doe)"},
		// Phone:       []string{"+1 617 555-6011"},
		Connection: &sdp.Connection{
			Network: sdp.NetworkInternet,
			Type:    sdp.TypeIPv4,
			Address: ClientIPv4.String(),
			TTL:     0,
		},
		// Bandwidth: []*Bandwidth{
		// 	{"AS", 2000},
		// },
		// Timing: &Timing{
		// 	Start: parseTime("1996-02-27 15:26:59 +0000 UTC"),
		// 	Stop:  parseTime("1996-05-30 16:26:59 +0000 UTC"),
		// },
		// Repeat: []*Repeat{
		// 	{
		// 		Interval: time.Duration(604800) * time.Second,
		// 		Duration: time.Duration(3600) * time.Second,
		// 		Offsets: []time.Duration{
		// 			time.Duration(0),
		// 			time.Duration(90000) * time.Second,
		// 		},
		// 	},
		// },
		// TimeZone: []*TimeZone{
		// 	{Time: parseTime("1996-02-27 15:26:59 +0000 UTC"), Offset: -time.Hour},
		// 	{Time: parseTime("1996-05-30 16:26:59 +0000 UTC"), Offset: 0},
		// },
	}

	for i := 0; i < len(sdpses.Media); i++ {
		media := sdpses.Media[i]
		var newmedia *sdp.Media
		if media.Chosen {
			newmedia = &sdp.Media{
				Chosen:     true,
				Type:       "audio",
				Port:       system.GetUDPortFromConn(ss.MediaListener),
				Proto:      "RTP/AVP",
				Format:     []*sdp.Format{audioFormat},
				Attributes: []*sdp.Attr{{Name: "ptime", Value: "20"}},
				Mode:       sdp.NegotiateMode(sdp.SendRecv, sdpses.GetEffectiveMediaDirective())}
			if dtmfFormat != nil {
				newmedia.Format = append(newmedia.Format, dtmfFormat)
			}
		} else {
			newmedia = &sdp.Media{Type: media.Type, Port: 0, Proto: media.Proto}
		}
		mySDP.Media = append(mySDP.Media, newmedia)
	}

	if ss.LocalSDP != nil && !mySDP.Equals(ss.LocalSDP) {
		ss.SDPSessionVersion += 1
		mySDP.Origin.SessionVersion = ss.SDPSessionVersion
	}

	ss.LocalSDP = mySDP
	ss.rtpPayloadType = audioFormat.Payload
	ss.WithTeleEvents = dtmfFormat != nil

	if !ss.WithTeleEvents {
		ss.audioBytes = make([]byte, 0, DTMFPacketsCount*PayloadSize)
	}

	return
}

func (ss *SipSession) answerMRF(trans *Transaction, sipmsg *SipMessage) {
	if sc, qc, wr := ss.buildSDPAnswer(sipmsg); sc != 0 {
		ss.RejectMe(trans, sc, qc, wr)
		return
	}

	// initializations
	ss.rtpSSRC = system.RandomNum(2000, 9000000)
	ss.rtpSequenceNum = uint16(system.RandomNum(1000, 2000))
	ss.rtpTimeStmp = 0
	ss.SDPSessionID = int64(system.RandomNum(1000, 9000))
	ss.SDPSessionVersion = 1

	ss.SendResponse(trans, status.Ringing, EmptyBody())

	<-time.After(AnswerDelay * time.Millisecond)

	if !ss.IsBeingEstablished() {
		return
	}

	// ss.speechBytes = make([]byte, 0, 2*50*160)

	ss.SendResponse(trans, status.OK, NewMessageSDPBody(ss.LocalSDP.Bytes()))
}

func (ss *SipSession) mediaReceiver() {
	for {
		if ss.MediaListener == nil {
			return
		}
		buf := RTPRXBufferPool.Get().(*[]byte)
		n, addr, err := ss.MediaListener.ReadFromUDP(*buf)
		if err != nil {
			if buf != nil {
				RTPRXBufferPool.Put(buf)
			}
			if opErr, ok := err.(*net.OpError); ok {
				_ = opErr
				return
			}
			fmt.Println(err)
			continue
		}

		if !system.AreUAddrsEqual(addr, ss.RemoteMedia) {
			fmt.Println("Received RTP from unknown remote connection")
			continue
		}

		bytes := (*buf)[:n]
		payload := bytes[RTPHeadersSize:]

		if ss.WithTeleEvents {
			if n == 16 { // TODO check if no RFC 4733 is negotiated - transcode InBand DTMF into teleEvents
				ts := binary.BigEndian.Uint32(bytes[4:8]) //TODO check how to use IsSystemBigEndian
				if ss.rtpRFC4733TS != ts {
					ss.rtpRFC4733TS = ts
					dtmf := DicDTMFEvent[bytes[12]]
					ss.processDTMF(dtmf, "Inband - RTP Telephone Event (RFC 4733) - Received: ")
					// switch dtmf {
					// case "DTMF #":
					// 	// ss.stopRTPStreaming() // TODO use this if audiofile can be interrupted by any DTMF or a specific DTMF or not at all
					// case "DTMF *":

					// }
				}
			}
		} else {
			if n == RTPHeadersSize+PayloadSize {
				b1 := bytes[1]

				if b1 >= 128 {
					ss.NewDTMF = true
					ss.audioBytes = ss.audioBytes[:0]
				} else if ss.NewDTMF {
					if len(ss.audioBytes) == DTMFPacketsCount*len(payload) {
						ss.audioBytes = append(ss.audioBytes, payload...)
						ss.NewDTMF = false
						pcm := rtp.DecodeToPCM(ss.audioBytes, ss.rtpPayloadType)
						signal := dtmf.DetectDTMF(pcm)
						if signal != "" {
							dtmf := DicDTMFEvent[DicDTMFSignal[signal]]
							frmt := ss.LocalSDP.GetChosenMedia().FormatByPayload(ss.rtpPayloadType)
							ss.processDTMF(dtmf, fmt.Sprintf("Inband - RTP Audio Tone (%s) - Received: ", frmt.Name))
						}
					} else {
						ss.audioBytes = append(ss.audioBytes, payload...)
					}
				}
			}
		}

		// if n == RTPHeadersSize+PayloadSize && ss.collectSpeech {
		// 	if len(ss.speechBytes) == 2*50*160 {
		// 		pcmData := rtp.DecodeToPCM(ss.speechBytes, ss.rtpPayloadType)
		// 		pcmBytes := rtp.Int16stoBytes(pcmData)
		// 		// txt, err := speech.TranscribePCM(pcmBytes)
		// 		// speech.TranscribePCM(pcmBytes)
		// 		// if err != nil {
		// 		// 	system.LogWarning(system.LTChatMessage, err.Error())
		// 		// } else {
		// 		// 	system.LogInfo(system.LTChatMessage, txt)
		// 		// }
		// 		ss.collectSpeech = false
		// 		ss.speechBytes = ss.speechBytes[:0]
		// 		fmt.Println("Speech collected")
		// 	} else {
		// 		ss.speechBytes = append(ss.speechBytes, payload...)
		// 	}
		// }

		RTPRXBufferPool.Put(buf)
	}
}

func (ss *SipSession) parseDTMF(bytes []byte, m Method, bt BodyType) {
	strng := string(bytes)
	var mtch []string
	var signal string
	if bt == DTMFRelay {
		for _, ln := range strings.Split(strng, "\r\n") {
			if RMatch(ln, SignalDTMF, &mtch) {
				signal = mtch[1]
				break
			}
		}
	} else {
		signal = strng
	}
	if signal == "" {
		return
	}
	dtmf := DicDTMFEvent[DicDTMFSignal[signal]]
	ss.processDTMF(dtmf, fmt.Sprintf("OOB - SIP %s (%s) - Received: ", m.String(), DicBodyContentType[bt]))
}

func (ss *SipSession) processDTMF(dtmf, details string) {
	ss.lastDTMF = dtmf
	// if dtmf == "DTMF #" && !ss.collectSpeech {
	// 	fmt.Println("Received:", dtmf, " - Begin collecting speech")
	// 	ss.collectSpeech = true
	// }
	if ss.bargeEnabled && ss.stopRTPStreaming() {
		system.LogInfo(system.LTMediaCapability, "Audio streaming has been interrupted")
	}
	system.LogInfo(system.LTDTMF, details+dtmf)
}

func (ss *SipSession) stopRTPStreaming() bool {
	ss.rtpmutex.Lock()
	if !ss.isrtpstreaming {
		ss.rtpmutex.Unlock()
		return false
	}
	ss.rtpmutex.Unlock()

	select {
	case ss.rtpChan <- true:
		return true
	default:
		<-ss.rtpChan
	}
	return false
}

func (ss *SipSession) startRTPStreaming(audiokey string, resetflag, loopflag, dropCallflag bool) bool {
	ss.rtpmutex.Lock()
	if ss.isrtpstreaming {
		ss.rtpmutex.Unlock()
		return true
	}
	ss.isrtpstreaming = true
	ss.rtpmutex.Unlock()

	origPayload := ss.rtpPayloadType

	// { To test transcoding is not corrupting data
	// 	g722 := rtp.PCM2G722(pcm)
	// 	pcm = rtp.G722toPCM(g722)
	// 	ulaw := rtp.PCM2G711U(pcm)
	// 	pcm = rtp.G711U2PCM(ulaw)
	// 	alaw := rtp.PCM2G711A(pcm)
	// 	pcm = rtp.G711A2PCM(alaw)
	// }

	isFinished := true // to know that streaming has reached its end

	{
		data, silence, ok := ss.MRFRepo.GetTx(audiokey, origPayload)
		if !ok {
			goto finish1
		}

		tckr := time.NewTicker(20 * time.Millisecond)
		defer tckr.Stop()

		Marker := true

		if resetflag {
			ss.rtpIndex = 0
		}

		for {
			select {
			case <-ss.rtpChan:
				isFinished = false
				goto finish2
			case <-tckr.C:
			}

			if origPayload != ss.rtpPayloadType {
				defer ss.startRTPStreaming(audiokey, false, loopflag, dropCallflag)
				goto finish1
			}

			// TODO uncomment below to allow pausing streaming when call is held
			// if ss.IsCallHeld {
			// 	goto finish1
			// }

			ss.rtpTimeStmp += uint32(RTPPayloadSize)
			if ss.rtpSequenceNum == math.MaxUint16 {
				ss.rtpSequenceNum = 0
			} else {
				ss.rtpSequenceNum++
			}

			delta := len(data) - ss.rtpIndex
			var payload []byte
			if RTPPayloadSize <= delta {
				payload = (data)[ss.rtpIndex : ss.rtpIndex+RTPPayloadSize]
				ss.rtpIndex += RTPPayloadSize
				isFinished = false
			} else {
				payload = (data)[ss.rtpIndex : ss.rtpIndex+delta]
				for n := delta; n < RTPPayloadSize; n++ {
					payload = append(payload, silence)
				}
				ss.rtpIndex += delta
				isFinished = true
			}

			if !ss.IsCallHeld {
				pktptr := RTPTXBufferPool.Get().(*[]byte)
				pkt := (*pktptr)[:0]
				pkt = append(pkt, 128)
				pkt = append(pkt, bool2byte(Marker)*128+ss.rtpPayloadType)
				pkt = append(pkt, uint16ToBytes(ss.rtpSequenceNum)...)
				pkt = append(pkt, uint32ToBytes(ss.rtpTimeStmp)...)
				pkt = append(pkt, uint32ToBytes(ss.rtpSSRC)...)
				pkt = append(pkt, payload...)
				_, err := ss.MediaListener.WriteToUDP(pkt, ss.RemoteMedia)
				if err != nil {
					goto finish1
				}
				RTPTXBufferPool.Put(pktptr)
			}

			Marker = false

			if isFinished {
				if loopflag {
					ss.rtpIndex = 0
					isFinished = false
					Marker = true
					continue
				}
				ss.rtpIndex = 0
				break
			}
		}
	}

finish1:
	select {
	case <-ss.rtpChan:
	default:
	}

finish2:
	ss.rtpmutex.Lock()
	ss.isrtpstreaming = false
	ss.rtpmutex.Unlock()

	if dropCallflag {
		ss.ReleaseMe("audio playback ended")
	}

	return !isFinished
}

// =========================================================================================================================

func bool2byte(b bool) byte {
	if b {
		return 1
	}
	return 0
}

func uint16ToBytes(num uint16) []byte {
	bytes := make([]byte, 2)
	binary.BigEndian.PutUint16(bytes, num)
	return bytes
}

func uint32ToBytes(num uint32) []byte {
	bytes := make([]byte, 4)
	binary.BigEndian.PutUint32(bytes, num)
	return bytes
}

// ============================================================================

func ProbeUA(conn *net.UDPConn, ua *SipUdpUserAgent) {
	if conn == nil || ua == nil {
		return
	}
	ss := NewSS(OUTBOUND)
	ss.RemoteUDP = ua.UDPAddr
	ss.SIPUDPListenser = conn
	ss.RemoteUserAgent = ua

	hdrs := NewSipHeaders()
	hdrs.AddHeader(Subject, "Out-of-dialogue keep-alive")
	hdrs.AddHeader(Accept, "application/sdp")

	trans := ss.CreateSARequest(RequestPack{Method: OPTIONS, Max70: true, CustomHeaders: hdrs, RUriUP: "ping", FromUP: "ping", IsProbing: true}, EmptyBody())

	ss.SetState(state.BeingProbed)
	ss.AddMe()
	ss.SendSTMessage(trans)
}

func StartUEListener(ue *UserEquipment) error {
	ul, err := system.StartListening(ClientIPv4, ue.UdpPort)
	if err != nil {
		return err
	}
	ue.DataChan = make(chan Packet, QueueSize)
	startWorkers(ul, ue.DataChan)
	udpLoopWorkers(ul, ue.DataChan)
	ue.UDPListener = ul
	// go RegisterMe(ue, "")
	return nil
}

func RegisterMe(ue *UserEquipment, wwwauth string) {
	if PCSCFSocket == nil {
		system.LogError(system.LTConfiguration, "Missing PCSCF Socket")
		return
	}

	ss := NewSS(OUTBOUND)
	ss.RemoteUDP = PCSCFSocket
	ss.SIPUDPListenser = ue.UDPListener
	ss.UserEquipment = ue

	hdrs := NewSipHeaders()
	hdrs.AddHeader(P_Access_Network_Info, "IEEE-802.3") //"3GPP-E-UTRAN-FDD; utran-cell-id-3gpp=001010001000019B")
	hdrs.AddHeader(Expires, "600000")
	hdrs.AddHeader(Supported, "path")
	hdrs.AddHeader(Contact, fmt.Sprintf(`<sip:%s;transport=udp>;+g.3gpp.icsi-ref="urn:Aurn-7:3gpp-service.ims.icsi.mmtel";+g.3gpp.smsip;video;+sip.instance="<urn:gsma:imei:86728703-952237-0>";+g.3gpp.accesstype="wired"`, system.GetUDPAddrStringFromConn(ue.UDPListener)))
	// hdrs.AddHeader(Contact, fmt.Sprintf(`<sip:%s>;+g.3gpp.icsi-ref="urn:Aurn-7:3gpp-service.ims.icsi.mmtel";+g.3gpp.smsip;video;+sip.instance="<urn:gsma:imei:86728703-952237-0>";+g.3gpp.accesstype="wired"`, system.GetUDPAddrStringFromConn(ue.UDPListener)))

	if wwwauth != "" {
		auths := ParseWWWAuthenticateOptimized(wwwauth)
		author := computeAuthorizationHeader(ImsDomain, auths[0].Params["nonce"], REGISTER.String(), "00000001", ue)
		hdrs.AddHeader(Authorization, author)
		ue.Authorization = author
	}

	trans := ss.CreateSARequest(RequestPack{Method: REGISTER, Max70: true, RUriUP: ue.Imsi, FromUP: ue.Imsi, CustomHeaders: hdrs}, EmptyBody())

	ss.SetState(state.BeingRegistered)
	ss.AddMe()
	ss.SendSTMessage(trans)
}

func UnregisterMe(ue *UserEquipment, wwwauth string) {
	if PCSCFSocket == nil {
		system.LogError(system.LTConfiguration, "Missing PCSCF Socket")
		return
	}

	ss := NewSS(OUTBOUND)
	ss.RemoteUDP = PCSCFSocket
	ss.SIPUDPListenser = ue.UDPListener
	ss.UserEquipment = ue
	ss.IsUnregistering = true

	hdrs := NewSipHeaders()
	// hdrs.AddHeader(P_Access_Network_Info, "IEEE-802.3") //"3GPP-E-UTRAN-FDD; utran-cell-id-3gpp=001010001000019B")
	hdrs.AddHeader(Expires, "0")
	// hdrs.AddHeader(Supported, "path")
	hdrs.AddHeader(Contact, fmt.Sprintf(`<sip:%s;transport=udp>;+g.3gpp.icsi-ref="urn:Aurn-7:3gpp-service.ims.icsi.mmtel";+g.3gpp.smsip;video;+sip.instance="<urn:gsma:imei:86728703-952237-0>";+g.3gpp.accesstype="wired"`, system.GetUDPAddrStringFromConn(ue.UDPListener)))
	// hdrs.AddHeader(Contact, fmt.Sprintf(`<sip:%s>;+g.3gpp.icsi-ref="urn:Aurn-7:3gpp-service.ims.icsi.mmtel";+g.3gpp.smsip;video;+sip.instance="<urn:gsma:imei:86728703-952237-0>";+g.3gpp.accesstype="wired"`, system.GetUDPAddrStringFromConn(ue.UDPListener)))

	if wwwauth != "" {
		auths := ParseWWWAuthenticateOptimized(wwwauth)
		author := computeAuthorizationHeader(ImsDomain, auths[0].Params["nonce"], REGISTER.String(), "00000001", ue)
		hdrs.AddHeader(Authorization, author)
		ue.Authorization = author
	}

	trans := ss.CreateSARequest(RequestPack{Method: REGISTER, Max70: true, RUriUP: ue.Imsi, FromUP: ue.Imsi, CustomHeaders: hdrs}, EmptyBody())

	ss.SetState(state.BeingUnregistered)
	ss.AddMe()
	ss.SendSTMessage(trans)
}

func CallViaUE(ue *UserEquipment, cdpn string) {
	if PCSCFSocket == nil {
		system.LogError(system.LTConfiguration, "Missing PCSCF Socket")
		return
	}

	ss := NewSS(OUTBOUND)
	ss.RemoteUDP = PCSCFSocket
	ss.SIPUDPListenser = ue.UDPListener
	ss.UserEquipment = ue

	hdrs := NewSipHeaders()
	hdrs.AddHeader(P_Access_Network_Info, "IEEE-802.3") //"3GPP-E-UTRAN-FDD; utran-cell-id-3gpp=001010001000019B")
	// hdrs.AddHeader(Expires, "600000")
	hdrs.AddHeader(Supported, "path")
	hdrs.AddHeader(Contact, fmt.Sprintf(`<sip:%s@%s>;+g.3gpp.icsi-ref="urn:Aurn-7:3gpp-service.ims.icsi.mmtel";+g.3gpp.smsip;video;+sip.instance="<urn:gsma:imei:86728703-952237-0>";+g.3gpp.accesstype="wired"`, ue.Imsi, system.GetUDPAddrStringFromConn(ue.UDPListener)))

	hdrs.AddHeader(Authorization, ue.Authorization)

	trans := ss.CreateSARequest(RequestPack{Method: INVITE, Max70: true, RUriUP: cdpn, FromUP: ue.MsIsdn, CustomHeaders: hdrs}, EmptyBody())

	ss.SetState(state.BeingEstablished)
	ss.AddMe()
	ss.SendSTMessage(trans)
}

func computeAuthorizationHeader(realm, nonce, method, nonceCount string, ue *UserEquipment) string {
	uri := "sip:" + realm
	cnonce := guid.GenerateCNonce()
	ha1 := guid.Md5Hash(fmt.Sprintf("%s:%s:%s", ue.Imsi, realm, ue.Ki))
	ha2 := guid.Md5Hash(fmt.Sprintf("%s:%s", method, uri))
	response := guid.Md5Hash(fmt.Sprintf("%s:%s:%s:%s:auth:%s", ha1, nonce, nonceCount, cnonce, ha2))
	return fmt.Sprintf(`Digest username="%s", realm="%s", nonce="%s", uri="%s", response="%s", algorithm=MD5, qop=auth, nc=%s, cnonce="%s"`, ue.Imsi, realm, nonce, uri, response, nonceCount, cnonce)
}

type AuthScheme struct {
	Scheme string
	Params map[string]string
}

func ParseWWWAuthenticate(header string) []AuthScheme {
	var schemes []AuthScheme
	schemeRegex := regexp.MustCompile(`(?i)^(Basic|Bearer|Digest|NTLM|OAuth|Negotiate)\b`)
	var currentScheme *AuthScheme

	// Manually parse the header while respecting quotes
	inQuotes := false
	var partBuilder strings.Builder
	var parts []string

	for _, char := range header {
		switch char {
		case '"':
			inQuotes = !inQuotes // Toggle quote state
		case ',':
			if !inQuotes { // Only split if not inside quotes
				parts = append(parts, strings.TrimSpace(partBuilder.String()))
				partBuilder.Reset()
				continue
			}
		}
		partBuilder.WriteRune(char)
	}
	if partBuilder.Len() > 0 {
		parts = append(parts, strings.TrimSpace(partBuilder.String()))
	}

	// Process extracted parts
	for _, part := range parts {
		if match := schemeRegex.FindString(part); match != "" {
			// Start a new scheme
			if currentScheme != nil {
				schemes = append(schemes, *currentScheme)
			}
			currentScheme = &AuthScheme{Scheme: match, Params: make(map[string]string)}
			part = strings.TrimSpace(strings.TrimPrefix(part, match))
		}

		if currentScheme == nil {
			continue
		}

		// Extract key-value pairs
		paramRegex := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*(?:"([^"]+)"|([^,]+))`)
		paramMatches := paramRegex.FindAllStringSubmatch(part, -1)

		for _, pm := range paramMatches {
			key := strings.TrimSpace(pm[1])
			var value string
			if pm[2] != "" { // Quoted value
				value = pm[2]
			} else { // Unquoted value
				value = strings.TrimSpace(pm[3])
			}
			currentScheme.Params[key] = value
		}
	}

	// Append the last parsed scheme
	if currentScheme != nil {
		schemes = append(schemes, *currentScheme)
	}

	return schemes
}

func ParseWWWAuthenticateOptimized(header string) []AuthScheme {
	var schemes []AuthScheme
	schemeRegex := regexp.MustCompile(`(?i)^(Basic|Bearer|Digest|NTLM|OAuth|Negotiate)\b`)
	paramRegex := regexp.MustCompile(`([a-zA-Z0-9_-]+)\s*=\s*("[^"]+"|[^,]+)`)

	parts := splitHeader(header)

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if match := schemeRegex.FindString(part); match != "" {
			schemes = append(schemes, AuthScheme{
				Scheme: match,
				Params: extractParams(strings.TrimSpace(strings.TrimPrefix(part, match)), paramRegex),
			})
		} else if len(schemes) > 0 {
			currentScheme := &schemes[len(schemes)-1]
			for key, value := range extractParams(part, paramRegex) {
				currentScheme.Params[key] = value
			}
		}
	}

	return schemes
}

func splitHeader(header string) []string {
	var parts []string
	inQuotes := false
	var partBuilder strings.Builder

	for _, char := range header {
		switch char {
		case '"':
			inQuotes = !inQuotes
		case ',':
			if !inQuotes {
				parts = append(parts, partBuilder.String())
				partBuilder.Reset()
				continue
			}
		}
		partBuilder.WriteRune(char)
	}
	if partBuilder.Len() > 0 {
		parts = append(parts, partBuilder.String())
	}

	return parts
}

func extractParams(part string, paramRegex *regexp.Regexp) map[string]string {
	params := make(map[string]string)
	for _, pm := range paramRegex.FindAllStringSubmatch(part, -1) {
		key := strings.TrimSpace(pm[1])
		value := pm[2]
		params[key] = strings.Trim(value, `"`)
	}
	return params
}

func (ss *SipSession) logRegData(sipmsg *SipMessage) {
	ue := ss.UserEquipment

	msisdn := "N/A"
	expires := ""

	if sipmsg != nil {

		if pau := sipmsg.Headers.Value("P-Associated-URI"); pau != "" {
			rgx := regexp.MustCompile("tel:([0-9]+)")
			if mtchs := rgx.FindStringSubmatch(pau); mtchs != nil {
				msisdn = mtchs[1]
			}
		}

		if cntct := sipmsg.Headers.Value("Contact"); cntct != "" {
			rgx := regexp.MustCompile(";expires=([0-9]+)")
			if mtchs := rgx.FindStringSubmatch(cntct); mtchs != nil {
				expires = mtchs[1]
			}
		}
	}

	ue.MsIsdn = msisdn
	if expires != "" {
		ue.Expires = expires
	}
	ue.RegStatus = ss.GetState().String()

	WSServer.WriteJSON(ue)
}
