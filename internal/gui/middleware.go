package gui

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
)

// RuntimeConfig holds the configuration to inject into the frontend
type RuntimeConfig struct {
	APIPort int    `json:"apiPort"`
	WSPort  int    `json:"wsPort"`
	Mode    string `json:"mode"`    // "wails" or "qt"
	Runtime string `json:"runtime"` // "wails" or "anki"
}

// NewConfigInjectionMiddleware creates a middleware that injects runtime configuration
// into HTML pages. This middleware is decoupled from Wails and can be used with any
// HTTP server, making it suitable for both Wails and Qt WebEngine runtimes.
func NewConfigInjectionMiddleware(config RuntimeConfig) assetserver.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only process root HTML pages
			if r.URL.Path != "/" && r.URL.Path != "/index.html" {
				next.ServeHTTP(w, r)
				return
			}
			
			// Capture the response using httptest.ResponseRecorder
			recorder := httptest.NewRecorder()
			next.ServeHTTP(recorder, r)
			
			// Only modify successful HTML responses
			contentType := recorder.Header().Get("Content-Type")
			if recorder.Code != http.StatusOK || !strings.Contains(contentType, "text/html") {
				// Copy response as-is
				for k, v := range recorder.Header() {
					w.Header()[k] = v
				}
				w.WriteHeader(recorder.Code)
				recorder.Body.WriteTo(w)
				return
			}
			
			// Inject configuration before </head>
			body := recorder.Body.String()
			
			// Marshal config to JSON for safe injection
			configJSON, err := json.Marshal(config)
			if err != nil {
				// If marshaling fails, serve original response
				for k, v := range recorder.Header() {
					w.Header()[k] = v
				}
				w.WriteHeader(recorder.Code)
				recorder.Body.WriteTo(w)
				return
			}
			
			configScript := fmt.Sprintf(`<script>
window.__LANGKIT_CONFIG__ = %s;
</script>
`, string(configJSON))
			
			newBody := strings.Replace(body, "</head>", configScript + "</head>", 1)
			
			// Write modified response
			for k, v := range recorder.Header() {
				w.Header()[k] = v
			}
			w.Header().Set("Content-Length", fmt.Sprint(len(newBody)))
			w.WriteHeader(recorder.Code)
			w.Write([]byte(newBody))
		})
	}
}