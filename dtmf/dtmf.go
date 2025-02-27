package dtmf

import (
	"math"
)

const (
	// blockSize  = 480  // 3 RTP packets i.e. 160 x 3 = 480 bytes // Block size for DTMF detection (2 RTP packets = 320 samples)
	threshold = 1e11 // Power threshold for DTMF detection 1e5
)

var (
	dtmfFrequencies = []float64{697, 770, 852, 941, 1209, 1336, 1477, 1633}
	dtmfMap         = [][]string{
		{"1", "2", "3", "A"},
		{"4", "5", "6", "B"},
		{"7", "8", "9", "C"},
		{"*", "0", "#", "D"},
	}
	coefficients []float64 // Precomputed coefficients for Goertzel algorithm
)

func Initialize(sr float64) {
	// Precompute coefficients for each DTMF frequency
	coefficients = make([]float64, len(dtmfFrequencies))
	for i, freq := range dtmfFrequencies {
		normalizedFreq := freq / sr
		coefficients[i] = 2.0 * math.Cos(2.0*math.Pi*normalizedFreq)
	}
}

func goertzel(samples []int16, coeff float64) float64 {
	var sPrev, sPrev2 float64

	for _, sample := range samples {
		s := float64(sample) + coeff*sPrev - sPrev2
		sPrev2 = sPrev
		sPrev = s
	}

	return sPrev2*sPrev2 + sPrev*sPrev - coeff*sPrev*sPrev2
}

func DetectDTMF(samples []int16) string {
	power := make([]float64, len(dtmfFrequencies))

	// Calculate power for each DTMF frequency
	for i, coeff := range coefficients {
		power[i] = goertzel(samples, coeff)
	}

	// Find the strongest row and column frequencies
	rowPower := power[:4]
	colPower := power[4:]

	rowMaxIndex := maxIndex(rowPower)
	colMaxIndex := maxIndex(colPower)

	// fmt.Printf("Threshold Power:\t\t\t\t%.2f\n", threshold)
	// fmt.Printf("Horizontal DTMF Frequency: %.0f with Power:\t%.2f\n", dtmfFrequencies[rowMaxIndex], rowPower[rowMaxIndex])
	// fmt.Printf("Vertical DTMF Frequency: %.0f with Power:\t%.2f\n", dtmfFrequencies[colMaxIndex+4], colPower[colMaxIndex])

	// Check if the detected power exceeds the threshold
	if rowPower[rowMaxIndex] < threshold || colPower[colMaxIndex] < threshold {
		return "" // No DTMF detected
	}

	return dtmfMap[rowMaxIndex][colMaxIndex]
}

func maxIndex(power []float64) int {
	maxIndex := 0
	maxPower := power[0]

	for i, p := range power {
		if p > maxPower {
			maxPower = p
			maxIndex = i
		}
	}

	return maxIndex
}

// TODO create DTMF Generator using the below

// func main() {
// 	// Example PCM data (replace with actual PCM data)
// 	pcmData := make([]int16, blockSize)
// 	for i := range pcmData {
// 		// Simulate a DTMF tone (e.g., "1" = 697 Hz + 1209 Hz)
// 		pcmData[i] = int16(32767 * (math.Sin(2*math.Pi*697*float64(i)/sampleRate) + math.Sin(2*math.Pi*1209*float64(i)/sampleRate)))
// 	}

// 	dtmf := detectDTMF(pcmData)
// 	if dtmf != "" {
// 		fmt.Printf("Detected DTMF: %s\n", dtmf)
// 	} else {
// 		fmt.Println("No DTMF detected")
// 	}
// }
