package core

import (
	"encoding/json"
	"fmt"

	iso "github.com/barbashov/iso639-3"
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	"github.com/tidwall/pretty"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/crash"
)

var MediainfoPath = "mediainfo"

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
	LangRaw                string `json:"Language"`
	Language               Lang
	Default                string `json:"Default"`
	AlternateGroup         string `json:"AlternateGroup"`
}

// TextTrack represents subtitle/text track metadata from mediainfo
type TextTrack struct {
	Type        string `json:"@type"`
	StreamOrder string `json:"StreamOrder"` // For FFmpeg -map (0-based stream index)
	ID          string `json:"ID"`
	Format      string `json:"Format"`      // "ASS", "PGS", "SubRip", "UTF-8"
	CodecID     string `json:"CodecID"`     // "S_TEXT/ASS", "S_HDMV/PGS"
	Duration    string `json:"Duration"`
	Title       string `json:"Title"`       // "Dialogue@MTBB", "Signs/Songs"
	LangRaw     string `json:"Language"`    // "en", "ja", "zh-Hans"
	Language    Lang                        // Parsed via ParseLanguageTags
	Default     string `json:"Default"`     // "Yes" or "No"
	Forced      string `json:"Forced"`      // "Yes" or "No"
}

// MediaInfo represents the media information including general, video, audio, and text tracks
type MediaInfo struct {
	CreatingLibrary CreatingLibrary
	GeneralTrack    GeneralTrack
	VideoTrack      VideoTrack
	AudioTracks     []AudioTrack
	TextTracks      []TextTrack
}

type RawMedia struct {
	Ref   string            `json:"@ref"`
	Track []json.RawMessage `json:"track"`
}

type RawMediaInfo struct {
	CreatingLibrary CreatingLibrary `json:"creatingLibrary"`
	Media           RawMedia        `json:"media"`
}

// Mediainfo() processes each track by dynamically determining the type based on the @type field
func Mediainfo(path string) (media MediaInfo, err error) {
	output, err := getMediaInfoJSON(path)
	if err != nil {
		return media, fmt.Errorf("error calling mediainfo: %w", err)
	}
	
	var RawMediaInfo RawMediaInfo
	err = json.Unmarshal(output, &RawMediaInfo)
	if err != nil {
		return media, fmt.Errorf("error parsing mediainfo JSON: %w", err)
	}

	crash.Reporter.Record(func(gs *crash.GlobalScope, es *crash.ExecutionScope) {
		gs.MediaInfoVer = RawMediaInfo.CreatingLibrary.Version
		es.MediaInfoDump = string(pretty.Pretty(output))
	})

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
			//fmt.Printf("%+v\n", generalTrack)
		case "Video":
			if err := json.Unmarshal(rawTrack, &media.VideoTrack); err != nil {
				fmt.Println("Error unmarshalling Video track:", err)
				continue
			}
			//fmt.Printf("%+v\n", videoTrack)
		case "Audio":
			var audioTrack AudioTrack
			if err := json.Unmarshal(rawTrack, &audioTrack); err != nil {
				fmt.Println("Error unmarshalling Audio track:", err)
				continue
			}
			// Use ParseLanguageTags for proper BCP 47 handling (same as TextTrack)
			if audioTrack.LangRaw != "" {
				langs, err := ParseLanguageTags([]string{audioTrack.LangRaw})
				if err == nil && len(langs) > 0 {
					audioTrack.Language = langs[0]
				}
			}
			// Fallback to "und" if no language detected
			if audioTrack.Language.Language == nil {
				audioTrack.Language.Language = iso.FromPart3Code("und")
			}
			media.AudioTracks = append(media.AudioTracks, audioTrack)
		case "Text":
			var textTrack TextTrack
			if err := json.Unmarshal(rawTrack, &textTrack); err != nil {
				fmt.Println("Error unmarshalling Text track:", err)
				continue
			}
			// Use ParseLanguageTags for proper BCP 47 handling (gets both Language AND Subtag)
			if textTrack.LangRaw != "" {
				langs, err := ParseLanguageTags([]string{textTrack.LangRaw})
				if err == nil && len(langs) > 0 {
					textTrack.Language = langs[0]
				}
			}
			// Fallback to "und" if no language detected
			if textTrack.Language.Language == nil {
				textTrack.Language.Language = iso.FromPart3Code("und")
			}
			media.TextTracks = append(media.TextTracks, textTrack)
		default:
			// Silently ignore unknown track types (e.g., Menu, Attachment)
			_ = trackType["@type"]
		}
	}
	return
}

func getMediaInfoJSON(filePath string) ([]byte, error) {
	cmd := executils.NewCommand(MediainfoPath, "--Output=JSON", filePath)
	return cmd.Output()
}

var CodecToExtension = map[string]string{
    "MP3":        ".mp3",   // MP3 (MPEG Audio Layer III)
    "AAC":        ".aac",   // AAC (Advanced Audio Codec)
    "WMA":        ".wma",   // WMA (Windows Media Audio)
    "FLAC":       ".flac",  // FLAC (Free Lossless Audio Codec)
    "ALAC":       ".m4a",   // ALAC (Apple Lossless Audio Codec)
    "Opus":       ".opus",  // Opus
    "Vorbis":     ".ogg",   // OGG Vorbis
    "PCM":        ".wav",   // PCM (Pulse Code Modulation - usually in WAV)
    "WAV":        ".wav",   // WAV (Waveform Audio File Format)
    "AIFF":       ".aiff",  // AIFF (Audio Interchange File Format)
    "RealAudio":  ".ra",    // RealAudio
    "AMR":        ".amr",   // Adaptive Multi-Rate Audio Codec
    "MPEG-4 ALS": ".mp4",   // MPEG-4 ALS (Audio Lossless Coding)
    "MPEG Audio": ".mp3",   // MPEG Audio (commonly refers to MP3)
    "AC-3":       ".ac3",   // AC-3 (Dolby Digital)
    "DTS":        ".dts",   // DTS (Digital Theater Systems)
    "TrueHD":     ".thd",   // Dolby TrueHD
    "E-AC-3":     ".eac3",  // Enhanced AC-3
    "MKA":        ".mka",   // Matroska Audio
    "WebM":       ".webm",  // WebM (Opus or Vorbis audio codec)
    "Speex":      ".spx",   // Speex (mainly used in .ogg containers)
    "Musepack":   ".mpc",   // Musepack
}


func placeholder2() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

