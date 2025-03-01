package sip

import (
	"fmt"
	"sipclientgo/global"
)

var (
	Sessions ConcurrentMapMutex
)

func StartServer() {
	fmt.Print("Initializing Global Parameters...")
	Sessions = NewConcurrentMapMutex()

	global.InitializeEngine()
	fmt.Println("Ready!")

	MediaPorts = NewMediaPortPool()

	fmt.Printf("Loading files in directory: %s\n", global.MediaPath)
	MRFRepos = NewMRFRepoCollection(global.MRFRepoName)
	fmt.Printf("Audio files loaded: %d\n", MRFRepos.FilesCount(global.MRFRepoName))
}
