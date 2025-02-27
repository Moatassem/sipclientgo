package main

import (
	"fmt"
	"net"
	"os"
	"sipclientgo/global"
	"sipclientgo/sip"
	"sipclientgo/system"
)

// environment variables
//
//nolint:revive
const (
	OwnIPv4       string = "server_ipv4"
	OwnSIPUdpPort string = "sip_udp_port"

	PcscfUdpSocket string = "pcscf_udp_socket"
	ImsDomain      string = "ims_domain" //"sip:ims.mnc001.mcc001.3gppnetwork.org"
	Ki             string = "ki"
	Opc            string = "opc"
	Imsi           string = "imsi"

	//nolint:stylecheck
	OwnHttpPort    string = "http_port"
	MediaDirectory string = "media_dir"
)

func main() {
	greeting()

	conn := sip.StartServer(checkArgs())
	defer conn.Close()

	go sip.RegisterMe("")

	global.WtGrp.Wait()
}

func greeting() {
	system.LogInfo(system.LTSystem, fmt.Sprintf("Welcome to %s - Product of %s 2025", global.B2BUAName, system.ASCIIPascal(global.EntityName)))
}

func checkArgs() (string, int, string) {
	ipv4, ok := os.LookupEnv(OwnIPv4)
	if !ok {
		system.LogWarning(system.LTConfiguration, "No self IPv4 address provided - First available shall be used")
	}

	global.ClientIPv4 = net.ParseIP(ipv4)

	var sipuport int

	sup, ok := os.LookupEnv(OwnSIPUdpPort)
	if !ok {
		system.LogWarning(system.LTConfiguration, fmt.Sprintf("No self SIP UDP port provided - [%d] shall be used", global.DefaultSipPort))
		sipuport = global.DefaultSipPort
	} else {
		minS := 5000
		maxS := 6000
		sipuport, ok = system.Str2IntDefaultMinMax(sup, global.DefaultSipPort, minS, maxS)
		if !ok {
			system.LogWarning(system.LTConfiguration, "Invalid SIP UDP port: "+sup)
		}
	}

	pcscfSocket, ok := os.LookupEnv(PcscfUdpSocket)
	if !ok {
		system.LogError(system.LTConfiguration, "No P-CSCF Provided - Exiting")
		os.Exit(2)
	}

	imsDomain, ok := os.LookupEnv(ImsDomain)
	if !ok {
		system.LogError(system.LTConfiguration, "No IMS Domain Provided - Exiting")
		os.Exit(2)
	}
	global.ImsDomain = imsDomain

	ki, ok := os.LookupEnv(Ki)
	if !ok {
		system.LogError(system.LTConfiguration, "No Ki Provided - Exiting")
		os.Exit(2)
	}
	global.Ki = ki

	opc, ok := os.LookupEnv(Opc)
	if !ok {
		system.LogError(system.LTConfiguration, "No OPC Provided - Exiting")
		os.Exit(2)
	}
	global.Opc = opc

	imsi, ok := os.LookupEnv(Imsi)
	if !ok {
		system.LogError(system.LTConfiguration, "No IMSI Provided - Exiting")
		os.Exit(2)
	}
	global.Imsi = imsi

	mp, ok := os.LookupEnv(MediaDirectory)
	if ok {
		global.MediaPath = mp
	} else {
		global.MediaPath = "./audio"
		system.LogWarning(system.LTConfiguration, fmt.Sprintf("No media directory provided - [%s] shall be used", global.MediaPath))
	}

	return ipv4, sipuport, pcscfSocket
}
