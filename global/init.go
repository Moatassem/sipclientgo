package global

import (
	"encoding/binary"
	"sipclientgo/dtmf"
	"sipclientgo/rtp"
	"sync"
)

func InitializeEngine() {
	responsesHeadersInit()

	BufferPool = newSyncPool(BufferSize, BufferSize)

	rtpsz := RTPHeaderSize + RTPPayloadSize
	RTPRXBufferPool = newSyncPool(rtpsz, rtpsz)
	RTPTXBufferPool = newSyncPool(0, rtpsz)

	IsSystemBigEndian = checkSystemIndian()
	rtp.InitializeTX()
	dtmf.Initialize(SamplingRate)
}

func newSyncPool(bsz, csz int) *sync.Pool {
	return &sync.Pool{
		New: func() any {
			lst := make([]byte, bsz, csz)
			return &lst
		},
	}
}

func checkSystemIndian() bool {
	var i int32 = 0x01020304
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, uint32(i))
	return i == int32(binary.BigEndian.Uint32(buf))
}
