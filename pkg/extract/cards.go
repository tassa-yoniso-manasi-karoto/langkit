package extract

import (
	"fmt"
	"os"
	"path"
	"strings"
	"strconv"
	"time"

	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/tassa-yoniso-manasi-karoto/subs2cards/pkg/subs"
)


type Task struct {
	Meta                 MediaInfo
	OriginalLang         string
	TargetLang           string
	TargetChan           int
	UseAudiotrack        int
	Offset               time.Duration
	STT                  bool
	ForeignSubtitlesFile string
	NativeSubtitlesFile  string
	MediaSourceFile      string
	OutputFieldSeparator string // defaults to "\t"
	OutputFileExtension  string // defaults to ".tsv" for "\t" and ".csv", otherwise
}

func (tsk *Task) setDefaults() {
	if tsk.OutputFieldSeparator == "" {
		tsk.OutputFieldSeparator = "\t"
	}

	if tsk.OutputFileExtension == "" {
		switch tsk.OutputFieldSeparator {
		case "\t":
			tsk.OutputFileExtension = ".tsv"
		default:
			tsk.OutputFileExtension = ".csv"
		}
	}
}

func (tsk *Task) outputBase() string {
	return strings.TrimSuffix(path.Base(tsk.ForeignSubtitlesFile), path.Ext(tsk.ForeignSubtitlesFile))
}

func (tsk *Task) outputFile() string {
	return path.Join(path.Dir(tsk.ForeignSubtitlesFile), tsk.outputBase()+"."+tsk.OutputFileExtension)
}

func (tsk *Task) mediaOutputDir() string {
	return path.Join(path.Dir(tsk.ForeignSubtitlesFile), tsk.outputBase()+".media")
}

func escape(s string) string {
	// https://datatracker.ietf.org/doc/html/rfc4180.html#section-2
	if strings.Contains(s, `"`) || strings.Contains(s, "\t") || strings.Contains(s, "\n") {
		var quoted = strings.ReplaceAll(s, `"`, `""`)
		return fmt.Sprintf(`"%s"`, quoted)
	}
	return s
}

func (tsk *Task) Execute() error {
	var nativeSubs *subs.Subtitles

	tsk.setDefaults()

	foreignSubs, err := subs.OpenFile(tsk.ForeignSubtitlesFile, false)
	if err != nil {
		return fmt.Errorf("can't read foreign subtitles: %v", err)
	}

	if tsk.NativeSubtitlesFile != "" {
		nativeSubs, err = subs.OpenFile(tsk.NativeSubtitlesFile, false)
		if err != nil {
			return fmt.Errorf("can't read native subtitles: %v", err)
		}
	}

	outStream, err := os.Create(tsk.outputFile())
	if err != nil {
		return fmt.Errorf("can't create output file: %s: %v", tsk.outputFile(), err)
	}
	defer outStream.Close()

	var mediaPrefix string
	if tsk.MediaSourceFile != "" {
		if err := os.MkdirAll(tsk.mediaOutputDir(), os.ModePerm); err != nil {
			return fmt.Errorf("can't create output directory: %s: %v", tsk.mediaOutputDir(), err)
		}
		mediaPrefix = path.Join(tsk.mediaOutputDir(), tsk.outputBase())
		tsk.Meta = mediainfo(tsk.MediaSourceFile)
		for _, track := range tsk.Meta.AudioTracks {
			if strings.Contains(strings.ToLower(track.Title), "original") {
				tsk.OriginalLang = track.Language
			}
		}
		color.Yellowln("Detected original lang:", tsk.OriginalLang)
		tsk.ChooseAudio(func(i int, track AudioTrack) {
			num, _ := strconv.Atoi(track.Channels)
			if track.Language == tsk.TargetLang && num == tsk.TargetChan {
				tsk.UseAudiotrack = i
			}
		})
		tsk.ChooseAudio(func(i int, track AudioTrack) {
			if track.Language == tsk.TargetLang {
				tsk.UseAudiotrack = i
			}
		})
		tsk.ChooseAudio(func(i int, track AudioTrack) {
			if track.Default == "Yes" {
				tsk.UseAudiotrack = i
			}
		})
		color.Greenln("Choosen idx:", tsk.UseAudiotrack, "→", tsk.Meta.AudioTracks[tsk.UseAudiotrack].Title, "ch=", tsk.Meta.AudioTracks[tsk.UseAudiotrack].Channels)
	}

	return tsk.ExportItems(foreignSubs, nativeSubs, tsk.outputBase(), tsk.MediaSourceFile, mediaPrefix, func(item *ExportedItem) error {
		fmt.Fprintf(outStream, "%s\t", escape(item.Sound))
		fmt.Fprintf(outStream, "%s\t", escape(item.Time))
		fmt.Fprintf(outStream, "%s\t", escape(item.Source))
		fmt.Fprintf(outStream, "%s\t", escape(item.Image))
		fmt.Fprintf(outStream, "%s\t", escape(item.ForeignCurr))
		fmt.Fprintf(outStream, "%s\t", escape(item.NativeCurr))
		fmt.Fprintf(outStream, "%s\t", escape(item.ForeignPrev))
		fmt.Fprintf(outStream, "%s\t", escape(item.NativePrev))
		fmt.Fprintf(outStream, "%s\t", escape(item.ForeignNext))
		fmt.Fprintf(outStream, "%s\n", escape(item.NativeNext))
		return nil
	})
}

func (tsk *Task) ChooseAudio(f func(i int, track AudioTrack)) {
	if tsk.UseAudiotrack < 0 {
		for i, track := range tsk.Meta.AudioTracks {
			f(i, track)
		}
	}
}


func placeholder() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}



