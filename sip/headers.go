package sip

import (
	"fmt"
	"sipclientgo/global"
	"sipclientgo/system"
	"strings"
)

type SipHeaders struct {
	_map map[string][]string
}

func NewSipHeaders() SipHeaders {
	return SipHeaders{_map: make(map[string][]string)}
}

func NewSHsFromMap(mp map[string][]string) SipHeaders {
	headers := NewSipHeaders()
	for k, v := range mp {
		headers.AddHeaderValues(k, v)
	}
	return headers
}

// Used mainly in outbound messages or when pointer is needed i.e. mutable
func NewSHsPointer(setDefaults bool) *SipHeaders {
	headers := NewSipHeaders()
	if setDefaults {
		headers.AddHeader(global.User_Agent, global.B2BUAName)
		headers.AddHeader(global.Server, global.B2BUAName)
		headers.AddHeader(global.Allow, global.AllowedMethods)
	}
	return &headers
}

func NewSHQ850OrSIP(Q850OrSIP int, Details string, retryAfter string) SipHeaders {
	headers := NewSipHeaders()
	if retryAfter != "" {
		headers.AddHeader(global.Retry_After, retryAfter)
	}
	if Q850OrSIP == 0 {
		if strings.TrimSpace(Details) != "" {
			headers.AddHeader(global.Warning, fmt.Sprintf("399 mrfgo \"%s\"", Details))
		}
	} else {
		var reason string
		if Q850OrSIP <= 127 {
			reason = fmt.Sprintf("Q.850;cause=%d", Q850OrSIP)
		} else {
			reason = fmt.Sprintf("SIP;cause=%d", Q850OrSIP)
		}
		if strings.TrimSpace(Details) != "" {
			reason += fmt.Sprintf(";text=\"%s\"", Details)
		}
		headers.AddHeader(global.Reason, reason)
	}
	return headers
}

// ==========================================

func (headers SipHeaders) InternalMap() map[string][]string {
	if headers._map == nil {
		return nil
	}
	return headers._map
}

// returns headers as lowercase
func (headers *SipHeaders) GetHeaderNames() []string {
	var lst []string
	for h := range headers._map {
		lst = append(lst, h)
	}
	return lst
}

// headerName is case insensitive
func (headers *SipHeaders) HeaderExists(headerName string) bool {
	headerName = system.ASCIIToLower(headerName)
	_, ok := headers._map[headerName]
	return ok
}

func (headers *SipHeaders) HeaderCount(headerName string) int {
	headerName = system.ASCIIToLower(headerName)
	v, ok := headers._map[headerName]
	if ok {
		return len(v)
	}
	return 0
}

func (headers *SipHeaders) DoesValueExistInHeader(headerName string, headerValue string) bool {
	headerValue = system.ASCIIToLower(headerValue)
	_, values := headers.Values(headerName)
	for _, hv := range values {
		if strings.Contains(system.ASCIIToLower(hv), headerValue) {
			return true
		}
	}
	return false
}

func (headers *SipHeaders) AddHeader(header global.HeaderEnum, headerValue string) {
	headers.Add(header.String(), headerValue)
}

func (headers *SipHeaders) Add(headerName string, headerValue string) {
	headerName = system.ASCIIToLower(headerName)
	v, ok := headers._map[headerName]
	if ok {
		headers._map[headerName] = append(v, headerValue)
	} else {
		headers._map[headerName] = []string{headerValue}
	}
}

func (headers *SipHeaders) AddHeaderValues(headerName string, headerValues []string) {
	headerName = system.ASCIIToLower(headerName)
	v, ok := headers._map[headerName]
	if ok {
		headers._map[headerName] = append(v, headerValues...)
	} else {
		headers._map[headerName] = headerValues
	}
}

func (headers *SipHeaders) SetHeader(header global.HeaderEnum, headerValue string) {
	headers.Set(header.String(), headerValue)
}

func (headers *SipHeaders) Set(headerName string, headerValue string) {
	headerName = system.ASCIIToLower(headerName)
	headers._map[headerName] = []string{headerValue}
}

func (headers *SipHeaders) ValuesHeader(header global.HeaderEnum) (bool, []string) {
	return headers.Values(header.String())
}

func (headers *SipHeaders) Values(headerName string) (bool, []string) {
	headerName = system.ASCIIToLower(headerName)
	v, ok := headers._map[headerName]
	if ok {
		return true, v
	} else {
		return false, nil
	}
}

// returns headers with proper case - 'exceptHeaders' MUST be lower case headers!
func (headers *SipHeaders) ValuesWithHeaderPrefix(headersPrefix string, exceptHeaders ...string) map[string][]string {
	headersPrefix = system.ASCIIToLower(headersPrefix)
	data := make(map[string][]string)
outer:
	for k, v := range headers._map {
		if strings.HasPrefix(k, headersPrefix) {
			for _, eh := range exceptHeaders {
				if eh == k {
					continue outer
				}
			}
			data[global.HeaderCase(k)] = v
		}
	}
	return data
}

func (headers *SipHeaders) DeleteHeadersWithPrefix(headersPrefix string) {
	headersPrefix = system.ASCIIToLower(headersPrefix)
	var hdrs []string
	for ky := range headers._map {
		if strings.HasPrefix(ky, headersPrefix) {
			hdrs = append(hdrs, ky)
		}
	}
	for _, hdr := range hdrs {
		delete(headers._map, hdr)
	}
}

func (headers *SipHeaders) ValueHeader(header global.HeaderEnum) string {
	return headers.Value(header.String())
}

func (headers *SipHeaders) Value(headerName string) string {
	if ok, v := headers.Values(headerName); ok {
		return v[0]
	}
	return ""
}

func (headers *SipHeaders) Delete(headerName string) bool {
	headerName = system.ASCIIToLower(headerName)
	_, ok := headers._map[headerName]
	if ok {
		delete(headers._map, headerName)
	}
	return ok
}

func (headers *SipHeaders) ContainsToTag() bool {
	toheader := headers._map["to"]
	return strings.Contains(system.ASCIIToLower(toheader[0]), "tag")
}

func (headers *SipHeaders) AnyMandatoryHeadersMissing(m global.Method) (bool, string) {
	for _, mh := range global.MandatoryHeaders {
		if !headers.HeaderExists(mh) {
			return true, mh
		}
	}
	if m == global.INVITE {
		mh := global.Max_Forwards.String()
		if !headers.HeaderExists(mh) {
			return true, mh
		}
		mh = global.Contact.String()
		if !headers.HeaderExists(mh) {
			return true, mh
		}
	}
	return false, ""
}
