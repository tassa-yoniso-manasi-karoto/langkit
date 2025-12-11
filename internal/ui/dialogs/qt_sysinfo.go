package dialogs

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// AnkiSystemInfo holds system information from the Anki environment
type AnkiSystemInfo struct {
	AnkiVersion         string         `json:"anki_version"`
	VideoDriver         string         `json:"video_driver"`
	QtVersion           string         `json:"qt_version"`
	PyQtVersion         string         `json:"pyqt_version"`
	PythonVersion       string         `json:"python_version"`
	Platform            string         `json:"platform"`
	LangkitAddonVersion string         `json:"langkit_addon_version"`
	Screen              AnkiScreenInfo `json:"screen"`
	Addons              AnkiAddonsInfo `json:"addons"`
}

// AnkiScreenInfo holds screen information
type AnkiScreenInfo struct {
	Resolution  string  `json:"resolution"`
	RefreshRate float64 `json:"refresh_rate"`
}

// AnkiAddonsInfo holds addon information
type AnkiAddonsInfo struct {
	Active   []string `json:"active"`
	Inactive []string `json:"inactive"`
}

// GetAnkiSystemInfo retrieves system information from the Anki addon IPC server
func (q *QtFileDialog) GetAnkiSystemInfo() (*AnkiSystemInfo, error) {
	url := fmt.Sprintf("http://localhost:%d/system-info", q.dialogServerPort)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get system info from Anki: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Anki IPC server returned status %d", resp.StatusCode)
	}

	var info AnkiSystemInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to decode system info: %w", err)
	}

	return &info, nil
}
