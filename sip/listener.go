package sip

import (
	"fmt"
	"net"
	"sipclientgo/global"
	"sipclientgo/system"
)

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
