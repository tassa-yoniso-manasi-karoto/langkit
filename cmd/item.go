package cmd

import (
	"fmt"
	"os"
	"path"
	"strings"
	"time"
	"path/filepath"
	"sync"
	"sort"
	"io/fs"
	"errors"

	astisub "github.com/asticode/go-astisub"
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/media"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/subs"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/pkg/voice"
)

// ProcessedItem represents the exported information of a single subtitle item,
// where Time is the primary field which identifies the item and ForeignCurr is
// the actual text of the item. The fields NativeCurr, NativePrev and NativeNext
// will be empty unless a second subtitle file was specified for the export and
// that second subtitle file is sufficiently aligned with the first.
type ProcessedItem struct {
	ID          time.Duration
	AlreadyDone bool
	Sound       string
	Time        string
	Source      string
	Image       string
	ForeignCurr string
	NativeCurr  string
	ForeignPrev string
	NativePrev  string
	ForeignNext string
	NativeNext  string
}

// ProcessedItemWriter should write an exported item in whatever format is // selected by the user.
type ProcessedItemWriter func(*os.File, *ProcessedItem)

func (tsk *Task) Supervisor(foreignSubs *subs.Subtitles, outStream *os.File, write ProcessedItemWriter) {
	var (
		subLineChan = make(chan *astisub.Item)
		// all results are cached and later sorted, hence the chan has a cache
		itemChan = make(chan ProcessedItem, len(foreignSubs.Items))
		items []ProcessedItem
		wg sync.WaitGroup
		
		skipped int
		toCheckChan   = make(chan string)
		isAlreadyChan = make(chan bool)
	)
	tsk.Log.Debug().
		Int("capItemChan", len(foreignSubs.Items)).
		Int("workersMax", workersMax).
		Msg("Supervisor initialized, starting workers")
	for i := 1; i <= workersMax; i++ {
		wg.Add(1)
		go tsk.worker(i, subLineChan, itemChan, &wg)
	}
	go checkStringsInFile(tsk.outputFile(), toCheckChan, isAlreadyChan)
	go func() {
		for i, subLine := range foreignSubs.Items {
			// FIXME: Right now, due to parrallelism, writing is only done when all workers are done
			// so this kind of check will only work on completed tasks, therefore there is no resuming capability
			// therefore ASR/TTS already done is lost upon interruption.
			// The obvious way to fix this is to write another goroutine that sorts and checks the ID of items and
			// write them on-the-fly in order when the next ID expected is available, Keeping that for some other time...
			if toCheckChan <- tsk.FieldSep + timePosition(subLine.StartAt) + tsk.FieldSep; <-isAlreadyChan {
				tsk.Log.Trace().Int("lineNum", i).Msg("Skipping subtitle line previously processed")
				skipped += 1
				totalItems -= 1
				continue
			}
			subLineChan <- subLine
		}
		close(subLineChan)
		if skipped != 0 {
			tsk.Log.Info().Msg(fmt.Sprintf(
				"%.1f%% of items were already done and skipped (%d/%d)",
					float64(skipped)/float64(len(foreignSubs.Items))*100, skipped, len(foreignSubs.Items)))
		} else {
			tsk.Log.Debug().Msg("No line previously processed was found")
		}
	}()
	go func() {
		tsk.Log.Debug().
			Int("lenItemChan", len(itemChan)).
			Int("capItemChan", len(foreignSubs.Items)). // FIXME cleanup dbg log
			Int("lenSubLineChan", len(subLineChan)).
			Msg("Waiting for workers to finish.")
		wg.Wait()
		tsk.Log.Debug().Msg("All workers finished. Closing itemChan, toCheckChan, isAlreadyChan")
		close(itemChan)
		close(toCheckChan)
		close(isAlreadyChan)
	}()
	for item := range itemChan {
		items = append(items, item)
		if !item.AlreadyDone {
			if itembar == nil {
				itembar = mkItemBar(totalItems, tsk.descrBar())
			// in case: some encoding was done with incorrect settings,
			// user deleted .media directory to redo but in the os.Walk order,
			// there is some already processed media between that deleted dir
			// and the one that remain to do. Hence need to update bar to count these out.
			} else if itembar.GetMax() != totalItems {
				itembar.ChangeMax(totalItems)
			}
			itembar.Add(1)
		}
	}
	if itembar != nil {
		itembar.Clear()
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	for _, item := range items {
		write(outStream, &item)
	}
	tsk.ConcatWAVstoOGG("CONDENSED") // TODO probably better to put it elsewhere
}



func (tsk *Task) worker(id int, subLineChan <-chan *astisub.Item, itemChan chan ProcessedItem, wg *sync.WaitGroup) {
	for subLine := range subLineChan {
		tsk.Log.Trace().Int("workerID", id).Msg("received a subLine")
		item := tsk.ProcessItem(subLine)
		/*if err != nil { // FIXME, there is no err return anymore
			tsk.Log.Error().
				Int("srt row", i).
				Str("item", foreignItem.String()).
				Err(err).
				Msg("can't export item")
		}*/
		tsk.Log.Trace().Int("workerID", id).Int("lenItemChan", len(itemChan)).Msg("Sending item")
		itemChan <- item
		tsk.Log.Trace().Int("workerID", id).Int("lenItemChan", len(itemChan)).Msg("Item successfully sent")
	}
	tsk.Log.Trace().Int("workerID", id).Int("lenSubLineChan", len(subLineChan)).Msg("Terminating worker")
	wg.Done()
}



func (tsk *Task) ProcessItem(foreignItem *astisub.Item) (item ProcessedItem) {
	item.Source = tsk.outputBase()
	item.ForeignCurr = joinLines(foreignItem.String())

	if tsk.NativeSubs != nil {
		if nativeItem := tsk.NativeSubs.Translate(foreignItem); nativeItem != nil {
			item.NativeCurr = joinLines(nativeItem.String())
		}
	}
	audiofile, err := media.ExtractAudio("ogg", tsk.UseAudiotrack,
		tsk.Offset, foreignItem.StartAt, foreignItem.EndAt,
			tsk.MediaSourceFile, tsk.MediaPrefix, false)
	if err != nil && !errors.Is(err, fs.ErrExist) {
		tsk.Log.Error().Err(err).Msg("can't extract ogg audio")
	}
	if !tsk.DubsOnly {
		_, err = media.ExtractAudio("wav", tsk.UseAudiotrack,
			time.Duration(0), foreignItem.StartAt, foreignItem.EndAt,
				tsk.MediaSourceFile, tsk.MediaPrefix, false)
		if err != nil && !errors.Is(err, fs.ErrExist) {
			tsk.Log.Error().Err(err).Msg("can't extract wav audio")
		}
	}
	imageFile, err := media.ExtractImage(foreignItem.StartAt, foreignItem.EndAt,
		tsk.MediaSourceFile, tsk.MediaPrefix, tsk.DubsOnly)
	if err != nil {
		// check done on the AVIF because it is the most computing intensive
		if errors.Is(err, fs.ErrExist) {
			item.AlreadyDone = true
			totalItems -= 1
		} else {
			tsk.Log.Error().Err(err).Msg("can't extract image")
		}
	}
	item.ID = foreignItem.StartAt
	item.Time = timePosition(foreignItem.StartAt)
	item.Image = fmt.Sprintf("<img src=\"%s\">", path.Base(imageFile))
	item.Sound = fmt.Sprintf("[sound:%s]", path.Base(audiofile))
	
	lang := tsk.Meta.MediaInfo.AudioTracks[tsk.UseAudiotrack].Language
	switch tsk.STT {
	case "whisper":
		b, err := voice.Whisper(audiofile, 5, tsk.TimeoutSTT, lang.Part1, "")
		if err != nil {
			tsk.Log.Error().Err(err).
				Str("item", foreignItem.String()).
				Msg("Whisper error")
		}
		item.ForeignCurr = string(b)
	case "insanely-fast-whisper":
		b, err := voice.InsanelyFastWhisper(audiofile, 5, tsk.TimeoutSTT, lang.Part1)
		if err != nil {
			tsk.Log.Error().Err(err).
				Str("item", foreignItem.String()).
				Msg("InsanelyFastWhisper error")
		}
		item.ForeignCurr = string(b)
	case "universal-1":
		s, err := voice.Universal1(audiofile, 5, tsk.TimeoutSTT, lang.Part1)
		if err != nil {
			tsk.Log.Error().Err(err).
				Str("item", foreignItem.String()).
				Msg("Universal1 error")
		}
		item.ForeignCurr = s
	}
	/*if i > 0 { // FIXME this has never worked for some reason
		prevItem := foreignSubs.Items[i-1]
		item.ForeignPrev = prevItem.String()
	}

	if i+1 < len(foreignSubs.Items) {
		nextItem := foreignSubs.Items[i+1]
		item.ForeignNext = nextItem.String()
	}*/
	return
}





func (tsk *Task) ConcatWAVstoOGG(suffix string) {
	out := fmt.Sprint(tsk.MediaPrefix, ".", suffix,".ogg")
	if  _, err := os.Stat(out); err == nil {
		return
	}
	wavFiles, err := filepath.Glob(tsk.MediaPrefix+ "_*.wav")
	if err != nil {
		tsk.Log.Error().Err(err).
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("Error searching for .wav files")
	}

	if len(wavFiles) == 0 {
		tsk.Log.Warn().
			Str("mediaOutputDir", tsk.mediaOutputDir()).
			Msg("No .wav files found")
	}
	// Generate the concat list for ffmpeg
	concatFile, err := media.CreateConcatFile(wavFiles)
	if err != nil {
		tsk.Log.Error().Err(err).Msg("Error creating temporary concat file")
	}
	defer os.Remove(concatFile)

	// Run FFmpeg to concatenate and create the audio file
	media.RunFFmpegConcat(concatFile, tsk.MediaPrefix+".wav")

	// Convert WAV to OPUS using FFmpeg
	media.RunFFmpegConvert(tsk.MediaPrefix+".wav", out)
	// Clean up
	os.Remove(tsk.MediaPrefix+".wav")
	for _, f := range wavFiles {
		if err := os.Remove(f); err != nil {
			tsk.Log.Warn().Str("file", f).Msg("Removing file failed")
		}
	}
}


func (tsk *Task) descrBar() string {
	if tsk.IsBulkProcess {
		return "Bulk"
	}
	return "Items"
}


// Function that runs in a goroutine to check multiple strings in a file
func checkStringsInFile(filepath string, toCheckChan <-chan string, isAlreadyChan chan<- bool) error {
	content, _ := os.ReadFile(filepath)
	fileContent := string(content)
	for searchString := range toCheckChan {
		if len(content) == 0 {
			//color.Redln("len(content) == 0, ASSUMING "+searchString+" IT IS NOT IN FILE "+filepath)
			isAlreadyChan <- false
		} else {
			//color.Redln("SEARCHING "+searchString+"IN FILE")
			isAlreadyChan <- strings.Contains(fileContent, searchString)
		}
	}
	return nil
}

// timePosition formats the given time.Duration as a time code which can safely
// be used in file names on all platforms.
func timePosition(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	d -= s * time.Second
	ms := d / time.Millisecond
	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func joinLines(s string) string {
	s = strings.Replace(s, "\t", " ", -1)
	return strings.Replace(s, "\n", " ", -1)
}

func IsZeroLengthTimespan(last, t time.Duration) (b bool) {
	if t - last == 0 {
		b = true
	}
	return
}



func placeholder4() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}


