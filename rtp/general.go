package rtp

const (
	PCMU uint8 = 0
	PCMA uint8 = 8
	G722 uint8 = 9
)

var codecSilence = map[uint8]byte{PCMU: 255, PCMA: 213, G722: 85}

func GetSilence(pt uint8) byte {
	switch pt {
	case PCMU:
		return codecSilence[pt]
	case PCMA:
		return codecSilence[pt]
	case G722:
		return codecSilence[pt]
	default:
		return 0
	}
}

func DecodeToPCM(frame []byte, pt uint8) []int16 {
	switch pt {
	case PCMU:
		return G711U2PCM(frame)
	case PCMA:
		return G711A2PCM(frame)
	case G722:
		return G722toPCM(frame)
	default:
		return nil
	}
}

func EncodePCM(pcm []int16, pt uint8) []byte {
	switch pt {
	case PCMU:
		return PCM2G711U(pcm)
	case PCMA:
		return PCM2G711A(pcm)
	case G722:
		return PCM2G722(pcm)
	default:
		return nil
	}
}

func TxPCMnSilence(pcm []int16, pt byte) ([]byte, byte) {
	switch pt {
	case PCMU:
		return PCM2G711U(pcm), codecSilence[pt]
	case PCMA:
		return PCM2G711A(pcm), codecSilence[pt]
	case G722:
		return PCM2G722(pcm), codecSilence[pt]
	default:
		return nil, 0
	}
}
