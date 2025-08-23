package sip

import (
	"fmt"
	"sipclientgo/global"

	"github.com/Moatassem/sdp"
)

const (
	SipPort int = 5060
)

var (
	SupportedCodecs = []uint8{sdp.PCMA, sdp.PCMU, sdp.G722}
)

func StartServer() {
	fmt.Print("Initializing Global Parameters...")
	global.InitializeEngine()
	fmt.Println("Ready!")

	MediaPorts = NewMediaPortPool()

	fmt.Printf("Loading files in directory: %s\n", global.MediaPath)
	MRFRepos = NewMRFRepoCollection(global.MRFRepoName)
	fmt.Printf("Audio files loaded: %d\n", MRFRepos.FilesCount(global.MRFRepoName))
}
