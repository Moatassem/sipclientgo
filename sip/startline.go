package sip

import (
	"fmt"
	"sipclientgo/global"
	"sipclientgo/system"
)

// -------------------------------------------

type SipStartLine struct {
	global.Method
	UriScheme      string
	UserPart       string
	HostPart       string
	UserParameters *map[string]string
	Password       string

	StatusCode   int
	ReasonPhrase string

	RUri string

	UriParameters *map[string]string
	UriHeaders    string
}

func (ssl *SipStartLine) BuildRURI() {
	if ssl.UserPart == "" {
		ssl.RUri = fmt.Sprintf("%s:%s%s%s", ssl.UriScheme, ssl.HostPart, system.GenerateParameters(ssl.UriParameters), ssl.UriHeaders)
		return
	}
	ssl.RUri = fmt.Sprintf("%s:%s%s%s@%s%s%s", ssl.UriScheme, ssl.UserPart, system.GenerateParameters(ssl.UserParameters), ssl.Password, ssl.HostPart, system.GenerateParameters(ssl.UriParameters), ssl.UriHeaders)
}

func (ssl *SipStartLine) GetStartLine(mt global.MessageType) string {
	if mt == global.REQUEST {
		return fmt.Sprintf("%s %s %s\r\n", ssl.Method.String(), ssl.RUri, global.SipVersion)
	}
	return fmt.Sprintf("%s %d %s\r\n", global.SipVersion, ssl.StatusCode, ssl.ReasonPhrase)
}

type RequestPack struct {
	global.Method
	RUriUP        string
	FromUP        string
	Max70         bool
	CustomHeaders SipHeaders
	IsProbing     bool
}

type ResponsePack struct {
	StatusCode    int
	ReasonPhrase  string
	ContactHeader string

	CustomHeaders SipHeaders

	LinkedPRACKST  *Transaction
	PRACKRequested bool
}

func NewResponsePackRFWarning(stc int, rsnphrs, warning string) ResponsePack {
	return ResponsePack{
		StatusCode:    stc,
		ReasonPhrase:  rsnphrs,
		CustomHeaders: NewSHQ850OrSIP(0, warning, ""),
	}
}

// reason != "" ==> Warning & Reason headers are always created.
//
// reason == "" ==>
//
// stc == 0 ==> only Warning header
//
// stc != 0 ==> only Reason header
func NewResponsePackSRW(sipc int, warning string, reason string) ResponsePack {
	var hdrs SipHeaders
	if reason == "" {
		hdrs = NewSHQ850OrSIP(sipc, warning, "")
	} else {
		hdrs = NewSHQ850OrSIP(0, warning, "")
		hdrs.SetHeader(global.Reason, reason)
	}
	return ResponsePack{
		StatusCode:    sipc,
		CustomHeaders: hdrs,
	}
}

func NewResponsePackSIPQ850Details(sipc, q850c int, details string) ResponsePack {
	hdrs := NewSHQ850OrSIP(q850c, details, "")
	return ResponsePack{
		StatusCode:    sipc,
		CustomHeaders: hdrs,
	}
}

func NewResponsePackWarning(sipc int, warning string) ResponsePack {
	hdrs := NewSHQ850OrSIP(0, warning, "")
	return ResponsePack{
		StatusCode:    sipc,
		CustomHeaders: hdrs,
	}
}
