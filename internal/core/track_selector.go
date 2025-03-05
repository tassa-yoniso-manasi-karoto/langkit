package core

import (
	"fmt"
	"strings"
)

// DefaultTrackSelector implements the TrackSelector interface
type DefaultTrackSelector struct {
	handler MessageHandler
}

// NewTrackSelector creates a new DefaultTrackSelector
func NewTrackSelector(handler MessageHandler) TrackSelector {
	return &DefaultTrackSelector{
		handler: handler,
	}
}

// SelectTrack selects the best audio track based on given criteria
func (ts *DefaultTrackSelector) SelectTrack(tracks []AudioTrack, targetLang Lang) (int, error) {
	// Try to find ideal track (matching language and channel preferences)
	if trackIdx := ts.findIdealTrack(tracks, targetLang); trackIdx >= 0 {
		ts.handler.ZeroLog().Debug().
			Int("trackIdx", trackIdx).
			Str("targetLang", targetLang.String()).
			Msg("Found ideal audio track matching language and channels")
		return trackIdx, nil
	}
	
	// Try to find any track with matching language
	if trackIdx := ts.findAnyLanguageMatch(tracks, targetLang); trackIdx >= 0 {
		ts.handler.ZeroLog().Debug().
			Int("trackIdx", trackIdx).
			Str("targetLang", targetLang.String()).
			Msg("Found audio track matching language (but not ideal channels)")
		return trackIdx, nil
	}
	
	// Fallback to first track
	return ts.findFirstUsableTrack(tracks, targetLang)
}

// findIdealTrack finds a track that matches the target language and has the desired channel count
func (ts *DefaultTrackSelector) findIdealTrack(tracks []AudioTrack, targetLang Lang) int {
	desiredChannels := 2 // Default to stereo
	
	for idx, track := range tracks {
		// Skip tracks with no language info
		if track.Language == nil {
			continue
		}
		
		// Skip audio description tracks
		if isAudioDescription(track.Title) {
			continue
		}
		
		// Skip duplicate language tracks when not yet at the ideal channel count track
		if track.Language.Part3 == targetLang.Part3 && track.Channels == fmt.Sprintf("%d", desiredChannels) {
			return idx
		}
	}
	
	return -1
}

// findAnyLanguageMatch finds any track that matches the target language
func (ts *DefaultTrackSelector) findAnyLanguageMatch(tracks []AudioTrack, targetLang Lang) int {
	for idx, track := range tracks {
		// Skip tracks with no language info
		if track.Language == nil {
			continue
		}
		
		// Skip audio description tracks
		if isAudioDescription(track.Title) {
			continue
		}
		
		if track.Language.Part3 == targetLang.Part3 {
			return idx
		}
	}
	
	return -1
}

// findFirstUsableTrack finds the first usable track when no language match is found
func (ts *DefaultTrackSelector) findFirstUsableTrack(tracks []AudioTrack, targetLang Lang) (int, error) {
	if len(tracks) == 0 {
		return -1, fmt.Errorf("no audio tracks found in the media file")
	}
	
	// Try to use first non-description track
	for idx, track := range tracks {
		if !isAudioDescription(track.Title) {
			ts.handler.ZeroLog().Debug().
				Int("trackIdx", idx).
				Str("targetLang", targetLang.String()).
				Str("trackLang", Str(track.Language)).
				Msg("Using first available track (language mismatch)")
			
			// Generate warning for language mismatch
			if track.Language != nil && track.Language.Part3 != targetLang.Part3 {
				return idx, fmt.Errorf("language mismatch: target language is %s but audio track is %s", 
					targetLang.String(), Str(track.Language))
			}
			
			return idx, nil
		}
	}
	
	// As last resort, use the first track even if it's an audio description
	ts.handler.ZeroLog().Debug().
		Int("trackIdx", 0).
		Str("targetLang", targetLang.String()).
		Msg("Using first track (audio description) as last resort")
	
	return 0, nil
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