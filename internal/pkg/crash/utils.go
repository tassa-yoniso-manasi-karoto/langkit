package crash

import (
	"fmt"
	"time"
	"strings"
)

func MaskAPIKey(key string) string {
	if len(key) == 0 {
		return "not set!"
	} else if len(key) <= 8 {
		return "********"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}


func containsSensitiveInfo(env string) bool {
	sensitive := []string{
		"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL",
		"AUTH", "PRIVATE", "CERT", "PWD", "PASS",
	}
	envUpper := strings.ToUpper(env)
	for _, s := range sensitive {
		if strings.Contains(envUpper, s) {
			return true
		}
	}
	return false
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", float64(d)/float64(time.Second))
}
