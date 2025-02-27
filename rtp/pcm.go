package rtp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func RunSox(soxpath, parentDir, filenamewithExt, filenameonly string) (string, error) {
	var filename, rawfilename string
	var err error
	filename, err = filepath.Abs(filepath.Join(parentDir, filenamewithExt))
	if err != nil {
		return "", err
	}

	rawfilename, err = filepath.Abs(filepath.Join(parentDir, fmt.Sprintf("%s.raw", filenameonly)))
	if err != nil {
		return "", err
	}

	soxcmd := filepath.Join(soxpath, "sox")
	cmd := exec.Command(soxcmd, "--clobber", "--no-glob",
		filename,
		"-e", "signed-integer",
		"-b", "16",
		"-c", "1",
		"-r", "16000",
		rawfilename,
		"speed", "2")

	// Redirect output to console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Run the command
	err = cmd.Run()
	if err == nil {
		_ = os.Remove(filename)
	}
	return rawfilename, err
}

func ReadPCMRaw(filename string) ([]int16, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		fmt.Println("Error opening file:", err)
		return nil, err
	}
	pcmData := bytesToInt16s(file)
	return pcmData, nil
}

// bytesToInt16s converts a byte slice into a slice of int16 samples
func bytesToInt16s(data []byte) []int16 {
	int16Data := make([]int16, len(data)/2)
	err := binary.Read(bytes.NewReader(data), binary.LittleEndian, &int16Data)
	if err != nil {
		fmt.Println("Error converting to int16:", err)
	}
	return int16Data
}

//===================================================================

func Int16stoBytes(pcmData []int16) []byte {
	byteData := make([]byte, 2*len(pcmData))
	for i, sample := range pcmData {
		byteData[i*2] = byte(sample)
		byteData[i*2+1] = byte(sample >> 8)
	}
	return byteData
}

func Int16stoBytes2(pcmData []int16) []byte {
	byteBuffer := new(bytes.Buffer)
	err := binary.Write(byteBuffer, binary.LittleEndian, pcmData)
	if err != nil {
		fmt.Println("Error converting PCM data:", err)
	}
	return byteBuffer.Bytes()
}
