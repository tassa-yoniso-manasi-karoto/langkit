package dialogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Compile-time check that QtFileDialog implements FileDialog
var _ FileDialog = (*QtFileDialog)(nil)

// QtFileDialog implements FileDialog using Qt dialogs via HTTP IPC
// This communicates with the Python/Qt dialog server in Anki addon
type QtFileDialog struct {
	dialogServerPort int
	httpClient       *http.Client
}

// NewQtFileDialog creates a new Qt file dialog instance
func NewQtFileDialog(dialogServerPort int) *QtFileDialog {
	return &QtFileDialog{
		dialogServerPort: dialogServerPort,
		httpClient: &http.Client{
			Timeout: 60 * time.Second, // 60 second timeout for dialog operations
		},
	}
}

// dialogRequest represents a request to the dialog server
type dialogRequest struct {
	Title           string       `json:"title,omitempty"`
	DefaultFilename string       `json:"defaultFilename,omitempty"`
	Filters         []FileFilter `json:"filters,omitempty"`
}

// dialogResponse represents a response from the dialog server
type dialogResponse struct {
	Path  string `json:"path"`
	Error string `json:"error,omitempty"`
}

// SaveFile opens a save file dialog
func (q *QtFileDialog) SaveFile(options SaveFileOptions) (string, error) {
	log.Printf("[QtFileDialog] SaveFile called with title: %s, defaultFilename: %s",
		options.Title, options.DefaultFilename)

	req := dialogRequest{
		Title:           options.Title,
		DefaultFilename: options.DefaultFilename,
		Filters:         options.Filters,
	}

	resp, err := q.sendDialogRequest("/dialog/save", req)
	if err != nil {
		log.Printf("[QtFileDialog] SaveFile error: %v", err)
		return "", fmt.Errorf("failed to show save dialog: %w", err)
	}

	if resp.Error != "" {
		log.Printf("[QtFileDialog] Dialog returned error: %s", resp.Error)
		return "", fmt.Errorf("dialog error: %s", resp.Error)
	}

	log.Printf("[QtFileDialog] SaveFile success, path: %s", resp.Path)
	return resp.Path, nil
}

// OpenFile opens an open file dialog
func (q *QtFileDialog) OpenFile(options OpenFileOptions) (string, error) {
	req := dialogRequest{
		Title:   options.Title,
		Filters: options.Filters,
	}

	resp, err := q.sendDialogRequest("/dialog/open", req)
	if err != nil {
		return "", fmt.Errorf("failed to show open dialog: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("dialog error: %s", resp.Error)
	}

	return resp.Path, nil
}

// OpenDirectory opens an open directory dialog
func (q *QtFileDialog) OpenDirectory(options OpenDirectoryOptions) (string, error) {
	req := dialogRequest{
		Title: options.Title,
	}

	resp, err := q.sendDialogRequest("/dialog/directory", req)
	if err != nil {
		return "", fmt.Errorf("failed to show directory dialog: %w", err)
	}

	if resp.Error != "" {
		return "", fmt.Errorf("dialog error: %s", resp.Error)
	}

	return resp.Path, nil
}

// sendDialogRequest sends a request to the dialog server
func (q *QtFileDialog) sendDialogRequest(endpoint string, req dialogRequest) (*dialogResponse, error) {
	// Construct URL
	url := fmt.Sprintf("http://localhost:%d%s", q.dialogServerPort, endpoint)
	log.Printf("[QtFileDialog] Sending request to %s", url)

	// Marshal request
	jsonData, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	log.Printf("[QtFileDialog] Request body: %s", string(jsonData))

	// Create HTTP request with appropriate timeout
	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	// Send request
	log.Printf("[QtFileDialog] Sending POST request...")
	httpResp, err := q.httpClient.Do(httpReq)
	if err != nil {
		log.Printf("[QtFileDialog] HTTP request failed: %v", err)
		return nil, fmt.Errorf("failed to send request to dialog server: %w", err)
	}
	defer httpResp.Body.Close()

	// Check status code
	if httpResp.StatusCode != http.StatusOK {
		log.Printf("[QtFileDialog] Server returned status %d", httpResp.StatusCode)
		return nil, fmt.Errorf("dialog server returned status %d", httpResp.StatusCode)
	}

	// Decode response
	var resp dialogResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		log.Printf("[QtFileDialog] Failed to decode response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	log.Printf("[QtFileDialog] Response received: path=%s, error=%s", resp.Path, resp.Error)
	return &resp, nil
}

// IsAvailable checks if the dialog server is available
func (q *QtFileDialog) IsAvailable() bool {
	url := fmt.Sprintf("http://localhost:%d/health", q.dialogServerPort)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return false
	}

	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}