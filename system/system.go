package system

import (
	"fmt"
	"log"
	"runtime"
)

const (
	DeltaRune rune = 'a' - 'A'
)

var (
	logtitles = [...]string{"All", "AnswerMachine", "BadSIPMessage", "ChatMessage", "ConfigFiles", "Configuration", "Connectivity", "ContactCenter", "CustomCommand", "CustomCommandResult", "DTMF", "EmailNotification", "ExternalData", "FileUpload", "Webserver", "IPCollection", "License", "LogInOut", "MediaCapability", "MediaStack", "NAT", "PESQScore", "ResourceLimitation", "RTDGrabber", "Security", "SDPStack", "SIPStack", "SNMP", "StirShaken", "StressTester", "System", "TLSStack", "TTS", "UnhandledCritical", "Unspecified", "WebSocketData", "None"}
	loglevels = [...]string{"Information", "Warning", "Error"}
)

// ==============================================================
// ==============================================================
type LogLevel int

const (
	LLInformation LogLevel = iota
	LLWarning
	LLError
)

func (ll LogLevel) String() string {
	return loglevels[ll]
}

type LogTitle int

const (
	LTAll LogTitle = iota
	LTAnswerMachine
	LTBadSIPMessage
	LTChatMessage
	LTConfigFiles
	LTConfiguration
	LTConnectivity
	LTContactCenter
	LTCustomCommand
	LTCustomCommandResult
	LTDTMF
	LTEmailNotification
	LTExternalData
	LTFileUpload
	LTWebserver
	LTIPCollection
	LTLicense
	LTLogInOut
	LTMediaCapability
	LTMediaStack
	LTNAT
	LTPESQScore
	LTResourceLimitation
	LTRTDGrabber
	LTSecurity
	LTSDPStack
	LTSIPStack
	LTSNMP
	LTStirShaken
	LTStressTester
	LTSystem
	LTTLSStack
	LTTTS
	LTUnhandledCritical
	LTUnspecified
	LTWebSocketData
	LTNone
)

func (lt LogTitle) String() string {
	return logtitles[lt]
}

// ==============================================================

func LogCallStack(r any) {
	fmt.Printf("Panic Recovered! Encountered Error:\n%v\n", r)
	buf := make([]byte, 1024)
	n := runtime.Stack(buf, false)
	fmt.Printf("Stack trace:\n%s\n", buf[:n])
}

//===================================================================

func LogInfo(lt LogTitle, msg string) {
	LogHandler(LLInformation, lt, msg)
}

func LogWarning(lt LogTitle, msg string) {
	LogHandler(LLWarning, lt, msg)
}

func LogError(lt LogTitle, msg string) {
	LogHandler(LLError, lt, msg)
}

func LogHandler(ll LogLevel, lt LogTitle, msg string) {
	log.Printf("\t%v\t%v\t%s\n", ll.String(), lt.String(), msg)
}
