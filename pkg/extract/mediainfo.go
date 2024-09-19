package extract

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
)

// CreatingLibrary represents the information about the library used to create the media information
type CreatingLibrary struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// GeneralTrack represents the general information about the media file
type GeneralTrack struct {
	Type                 string `json:"@type"`
	VideoCount           string `json:"VideoCount"`
	AudioCount           string `json:"AudioCount"`
	FileExtension        string `json:"FileExtension"`
	Format               string `json:"Format"`
	FormatProfile        string `json:"Format_Profile"`
	CodecID              string `json:"CodecID"`
	CodecIDCompatible    string `json:"CodecID_Compatible"`
	FileSize             string `json:"FileSize"`
	Duration             string `json:"Duration"`
	OverallBitRate       string `json:"OverallBitRate"`
	FrameRate            string `json:"FrameRate"`
	FrameCount           string `json:"FrameCount"`
	StreamSize           string `json:"StreamSize"`
	HeaderSize           string `json:"HeaderSize"`
	DataSize             string `json:"DataSize"`
	FooterSize           string `json:"FooterSize"`
	IsStreamable         string `json:"IsStreamable"`
	Title                string `json:"Title"`
	Collection           string `json:"Collection"`
	Season               string `json:"Season"`
	Track                string `json:"Track"`
	Description          string `json:"Description"`
	RecordedDate         string `json:"Recorded_Date"`
	FileModifiedDate     string `json:"File_Modified_Date"`
	FileModifiedDateLocal string `json:"File_Modified_Date_Local"`
	EncodedApplication   string `json:"Encoded_Application"`
	Cover                string `json:"Cover"`
	Extra                struct {
		PartID string `json:"Part_ID"`
	} `json:"extra"`
}

// VideoTrack represents the information about the video stream in the media file
type VideoTrack struct {
	Type                 string `json:"@type"`
	StreamOrder          string `json:"StreamOrder"`
	ID                   string `json:"ID"`
	Format               string `json:"Format"`
	FormatProfile        string `json:"Format_Profile"`
	FormatLevel          string `json:"Format_Level"`
	CodecID              string `json:"CodecID"`
	Duration             string `json:"Duration"`
	BitRate              string `json:"BitRate"`
	Width                string `json:"Width"`
	Height               string `json:"Height"`
	SampledWidth         string `json:"Sampled_Width"`
	SampledHeight        string `json:"Sampled_Height"`
	PixelAspectRatio     string `json:"PixelAspectRatio"`
	DisplayAspectRatio   string `json:"DisplayAspectRatio"`
	Rotation             string `json:"Rotation"`
	FrameRateMode        string `json:"FrameRate_Mode"`
	FrameRate            string `json:"FrameRate"`
	FrameRateMinimum     string `json:"FrameRate_Minimum"`
	FrameRateMaximum     string `json:"FrameRate_Maximum"`
	FrameCount           string `json:"FrameCount"`
	ColorSpace           string `json:"ColorSpace"`
	ChromaSubsampling    string `json:"ChromaSubsampling"`
	BitDepth             string `json:"BitDepth"`
	StreamSize           string `json:"StreamSize"`
	Title                string `json:"Title"`
	ColourRange          string `json:"colour_range"`
	ColourRangeSource    string `json:"colour_range_Source"`
	Extra                struct {
		CodecConfigurationBox string `json:"CodecConfigurationBox"`
	} `json:"extra"`
}

// AudioTrack represents the information about the audio stream in the media file
type AudioTrack struct {
	Type                   string `json:"@type"`
	StreamOrder            string `json:"StreamOrder"`
	ID                     string `json:"ID"`
	Format                 string `json:"Format"`
	FormatCommercialIfAny  string `json:"Format_Commercial_IfAny"`
	FormatSettingsSBR      string `json:"Format_Settings_SBR"`
	FormatAdditionalFeatures string `json:"Format_AdditionalFeatures"`
	CodecID                string `json:"CodecID"`
	Duration               string `json:"Duration"`
	BitRateMode            string `json:"BitRate_Mode"`
	BitRate                string `json:"BitRate"`
	Channels               string `json:"Channels"`
	ChannelPositions       string `json:"ChannelPositions"`
	ChannelLayout          string `json:"ChannelLayout"`
	SamplesPerFrame        string `json:"SamplesPerFrame"`
	SamplingRate           string `json:"SamplingRate"`
	SamplingCount          string `json:"SamplingCount"`
	FrameRate              string `json:"FrameRate"`
	FrameCount             string `json:"FrameCount"`
	CompressionMode        string `json:"Compression_Mode"`
	StreamSize             string `json:"StreamSize"`
	StreamSizeProportion   string `json:"StreamSize_Proportion"`
	Title                  string `json:"Title"`
	Language               string `json:"Language"`
	Default                string `json:"Default"`
	AlternateGroup         string `json:"AlternateGroup"`
}

// MediaInfo represents the media information including general, video, and audio tracks
type MediaInfo struct {
	CreatingLibrary CreatingLibrary
	GeneralTrack GeneralTrack
	VideoTrack   VideoTrack
	AudioTracks  []AudioTrack
}

type RawMedia struct {
	Ref   string            `json:"@ref"`
	Track []json.RawMessage `json:"track"`
}

type RawMediaInfo struct {
	CreatingLibrary CreatingLibrary `json:"creatingLibrary"`
	Media           RawMedia           `json:"media"`
}

// mediainfo() processes each track by dynamically determining the type based on the @type field
func mediainfo(path string) (media MediaInfo) {
	if !isMediainfoInstalled() {
		fmt.Println("mediainfo is not installed or not available in PATH")
		os.Exit(1)
	}

	// Call mediainfo to get JSON output
	output, err := getMediaInfoJSON(path)
	if err != nil {
		fmt.Printf("Error calling mediainfo: %v\n", err)
		os.Exit(1)
	}
	// Parse the JSON output
	var RawMediaInfo RawMediaInfo
	err = json.Unmarshal(output, &RawMediaInfo)
	if err != nil {
		fmt.Printf("Error parsing mediainfo JSON: %v\n", err)
		os.Exit(1)
	}
	media.CreatingLibrary = RawMediaInfo.CreatingLibrary
	// Iterate through the tracks and dynamically unmarshal based on the @type field
	for _, rawTrack := range RawMediaInfo.Media.Track {
		var trackType map[string]interface{}
		// First unmarshal to get the @type field
		if err := json.Unmarshal(rawTrack, &trackType); err != nil {
			fmt.Println("Error unmarshalling track to get @type:", err)
			continue
		}
		switch trackType["@type"] {
		case "General":
			if err := json.Unmarshal(rawTrack, &media.GeneralTrack); err != nil {
				fmt.Println("Error unmarshalling General track:", err)
				continue
			}
			fmt.Println("GeneralTrack")
			//fmt.Printf("%+v\n", generalTrack)
		case "Video":
			if err := json.Unmarshal(rawTrack, &media.VideoTrack); err != nil {
				fmt.Println("Error unmarshalling Video track:", err)
				continue
			}
			fmt.Println("VideoTrack")
			//fmt.Printf("%+v\n", videoTrack)
		case "Audio":
			var audioTrack AudioTrack
			if err := json.Unmarshal(rawTrack, &audioTrack); err != nil {
				fmt.Println("Error unmarshalling Audio track:", err)
				continue
			}
			media.AudioTracks = append(media.AudioTracks, audioTrack)
			fmt.Println("AudioTrack", audioTrack.Title)
		default:
			fmt.Println("Unknown track type:", trackType["@type"])
		}
	}
	return
}


func getMediaInfoJSON(filePath string) ([]byte, error) {
	cmd := exec.Command("mediainfo", "--Output=JSON", filePath)
	return cmd.Output()
}


func isMediainfoInstalled() bool {
	cmdName := "mediainfo"
	if runtime.GOOS == "windows" {
		cmdName = "mediainfo.exe"
	}
	_, err := exec.LookPath(cmdName)
	return err == nil
}


func placeholder2() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

