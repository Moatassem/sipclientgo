package sdp

type Codec struct {
	Payload  uint8
	Name     string
	Channels int
	Bitrate  int
}

var (
	SupportedCodecs = []uint8{PCMA, PCMU, G722, G729}
)

const (
	TelephoneEvents = "telephone-event"
)

const (
	PCMU uint8 = 0
	GSM        = 3
	G723       = 4
	DVI4       = 5
	// DVI4   = 6
	LPC   = 7
	PCMA  = 8
	G722  = 9
	L16   = 11
	QCELP = 12
	CN    = 13
	MPA   = 14
	G728  = 15
	// DVI4   = 16
	// DVI4   = 17
	G729            = 18
	telephone_event = 101

//	0,PCMU,8000,1
//
// 3,GSM,8000,1
// 4,G723,8000,1
// 5,DVI4,8000,1
// 6,DVI4,16000,1
// 7,LPC,8000,1
// 8,PCMA,8000,1
// 9,G722,8000,1
// 11,L16,44100,1
// 12,QCELP,8000,1
// 13,CN,8000,1
// 14,MPA,90000
// 15,G728,8000,1
// 16,DVI4,11025,1
// 17,DVI4,22050,1
// 18,G729,8000,1
)
