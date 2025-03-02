package sip

import (
	"fmt"
	"net"
	"sipclientgo/global"
	"sipclientgo/system"
	"sync/atomic"
)

// =================================================================================================
// Worker Pattern

var (
	WorkerCount = 3
	QueueSize   = 500
)

type Packet struct {
	sourceAddr *net.UDPAddr
	buffer     *[]byte
	bytesCount int
}

func startWorkers(conn *net.UDPConn, queue <-chan Packet) {
	global.WtGrp.Add(WorkerCount)
	atomic.AddInt32(&global.WtGrpC, int32(WorkerCount))
	for range WorkerCount {
		go worker(conn, queue)
	}
}

func udpLoopWorkers(conn *net.UDPConn, queue chan<- Packet) {
	global.WtGrp.Add(1)
	atomic.AddInt32(&global.WtGrpC, 1)
	defer func() {
		global.WtGrp.Done()
		atomic.AddInt32(&global.WtGrpC, -1)
		if r := recover(); r != nil {
			system.LogCallStack(r)
			udpLoopWorkers(conn, queue)
		}
	}()
	go func() {
		for {
			buf := global.BufferPool.Get().(*[]byte)
			n, addr, err := conn.ReadFromUDP(*buf)
			if err != nil {
				break
			}
			queue <- Packet{sourceAddr: addr, buffer: buf, bytesCount: n}
		}
	}()
}

func worker(conn *net.UDPConn, queue <-chan Packet) {
	defer global.WtGrp.Done()
	defer atomic.AddInt32(&global.WtGrpC, -1)
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
