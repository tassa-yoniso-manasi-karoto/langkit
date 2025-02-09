package crash

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"net/http"
	"sort"
	"strings"
	"time"
	"io"

	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
)


func GetCrashDir() string {
	dir, _ := config.GetConfigDir()
	dir = filepath.Join(dir, "crashes")
	os.MkdirAll(dir, 0755)
	return dir
}

// Print environment variables safely, redacting sensitive values
func printEnvironment(w io.Writer) {
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		name, value := parts[0], parts[1]
		
		if containsSensitiveInfo(name) {
			fmt.Fprintf(w, "%s=REDACTED\n", name)
		} else {
			fmt.Fprintf(w, "%s=%s\n", name, value)
		}
	}
}

func containsSensitiveInfo(env string) bool {
	sensitive := []string{
		"KEY", "TOKEN", "SECRET", "PASSWORD", "CREDENTIAL",
		"AUTH", "PRIVATE", "CERT", "PWD", "PASS",
	}
	envUpper := strings.ToUpper(env)
	for _, s := range sensitive {
		// Filter PWD for password but not for Present Working Directory
		if strings.Contains(envUpper, s) && envUpper != "PWD" {
			return true
		}
	}
	return false
}

func MaskAPIKey(key string) string {
	if len(key) == 0 {
		return "not set!"
	} else if len(key) <= 8 {
		return "********"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", float64(d)/float64(time.Second))
}


func checkEndpointConnectivity(w io.Writer, url, name string) {
	client := &http.Client{
		Timeout: 3 * time.Second,
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}

	start := time.Now()
	req, err := http.NewRequest("HEAD", url, nil)
	if err != nil {
		fmt.Fprintf(w, "%s: Failed to create request - %v\n", name, err)
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Fprintf(w, "%s: Failed to connect - %v\n", name, err)
		return
	}
	defer resp.Body.Close()

	latency := time.Since(start)
	fmt.Fprintf(w, "%s: Status %s (latency: %s)\n",
		name,
		resp.Status,
		formatDuration(latency),
	)
}



type dirEntry struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
	mode    os.FileMode
	symlink string
}

func FormatDirectoryListing(dirPath string) (string, error) {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute path for %s: %v", dirPath, err)
	}

	dirInfo, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to access directory %s: %v", absPath, err)
	}
	if !dirInfo.IsDir() {
		return "", fmt.Errorf("path %s is not a directory", absPath)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return "", fmt.Errorf("failed to read directory %s: %v", absPath, err)
	}

	var dirEntries []dirEntry
	var skippedEntries []string
	
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			skippedEntries = append(skippedEntries, fmt.Sprintf("%s (error: %v)", entry.Name(), err))
			continue
		}

		de := dirEntry{
			name:    entry.Name(),
			isDir:   info.IsDir(),
			size:    info.Size(),
			modTime: info.ModTime(),
			mode:    info.Mode(),
		}

		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(filepath.Join(absPath, entry.Name()))
			if err == nil {
				de.symlink = target
			}
		}

		dirEntries = append(dirEntries, de)
	}

	sort.Slice(dirEntries, func(i, j int) bool {
		if dirEntries[i].isDir != dirEntries[j].isDir {
			return dirEntries[i].isDir
		}
		return strings.ToLower(dirEntries[i].name) < strings.ToLower(dirEntries[j].name)
	})

	var result strings.Builder
	result.WriteString(fmt.Sprintf("Directory listing of %s\n", absPath))
	result.WriteString(fmt.Sprintf("Scanned at: %s\n\n", time.Now().Format("2006-01-02 15:04:05 MST")))

	// Create table buffer
	var tableBuffer bytes.Buffer
	table := tablewriter.NewWriter(&tableBuffer)
	
	// Configure table with enhanced formatting
	table.SetHeader([]string{"Type", "Permissions", "Size", "Modified", "Name"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("┼")
	table.SetColumnSeparator("│")
	table.SetRowSeparator("─")
	table.SetHeaderLine(true)
	table.SetBorder(true)
	table.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
	table.SetTablePadding(" ")
	table.SetNoWhiteSpace(true)

	// Style the table
	table.SetHeaderColor(
		tablewriter.Colors{tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold},
		tablewriter.Colors{tablewriter.Bold},
	)

	// Add entries to table
	for _, entry := range dirEntries {
		entryType := "F"
		if entry.isDir {
			entryType = "D"
		} else if entry.mode&os.ModeSymlink != 0 {
			entryType = "L"
		}

		name := entry.name
		if entry.symlink != "" {
			name = fmt.Sprintf("%s → %s", entry.name, entry.symlink)
		}

		size := humanize.Bytes(uint64(entry.size))
		if entry.isDir {
			size = "<DIR>"
		}

		table.Append([]string{
			entryType,
			entry.mode.Perm().String(),
			size,
			humanize.Time(entry.modTime),
			name,
		})
	}

	table.Render()
	result.WriteString(tableBuffer.String())

	// Add summary
	result.WriteString(fmt.Sprintf("\nTotal: %d items\n", len(dirEntries)))
	
	// Add errors table if needed
	if len(skippedEntries) > 0 {
		result.WriteString("\nSkipped entries due to errors:\n")
		table = tablewriter.NewWriter(&result)
		table.SetHeader([]string{"Entry", "Error"})
		table.SetAutoWrapText(false)
		table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		table.SetAlignment(tablewriter.ALIGN_LEFT)
		table.SetCenterSeparator("┼")
		table.SetColumnSeparator("│")
		table.SetRowSeparator("─")
		table.SetHeaderLine(true)
		table.SetBorder(true)
		table.SetBorders(tablewriter.Border{Left: true, Top: true, Right: true, Bottom: true})
		table.SetTablePadding(" ")
		table.SetNoWhiteSpace(true)
		
		// Style the error table header
		table.SetHeaderColor(
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor},
			tablewriter.Colors{tablewriter.Bold, tablewriter.FgRedColor},
		)

		for _, entry := range skippedEntries {
			parts := strings.SplitN(entry, " (error: ", 2)
			errorMsg := strings.TrimSuffix(parts[1], ")")
			table.Append([]string{parts[0], errorMsg})
		}
		table.Render()
	}

	return result.String(), nil
}


// GetUserCountry fetches the user's country from Mullvad's API
func GetUserCountry() (string, error) {
	client := &http.Client{
		Timeout: 2 * time.Second,
	}

	req, err := http.NewRequest("GET", "https://am.i.mullvad.net/country", nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %w", err)
	}
	return string(body), nil
}

