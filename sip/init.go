package sip

import (
	"fmt"
	"sipclientgo/global"
)

const (
	SipPort int = 5060
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
