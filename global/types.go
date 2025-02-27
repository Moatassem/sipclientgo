package global

import (
	"fmt"
	"net"
)

type SystemError struct {
	Code    int
	Details string
}

func NewError(code int, details string) error {
	return &SystemError{Code: code, Details: details}
}

func (se *SystemError) Error() string {
	return fmt.Sprintf("Code: %d - Details: %s", se.Code, se.Details)
}

type SipUdpUserAgent struct {
	UDPAddr *net.UDPAddr
	IsAlive bool
}

func NewSipUdpUserAgent(udpAddr *net.UDPAddr) *SipUdpUserAgent {
	if udpAddr == nil {
		return nil
	}
	return &SipUdpUserAgent{UDPAddr: udpAddr}
}
