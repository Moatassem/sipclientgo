package sip

import (
	"fmt"
	"os"
	"path/filepath"
	"sipclientgo/global"
	"sipclientgo/rtp"
	"sipclientgo/system"
	"strings"
	"sync"
	"time"
)

const (
	ExtRaw string = "raw"
	ExtWav string = "wav"
	ExtMp3 string = "mp3"
)

var MRFRepos *MRFRepoCollection

type MRFRepo struct {
	name    string
	mu      sync.RWMutex
	pcmdata map[string][]int16
	txdata  map[string]map[uint8][]byte
}

type MRFRepoCollection struct {
	mu    sync.RWMutex
	repos map[string]*MRFRepo
}

func NewMRFRepoCollection(rn string) *MRFRepoCollection {
	var ivrs MRFRepoCollection
	ivrs.repos = loadMedia(rn)
	return &ivrs
}

func dropExtension(fn string) string {
	idx := strings.LastIndex(fn, ".")
	if idx == -1 {
		return fn
	}
	return fn[:idx]
}

func getExtension(fn string) string {
	idx := strings.LastIndex(fn, ".")
	if idx == -1 {
		return "<no extension>"
	}
	return system.ASCIIToLower(fn[idx+1:])
}

func loadMedia(rn string) map[string]*MRFRepo {
	mrfrepos := make(map[string]*MRFRepo)
	mrfrepo := MRFRepo{name: rn, pcmdata: make(map[string][]int16), txdata: make(map[string]map[uint8][]byte)}
	mrfrepos[rn] = &mrfrepo

	dentries, err := os.ReadDir(global.MediaPath)
	if err != nil {
		panic(err)
	}
	for _, dentry := range dentries {
		if dentry.IsDir() {
			continue
		}
		filename := dentry.Name()
		fullpath := filepath.Join(global.MediaPath, filename)

		var pcmBytes []int16
		var err error
		var rawpath string

		filenameonly := dropExtension(filename)
		var comment string

		switch ext := getExtension(filename); ext {
		case ExtRaw:
			pcmBytes, err = rtp.ReadPCMRaw(fullpath)
		case ExtWav, ExtMp3:
			rawpath, err = rtp.RunSox(global.SoxPath, global.MediaPath, filename, filenameonly)
			if err == nil {
				pcmBytes, err = rtp.ReadPCMRaw(rawpath)
				comment = " (converted and deleted)"
			}
		default:
			fmt.Printf("Filename: %s - Unsupported Extension: %s - Skipped\n", filename, ext)
			continue
		}

		if err != nil {
			fmt.Println(err)
			continue
		}

		// Calculate duration -- TODO duration not accurate vs playback duration
		duration := float64(len(pcmBytes)) / global.PcmSamplingRate

		fmt.Printf("Filename: %s%s, Duration: %s\n", filename, comment, formattedTime(duration))

		mrfrepo.pcmdata[filenameonly] = pcmBytes
		mrfrepo.txdata[filenameonly] = make(map[uint8][]byte)
	}

	return mrfrepos
}

func formattedTime(totsec float64) string {
	duration := time.Duration(totsec * float64(time.Second))

	minutes := int(duration.Minutes())
	seconds := int(duration.Seconds()) % 60
	milliseconds := int(duration.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d.%03d", minutes, seconds, milliseconds)
}

func (mrfr *MRFRepoCollection) FilesCount(up string) int {
	mrfr.mu.RLock()
	defer mrfr.mu.RUnlock()
	mp, ok := mrfr.repos[up]
	if ok {
		return mp.FilesCount()
	}
	return -1
}

func (mrfrps *MRFRepoCollection) GetMRFRepo(upart string) (*MRFRepo, bool) {
	mrfrps.mu.RLock()
	defer mrfrps.mu.RUnlock()
	mrfrp, ok := mrfrps.repos[upart]
	return mrfrp, ok
}

func (mrfrps *MRFRepoCollection) AudioFileExists(upart, key string) bool {
	mrfrps.mu.RLock()
	defer mrfrps.mu.RUnlock()
	if mp, ok := mrfrps.repos[upart]; ok {
		return mp.AudioFileExists(key)
	}
	return false
}

func (mrfrp *MRFRepo) AudioFileExists(key string) bool {
	mrfrp.mu.RLock()
	defer mrfrp.mu.RUnlock()
	if pcm, ok := mrfrp.pcmdata[key]; ok {
		if len(pcm) == 0 {
			return false
		}
		return ok
	}
	return false
}

func (mrfrp *MRFRepo) GetTx(key string, codec uint8) ([]byte, byte, bool) {
	mrfrp.mu.Lock()
	defer mrfrp.mu.Unlock()
	silence := rtp.GetSilence(codec)
	txdata, ok := mrfrp.txdata[key]
	if !ok {
		return nil, 0, false
	}
	txbytes, ok := txdata[codec]
	if !ok {
		txbytes = rtp.EncodePCM(mrfrp.pcmdata[key], codec)
		txdata[codec] = txbytes
	}
	return txbytes, silence, true
}

func (mrfrp *MRFRepo) FilesCount() int {
	mrfrp.mu.RLock()
	defer mrfrp.mu.RUnlock()
	return len(mrfrp.pcmdata)
}

// func (mrfr *MRFRepoCollection) AddOrUpdate(upart, key string, bytes []int16) {
// 	mrfr.mu.Lock()
// 	defer mrfr.mu.Unlock()
// 	mrfr.repos[upart][key] = bytes
// }
