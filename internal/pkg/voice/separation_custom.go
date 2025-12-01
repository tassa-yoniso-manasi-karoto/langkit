package voice

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/failsafe-go/failsafe-go"
)

// CustomSeparationProvider implements AudioSeparationProvider using a user-configured
// endpoint for voice isolation/separation
type CustomSeparationProvider struct {
	Endpoint string
}

// NewCustomSeparationProvider creates a new CustomSeparationProvider with settings from CustomEndpoints
func NewCustomSeparationProvider() *CustomSeparationProvider {
	endpoint := ""

	if val, ok := CustomEndpoints.Load("voice_isolation_endpoint"); ok {
		if s, ok := val.(string); ok {
			endpoint = s
		}
	}

	return &CustomSeparationProvider{
		Endpoint: endpoint,
	}
}

// GetName returns the provider name
func (p *CustomSeparationProvider) GetName() string {
	return "custom"
}

// IsAvailable checks if the custom endpoint is configured and enabled
func (p *CustomSeparationProvider) IsAvailable() bool {
	if val, ok := CustomEndpoints.Load("voice_isolation_enabled"); ok {
		if enabled, ok := val.(bool); ok && enabled {
			return p.Endpoint != ""
		}
	}
	return false
}

// SeparateVoice extracts voice from audio using the custom endpoint
// The endpoint should accept a multipart form POST with an "audio" field
// and return the processed audio bytes directly
func (p *CustomSeparationProvider) SeparateVoice(ctx context.Context, audioFile, outputFormat string, maxTry, timeout int) ([]byte, error) {
	if p.Endpoint == "" {
		return nil, fmt.Errorf("custom voice isolation endpoint is not configured")
	}

	// Build a retry policy for the API call
	policy := buildRetryPolicy[[]byte](maxTry)

	// Execute the API call with the retry policy
	audioBytes, err := failsafe.Get(func() ([]byte, error) {
		// Create a fresh context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Prepare the file for upload
		file, err := os.Open(audioFile)
		if err != nil {
			return nil, fmt.Errorf("couldn't open audio file: %w", err)
		}
		defer file.Close()

		// Create a pipe for streaming the file
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		// Start a goroutine to write the file data to the pipe
		go func() {
			defer pw.Close()

			// Add output_format field if specified
			if outputFormat != "" {
				_ = writer.WriteField("output_format", outputFormat)
			}

			// Add the audio file
			part, err := writer.CreateFormFile("audio", filepath.Base(audioFile))
			if err != nil {
				pw.CloseWithError(fmt.Errorf("error creating form file: %w", err))
				return
			}

			// Copy the file data to the multipart form
			_, err = io.Copy(part, file)
			if err != nil {
				pw.CloseWithError(fmt.Errorf("error copying file data: %w", err))
				return
			}

			// Close the writer to finalize the form data
			err = writer.Close()
			if err != nil {
				pw.CloseWithError(fmt.Errorf("error closing multipart writer: %w", err))
				return
			}
		}()

		// Create the request
		req, err := http.NewRequestWithContext(attemptCtx, "POST", p.Endpoint, pr)
		if err != nil {
			return nil, fmt.Errorf("error creating request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error sending request: %w", err)
		}
		defer resp.Body.Close()

		// Check for non-2xx status code
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		// Read the response body - expecting raw audio bytes
		audioData, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading response: %w", err)
		}

		return audioData, nil
	}, policy)

	if err != nil {
		return nil, fmt.Errorf("custom voice isolation API query failed after retries: %w", err)
	}

	return audioBytes, nil
}

// IsCustomVoiceIsolationEnabled returns true if custom voice isolation is enabled in settings
func IsCustomVoiceIsolationEnabled() bool {
	if val, ok := CustomEndpoints.Load("voice_isolation_enabled"); ok {
		if enabled, ok := val.(bool); ok {
			return enabled
		}
	}
	return false
}
