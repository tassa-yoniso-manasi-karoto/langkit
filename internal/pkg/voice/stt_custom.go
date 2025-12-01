package voice

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/failsafe-go/failsafe-go"
)

// CustomSTTProvider implements SpeechToTextProvider using a user-configured
// OpenAI-compatible transcription endpoint (e.g., whisper.cpp server, faster-whisper)
type CustomSTTProvider struct {
	Endpoint string
	Model    string
}

// NewCustomSTTProvider creates a new CustomSTTProvider with settings from CustomEndpoints
func NewCustomSTTProvider() *CustomSTTProvider {
	endpoint := ""
	model := ""

	if val, ok := CustomEndpoints.Load("stt_endpoint"); ok {
		if s, ok := val.(string); ok {
			endpoint = s
		}
	}
	if val, ok := CustomEndpoints.Load("stt_model"); ok {
		if s, ok := val.(string); ok {
			model = s
		}
	}

	return &CustomSTTProvider{
		Endpoint: endpoint,
		Model:    model,
	}
}

// GetName returns the provider name
func (p *CustomSTTProvider) GetName() string {
	return "custom"
}

// IsAvailable checks if the custom endpoint is configured and enabled
func (p *CustomSTTProvider) IsAvailable() bool {
	if val, ok := CustomEndpoints.Load("stt_enabled"); ok {
		if enabled, ok := val.(bool); ok && enabled {
			return p.Endpoint != ""
		}
	}
	return false
}

// TranscribeAudio converts audio to text using the custom OpenAI-compatible endpoint
func (p *CustomSTTProvider) TranscribeAudio(ctx context.Context, audioFile, language, initialPrompt string, maxTry, timeout int) (string, error) {
	if p.Endpoint == "" {
		return "", fmt.Errorf("custom STT endpoint is not configured")
	}

	// Build a retry policy for the API call
	policy := buildRetryPolicy[string](maxTry)

	// Execute the API call with the retry policy
	transcription, err := failsafe.Get(func() (string, error) {
		// Create a fresh context for this attempt
		attemptCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		// Prepare the file for upload
		file, err := os.Open(audioFile)
		if err != nil {
			return "", fmt.Errorf("couldn't open audio file: %w", err)
		}
		defer file.Close()

		// Create a pipe for streaming the file
		pr, pw := io.Pipe()
		writer := multipart.NewWriter(pw)

		// Start a goroutine to write the file data to the pipe
		go func() {
			defer pw.Close()

			// Add model field if specified
			if p.Model != "" {
				_ = writer.WriteField("model", p.Model)
			}

			// Add language field if specified
			if language != "" {
				_ = writer.WriteField("language", language)
			}

			// Add prompt field if specified (for initial context)
			if initialPrompt != "" {
				_ = writer.WriteField("prompt", initialPrompt)
			}

			// Add the file - use "file" as the field name (OpenAI convention)
			part, err := writer.CreateFormFile("file", filepath.Base(audioFile))
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
			return "", fmt.Errorf("error creating request: %w", err)
		}

		// Set headers
		req.Header.Set("Content-Type", writer.FormDataContentType())

		// Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return "", fmt.Errorf("error sending request: %w", err)
		}
		defer resp.Body.Close()

		// Check for non-2xx status code
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			body, _ := io.ReadAll(resp.Body)
			return "", fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
		}

		// Parse the response - OpenAI format returns {"text": "..."}
		var result struct {
			Text string `json:"text"`
		}

		err = json.NewDecoder(resp.Body).Decode(&result)
		if err != nil {
			return "", fmt.Errorf("error decoding response: %w", err)
		}

		return result.Text, nil
	}, policy)

	if err != nil {
		return "", fmt.Errorf("custom STT API query failed after retries: %w", err)
	}

	return transcription, nil
}

// IsCustomSTTEnabled returns true if custom STT is enabled in settings
func IsCustomSTTEnabled() bool {
	if val, ok := CustomEndpoints.Load("stt_enabled"); ok {
		if enabled, ok := val.(bool); ok {
			return enabled
		}
	}
	return false
}
