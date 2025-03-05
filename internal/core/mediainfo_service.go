package core

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	iso "github.com/barbashov/iso639-3"
	"github.com/tidwall/pretty"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

// DefaultMediaInfoProvider implements the MediaInfoProvider interface
type DefaultMediaInfoProvider struct {
	mediaInfoPath string
	reporter      Reporter
}

// NewMediaInfoProvider creates a new DefaultMediaInfoProvider
func NewMediaInfoProvider(mediaInfoPath string, reporter Reporter) MediaInfoProvider {
	if mediaInfoPath == "" {
		mediaInfoPath = "mediainfo"
	}
	
	return &DefaultMediaInfoProvider{
		mediaInfoPath: mediaInfoPath,
		reporter:      reporter,
	}
}

// GetMediaInfo implements MediaInfoProvider interface
func (p *DefaultMediaInfoProvider) GetMediaInfo(filePath string) (MediaInfo, error) {
	var media MediaInfo
	
	if !p.isMediainfoInstalled() {
		return media, fmt.Errorf("mediainfo is not installed or not available in PATH")
	}

	// Call mediainfo to get JSON output
	output, err := p.getMediaInfoJSON(filePath)
	if err != nil {
		return media, fmt.Errorf("error calling mediainfo: %v", err)
	}
	
	// Parse the JSON output
	var rawMediaInfo RawMediaInfo
	err = json.Unmarshal(output, &rawMediaInfo)
	if err != nil {
		return media, fmt.Errorf("error parsing mediainfo JSON: %v", err)
	}
	
	// Record media info in crash reporter
	if p.reporter != nil {
		p.reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
			gs.MediaInfoVer = rawMediaInfo.CreatingLibrary.Version
			es.MediaInfoDump = string(pretty.Pretty(output))
		})
	}
	
	media.CreatingLibrary = rawMediaInfo.CreatingLibrary
	
	// Iterate through the tracks and dynamically unmarshal based on the @type field
	for _, rawTrack := range rawMediaInfo.Media.Track {
		var trackType map[string]interface{}
		// First unmarshal to get the @type field
		if err := json.Unmarshal(rawTrack, &trackType); err != nil {
			continue
		}
		
		switch trackType["@type"] {
		case "General":
			if err := json.Unmarshal(rawTrack, &media.GeneralTrack); err != nil {
				continue
			}
		case "Video":
			if err := json.Unmarshal(rawTrack, &media.VideoTrack); err != nil {
				continue
			}
		case "Audio":
			var audioTrack AudioTrack
			if err := json.Unmarshal(rawTrack, &audioTrack); err != nil {
				continue
			}
			audioTrack.Language = iso.FromAnyCode(audioTrack.LangRaw)
			media.AudioTracks = append(media.AudioTracks, audioTrack)
		}
	}
	
	return media, nil
}

// getMediaInfoJSON gets the JSON output from the mediainfo command
func (p *DefaultMediaInfoProvider) getMediaInfoJSON(filePath string) ([]byte, error) {
	cmd := exec.Command(p.mediaInfoPath, "--Output=JSON", filePath)
	return cmd.Output()
}

// isMediainfoInstalled checks if mediainfo is installed
func (p *DefaultMediaInfoProvider) isMediainfoInstalled() bool {
	cmdName := p.mediaInfoPath
	if runtime.GOOS == "windows" && !strings.HasSuffix(cmdName, ".exe") {
		cmdName = cmdName + ".exe"
	}
	_, err := exec.LookPath(cmdName)
	return err == nil
}