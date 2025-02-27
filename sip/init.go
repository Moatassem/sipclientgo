package sip

import (
	"fmt"
	"log"
	"net"
	"os"
	"sipclientgo/global"
	"sipclientgo/system"
)

var (
	Sessions    ConcurrentMapMutex
	UDPListener *net.UDPConn
)

func StartServer(ipv4 string, sup int, pcscf string) *net.UDPConn {
	fmt.Print("Initializing Global Parameters...")
	Sessions = NewConcurrentMapMutex()

	global.SipUdpPort = sup

	global.InitializeEngine()
	fmt.Println("Ready!")

	triedAlready := false
tryAgain:
	fmt.Print("Attempting to listen on SIP...")
	serverUDPListener, err := system.StartListening(global.ClientIPv4, global.SipUdpPort)
	if err != nil {
		if triedAlready {
			fmt.Println(err)
			os.Exit(2)
		}
		global.ClientIPv4 = getlocalIPv4(true)
		triedAlready = true
		goto tryAgain
	}

	global.PCSCFSocket, err = system.BuildUDPAddrFromSocketString(pcscf)
	if err != nil {
		fmt.Println(err)
		os.Exit(2)
	}

	MediaPorts = NewMediaPortPool()

	startWorkers(serverUDPListener)
	udpLoopWorkers(serverUDPListener)
	fmt.Println("Success: UDP", serverUDPListener.LocalAddr().String())

	fmt.Printf("Loading files in directory: %s\n", global.MediaPath)
	MRFRepos = NewMRFRepoCollection(global.MRFRepoName)
	fmt.Printf("Audio files loaded: %d\n", MRFRepos.FilesCount(global.MRFRepoName))

	UDPListener = serverUDPListener
	return serverUDPListener
}

func getlocalIPv4(getfirst bool) net.IP {
	fmt.Print("Checking Interfaces...")
	serverIPs, err := system.GetLocalIPs()
	if err != nil {
		fmt.Println("Failed to find an IPv4 interface:", err)
		os.Exit(1)
	}
	var serverIP net.IP
	if len(serverIPs) == 1 {
		serverIP = serverIPs[0]
		fmt.Println("Found:", serverIP)
	} else {
		var idx int
		for {
			fmt.Printf("Found (%d) interfaces:\n", len(serverIPs))
			for i, s := range serverIPs {
				fmt.Printf("%d- %s\n", i+1, s.String())
			}
			if getfirst {
				idx = 1
				break
			} else {
				fmt.Print("Your choice:? ")
				n, err := fmt.Scanln(&idx)
				if n == 0 {
					log.Panic("no proper interface selected")
				}
				if idx <= 0 || idx > len(serverIPs) {
					fmt.Println("Invalid interface selected")
					continue
				}
				if err == nil {
					break
				}
				fmt.Println(err)
			}
		}
		serverIP = serverIPs[idx-1]
		fmt.Println("Selected:", serverIP)
	}
	return serverIP
}

// =================================================================================================
// Worker Pattern

var (
	WorkerCount = 3
	QueueSize   = 500
	packetQueue = make(chan Packet, QueueSize)
)

type Packet struct {
	sourceAddr *net.UDPAddr
	buffer     *[]byte
	bytesCount int
}

func startWorkers(conn *net.UDPConn) {
	global.WtGrp.Add(WorkerCount)
	for range WorkerCount {
		go worker(conn, packetQueue)
	}
}

func udpLoopWorkers(conn *net.UDPConn) {
	global.WtGrp.Add(1)
	defer func() {
		global.WtGrp.Done()
		if r := recover(); r != nil {
			system.LogCallStack(r)
			udpLoopWorkers(conn)
		}
	}()
	go func() {
		for {
			buf := global.BufferPool.Get().(*[]byte)
			n, addr, err := conn.ReadFromUDP(*buf)
			if err != nil {
				fmt.Println(err)
				continue
			}
			packetQueue <- Packet{sourceAddr: addr, buffer: buf, bytesCount: n}
		}
	}()
}

func worker(conn *net.UDPConn, queue <-chan Packet) {
	defer global.WtGrp.Done()
	for packet := range queue {
		processPacket(packet, conn)
	}
}

func processPacket(packet Packet, conn *net.UDPConn) {
	pdu := (*packet.buffer)[:packet.bytesCount]
	for {
		if len(pdu) == 0 {
			break
		}
		msg, pdutmp, err := processPDU(pdu)
		if err != nil {
			fmt.Println("Bad PDU -", err)
			fmt.Println(string(pdu))
			break
		} else if msg == nil {
			break
		}
		ss, newSesType := sessionGetter(msg)
		if ss != nil {
			ss.RemoteUDP = packet.sourceAddr
			ss.SIPUDPListenser = conn
		}
		sipStack(msg, ss, newSesType)
		pdu = pdutmp
	}
	global.BufferPool.Put(packet.buffer)
}
