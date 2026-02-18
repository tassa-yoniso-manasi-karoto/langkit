package core

import (
	"fmt"
	"strings"
	"strconv"
)


// Audio track selection functions
func (tsk *Task) ChooseAudio(helper func(tsk *Task, i int, track AudioTrack) error) (err error) {
	if tsk.UseAudiotrack < 0 {
		for i, track := range tsk.Meta.MediaInfo.AudioTracks {
			if err = helper(tsk, i, track); err != nil {
				return
			}
		}
	}
	return
}


type SelectionHelper func(*Task, int, AudioTrack) error

func getIdealTrack(tsk *Task, i int, track AudioTrack) error {
	num, _ := strconv.Atoi(track.Channels)
	tsk.Handler.ZeroLog().Trace().
		Bool("isTargLang?", *track.Language.Language == *tsk.Targ.Language).
		Bool("isTargetChanNum?", num == tsk.TargetChan).
		Bool("track.Title_empty?", track.Title == "").
		Bool("track.Title_notEmpty_notAudioDescr", track.Title != "" && !strings.Contains(strings.ToLower(track.Title), "audio description")).
		Msg("getIdealTrack")
	if *track.Language.Language == *tsk.Targ.Language && num == tsk.TargetChan &&
		(track.Title == "" || track.Title != "" && !strings.Contains(strings.ToLower(track.Title), "audio description")) {
			tsk.UseAudiotrack = i
			tsk.Handler.ZeroLog().Debug().Msg("getIdealTrack selected UseAudiotrack")
	}
	return nil
}

func getAnyTargLangMatch(tsk *Task, i int, track AudioTrack) error {
	tsk.Handler.ZeroLog().Trace().
		Bool("isTargLang?", *track.Language.Language == *tsk.Targ.Language).Msg("getAnyTargLangMatch")
	if *track.Language.Language == *tsk.Targ.Language {
		tsk.UseAudiotrack = i
		tsk.Handler.ZeroLog().Debug().Msg("getAnyTargLangMatch selected UseAudiotrack")
	}
	return nil
}

func getFirstTrack(tsk *Task, i int, track AudioTrack) error {
	tsk.Handler.ZeroLog().Trace().
		Bool("hasLang", track.Language.Language != nil).
		Bool("lang_isn't_target", *track.Language.Language != *tsk.Targ.Language).Msg("getFirstTrack")
	if track.Language.Language != nil && *track.Language.Language != *tsk.Targ.Language {
		return fmt.Errorf("No audiotrack tagged with the requested target language exists. " +
			"If it isn't a misinput please use the audiotrack override to set a track number manually.")
	}
	// Having found no audiotrack tagged with target language, we can
	// assume first audiotrack is the target if it doesn't have a language tag
	tsk.UseAudiotrack = i
	tsk.Handler.ZeroLog().Debug().Msg("getFirstTrack selected UseAudiotrack")
	return nil
}


// isAudioDescription checks if a track title indicates it's an audio description track
func isAudioDescription(title string) bool {
	if title == "" {
		return false
	}
	
	lowerTitle := strings.ToLower(title)
	return strings.Contains(lowerTitle, "description") || 
		strings.Contains(lowerTitle, "descriptive") ||
		strings.Contains(lowerTitle, "commentary") ||
		strings.Contains(lowerTitle, "narration")
}