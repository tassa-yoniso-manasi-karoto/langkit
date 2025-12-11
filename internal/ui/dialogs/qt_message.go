package dialogs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Compile-time check that QtMessageDialog implements MessageDialog
var _ MessageDialog = (*QtMessageDialog)(nil)

// QtMessageDialog implements MessageDialog using Qt dialogs via HTTP IPC
type QtMessageDialog struct {
	dialogServerPort int
	httpClient       *http.Client
}

// NewQtMessageDialog creates a new Qt message dialog instance
func NewQtMessageDialog(dialogServerPort int) *QtMessageDialog {
	return &QtMessageDialog{
		dialogServerPort: dialogServerPort,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

// messageRequest represents a request to the message dialog endpoint
type messageRequest struct {
	Title   string `json:"title"`
	Message string `json:"message"`
	Type    string `json:"type"` // "info", "warning", "error", "question"
}

// messageResponse represents a response from the message dialog endpoint
type messageResponse struct {
	Accepted bool   `json:"accepted"` // true if OK/Yes clicked
	Error    string `json:"error,omitempty"`
}

// ShowMessage displays a message dialog via Qt IPC
func (q *QtMessageDialog) ShowMessage(title, message string, msgType MessageType) (bool, error) {
	typeStr := "info"
	switch msgType {
	case MessageInfo:
		typeStr = "info"
	case MessageWarning:
		typeStr = "warning"
	case MessageError:
		typeStr = "error"
	case MessageQuestion:
		typeStr = "question"
	}

	req := messageRequest{
		Title:   title,
		Message: message,
		Type:    typeStr,
	}

	url := fmt.Sprintf("http://localhost:%d/dialog/message", q.dialogServerPort)

	jsonData, err := json.Marshal(req)
	if err != nil {
		return false, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := q.httpClient.Do(httpReq)
	if err != nil {
		return false, fmt.Errorf("failed to send request to dialog server: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("dialog server returned status %d", httpResp.StatusCode)
	}

	var resp messageResponse
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return false, fmt.Errorf("failed to decode response: %w", err)
	}

	if resp.Error != "" {
		return false, fmt.Errorf("dialog error: %s", resp.Error)
	}

	return resp.Accepted, nil
}
