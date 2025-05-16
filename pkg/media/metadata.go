package media

import (
	"fmt"
	"os"
	"os/exec"
)

// AddMetadataToAudio adds metadata to an audio file using FFmpeg
func AddMetadataToAudio(filePath, metadataKey, metadataValue string) error {
	tempFile := filePath + ".tmp"
	
	// Create FFmpeg command to add metadata
	args := []string{
		"-i", filePath,
		"-c", "copy",
		"-metadata", metadataKey + "=" + metadataValue,
		"-y", tempFile,
	}
	
	cmd := exec.Command(FFmpegPath, args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("ffmpeg metadata error: %w: %s", err, string(output))
	}
	
	// Replace original file with the new one
	if err := os.Rename(tempFile, filePath); err != nil {
		return fmt.Errorf("error replacing file after adding metadata: %w", err)
	}
	
	return nil
}