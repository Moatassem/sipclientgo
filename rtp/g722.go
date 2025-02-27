package rtp

import (
	"sipclientgo/system"

	"github.com/gotranspile/g722"
	// "github.com/gotranspile/g722"
	// "github.com/xlab/opus-go/opus"
)

var TranscodingEngine TXEngine

type TXEngine struct {
	G722Encoder *g722.Encoder
	G722Decoder *g722.Decoder
}

func G722toPCM(frame []byte) []int16 {
	count := len(frame)
	if len(frame) == 0 {
		return nil
	}
	res := make([]int16, count)
	n := TranscodingEngine.G722Decoder.Decode(res, frame)
	if n == 0 {
		system.LogWarning(system.LTMediaCapability, "Failed to encode G.722 data")
		return nil
	}
	return res

}

func PCM2G722(pcm []int16) []byte {
	g722 := make([]byte, len(pcm))
	n := TranscodingEngine.G722Encoder.Encode(g722, pcm)
	if n == 0 {
		system.LogWarning(system.LTMediaCapability, "Failed to encode G.722 data")
		return nil
	}
	return g722
}

func InitializeTX() {
	// Create a G.722 encoder
	g722rate := 64000
	var g722flag g722.Flags = g722.FlagSampleRate8000
	TranscodingEngine.G722Encoder = g722.NewEncoder(g722rate, g722flag)

	// Create a G.722 decoder
	TranscodingEngine.G722Decoder = g722.NewDecoder(g722rate, g722flag)
}
