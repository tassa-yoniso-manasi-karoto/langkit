package crash

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	nurl "net/url"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
	"github.com/olekukonko/tablewriter"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"

	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/config"
	"github.com/tassa-yoniso-manasi-karoto/langkit/internal/executils"
)

func GetCrashDir() string {
	dir, _ := config.GetConfigDir()
	dir = filepath.Join(dir, "crashes")
	os.MkdirAll(dir, 0755)
	return dir
}

// Add this function to writer.go
func captureDockerInfo(w io.Writer) {
	fmt.Fprintln(w, "DOCKER STATUS")
	fmt.Fprintln(w, "=============")

	// Check if Docker is available first
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	
	cmd := executils.CommandContext(ctx, "docker", "version", "--format", "{{json .}}")
	_, versionErr := cmd.Output()
	if versionErr != nil {
		fmt.Fprintf(w, "Docker not available or not running: %v\n\n", versionErr)
		return
	}

	// If Docker is available, capture both version and info
	fmt.Fprintln(w, "Docker Version Output:")
	fmt.Fprintln(w, "---------------------")
	
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	versionCmd := executils.CommandContext(ctx2, "docker", "version")
	versionOutput, err := versionCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(w, "Error getting Docker version: %v\n", err)
	} else {
		fmt.Fprintf(w, "%s\n", string(versionOutput))
	}

	fmt.Fprintln(w, "\nDocker Info Output:")
	fmt.Fprintln(w, "------------------")
	
	ctx3, cancel3 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel3()
	infoCmd := executils.CommandContext(ctx3, "docker", "info")
	infoOutput, err := infoCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(w, "Error getting Docker info: %v\n", err)
	} else {
		fmt.Fprintf(w, "%s\n", string(infoOutput))
	}

	// List Docker images used by langkit
	fmt.Fprintln(w, "\nRelevant Docker Images:")
	fmt.Fprintln(w, "---------------------")
	relevantImages := []string{
		"ichiran",
		"aksharamukha",
	}

	ctx4, cancel4 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel4()
	imagesCmd := executils.CommandContext(ctx4, "docker", "images", "--format", "{{.Repository}}:{{.Tag}} ({{.Size}})")
	imagesOutput, err := imagesCmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(w, "Error listing Docker images: %v\n", err)
	} else {
		images := strings.Split(string(imagesOutput), "\n")
		foundRelevant := false

		for _, image := range images {
			for _, relevant := range relevantImages {
				if strings.Contains(image, relevant) {
					fmt.Fprintf(w, "%s\n", image)
					foundRelevant = true
				}
			}
		}

		if !foundRelevant {
			fmt.Fprintln(w, "No relevant langkit Docker images found")
		}
	}

	fmt.Fprintln(w)
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

// SanitizeBuffer replaces all occurrences of API keys in the buffer with REDACTED
func SanitizeBuffer(content []byte, settings config.Settings) []byte {
	// Convert to string for easier manipulation
	str := string(content)
	
	// List of API keys to sanitize
	apiKeys := []string{
		settings.APIKeys.Replicate,
		settings.APIKeys.ElevenLabs,
		settings.APIKeys.OpenAI,
		settings.APIKeys.OpenRouter,
		settings.APIKeys.Google,
	}
	
	// Replace each API key with REDACTED
	for _, key := range apiKeys {
		// Skip empty keys
		if key == "" {
			continue
		}
		// Replace all occurrences of the API key
		str = strings.ReplaceAll(str, key, "REDACTED")
	}
	
	return []byte(str)
}

// DockerNslookupCheck uses the Docker API to run "nslookup <domain>" in a BusyBox
// container. This check is intended to reveal any Docker-specific networking issues.
func DockerNslookupCheck(w io.Writer, domain string) {
	ctx := context.Background()
	finalMsg := "Docker connectivity check: "

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		fmt.Fprintf(w, finalMsg + color.Red.Sprintf("failed to create Docker client (%v)\n", err))
		log.Trace().Msgf("[docker-nslookup] Error creating Docker client: %v", err)
		return
	}
	defer func() {
		if err := cli.Close(); err != nil {
			log.Trace().Msgf("[docker-nslookup] Docker client close error: %v", err)
		}
	}()

	// Check if the busybox image is already available locally.
	_, _, err = cli.ImageInspectWithRaw(ctx, "busybox:latest")
	if err != nil {
		// If not found, pull the busybox image.
		log.Trace().Msg("[docker-nslookup] busybox image not found locally, pulling busybox:latest...")
		pullResp, err := cli.ImagePull(ctx, "docker.io/library/busybox:latest", image.PullOptions{})
		if err != nil {
			fmt.Fprintf(w, finalMsg + color.Red.Sprintf("failed to pull busybox image (%v)\n", err))
			log.Trace().Msgf("[docker-nslookup] Error pulling busybox image: %v", err)
			return
		}
		// Read the response completely to ensure the pull finishes.
		_, _ = io.Copy(ioutil.Discard, pullResp)
		_ = pullResp.Close()
	} else {
		log.Trace().Msg("[docker-nslookup] busybox image found locally; skipping pull.")
	}

	// Create a new container for the nslookup command.
	log.Trace().Msgf("[docker-nslookup] Creating container for nslookup %s...", domain)
	ctr, err := cli.ContainerCreate(
		ctx,
		&container.Config{
			Image: "busybox",
			Cmd:   []string{"nslookup", domain},
			Tty:   false,
		},
		nil, nil, nil, "",
	)
	if err != nil {
		fmt.Fprintf(w, finalMsg + color.Red.Sprintf("failed to create container (%v)\n", err))
		log.Trace().Msgf("[docker-nslookup] Error creating container: %v", err)
		return
	}
	// Remove the container once finished.
	defer func() {
		_ = cli.ContainerRemove(ctx, ctr.ID, container.RemoveOptions{Force: true})
	}()

	// Start the container.
	log.Trace().Msgf("[docker-nslookup] Starting container %s...", ctr.ID)
	if err := cli.ContainerStart(ctx, ctr.ID, container.StartOptions{}); err != nil {
		fmt.Fprintf(w, finalMsg + color.Red.Sprintf("failed to start container (%v)\n", err))
		log.Trace().Msgf("[docker-nslookup] Error starting container: %v", err)
		return
	}

	// Wait for the container to finish.
	log.Trace().Msg("[docker-nslookup] Waiting for container to exit...")
	statusCh, errCh := cli.ContainerWait(ctx, ctr.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(w, finalMsg + color.Red.Sprintf("error waiting for container (%v)\n", err))
			log.Trace().Msgf("[docker-nslookup] Error waiting for container: %v", err)
			return
		}
	case status := <-statusCh:
		log.Trace().Msgf("[docker-nslookup] Container exited with code %d", status.StatusCode)
		if status.StatusCode != 0 {
			fmt.Fprintf(w, finalMsg + color.Red.Sprintf("nslookup failed (exit code %d)\n", status.StatusCode))
			return
		}
	}

	// Fetch the container logs.
	log.Trace().Msg("[docker-nslookup] Fetching container logs...")
	logs, err := cli.ContainerLogs(ctx, ctr.ID, container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	})
	if err != nil {
		fmt.Fprintf(w, finalMsg + color.Red.Sprintf("failed to retrieve logs (%v)\n", err))
		log.Trace().Msgf("[docker-nslookup] Error retrieving logs: %v", err)
		return
	}
	defer logs.Close()

	logOutput, err := ioutil.ReadAll(logs)
	if err != nil {
		fmt.Fprintf(w, finalMsg + color.Red.Sprintf("error reading logs (%v)\n", err))
		log.Trace().Msgf("[docker-nslookup] Error reading logs: %v", err)
		return
	}

	if len(logOutput) == 0 {
		finalMsg += color.Red.Sprint("nslookup produced no output")
	} else if containsError(string(logOutput)) {
		finalMsg += color.Red.Sprint("nslookup failed (DNS error)")
	} else {
		finalMsg += color.Green.Sprint("nslookup succeeded (network available)")
	}

	// Write the summary to the crash report.
	fmt.Fprintln(w, finalMsg)
	log.Trace().Msgf("[docker-nslookup] Final result: %s", finalMsg)
}

// containsError checks if the nslookup logs contain common DNS error messages.
func containsError(logs string) bool {
	if strings.Contains(logs, "timed out") ||
		strings.Contains(logs, "connection refused") ||
		strings.Contains(logs, "no such host") {
		return true
	}
	return false
}

// contains is a simple helper function to check if a substring is in a string.
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}


func formatDuration(d time.Duration) string {
	d = d.Round(time.Millisecond)
	if d < time.Second {
		return fmt.Sprintf("%d ms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2f s", float64(d)/float64(time.Second))
}


// checkEndpointConnectivity tries to confirm that the host is reachable via TCP (with DialTimeout)
// and that the server responds to a minimal HTTP request within 3 seconds.
// currently overkill. See msg of commit 0dc32dc5ed5b794cc73667be243d443c3cc829c3.
func checkEndpointConnectivity(w io.Writer, rawURL, name string) {
	log.Trace().Msgf("[checkEndpointConnectivity] Starting check for %s: %s", name, rawURL)

	// We'll cancel this context if we time out.
	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan string, 1)

	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Trace().Msgf("[checkNet:%s] Recovered from panic: %v", name, r)
			}
			log.Trace().Msgf("[checkNet:%s] Goroutine exiting", name)
			close(resultCh)
		}()

		log.Trace().Msgf("[checkNet:%s] Goroutine started", name)
		start := time.Now()

		// 1) Parse the URL so we can do net.DialTimeout on host:port
		parsedURL, err := nurl.Parse(rawURL)
		if err != nil {
			msg := fmt.Sprintf("%s: Invalid URL '%s': %v", name, rawURL, err)
			log.Trace().Msgf("[checkNet:%s] %s", name, msg)
			resultCh <- msg
			return
		}

		host := parsedURL.Host
		scheme := parsedURL.Scheme

		// If the URL didn't include an explicit port, add one.
		// e.g. "https://replicate.com" => replicate.com:443
		if _, _, splitErr := net.SplitHostPort(host); splitErr != nil {
			switch scheme {
			case "https":
				host = host + ":443"
			case "http":
				host = host + ":80"
			default:
				// fallback if unknown scheme
				host = host + ":80"
			}
		}

		// 2) Low-level check: net.DialTimeout, guaranteed to break in 3s
		log.Trace().Msgf("[checkNet:%s] DialTimeout on %s", name, host)
		conn, dialErr := net.DialTimeout("tcp", host, 3*time.Second)
		if dialErr != nil {
			msg := fmt.Sprintf("%s: TCP dial failed - %v", name, dialErr)
			log.Trace().Msgf("[checkNet:%s] %s", name, msg)
			resultCh <- msg
			return
		}
		_ = conn.Close() // we only needed to confirm we can connect
		log.Trace().Msgf("[checkNet:%s] TCP connection successful in %s", name, time.Since(start))

		// 3) Do a short HTTP request to confirm it actually responds
		// We'll create a client that also times out quickly.
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig:       &tls.Config{InsecureSkipVerify: true}, // or false if you want real cert check
				DialContext:           (&net.Dialer{Timeout: 3 * time.Second}).DialContext,
				TLSHandshakeTimeout:   3 * time.Second,
				ResponseHeaderTimeout: 3 * time.Second,
				DisableKeepAlives:     true,
			},
			// If the server tries to stream a huge body, or never ends,
			// we won't be stuck due to the overall Timeout.
			Timeout: 3 * time.Second,
		}

		// Attach our parent context, so we can forcibly cancel if we want.
		req, err := http.NewRequestWithContext(ctx, "GET", rawURL, nil)
		if err != nil {
			msg := fmt.Sprintf("%s: Failed to create request - %v", name, err)
			log.Trace().Msgf("[checkNet:%s] %s", name, msg)
			resultCh <- msg
			return
		}

		log.Trace().Msgf("[checkNet:%s] Sending HTTP request...", name)
		resp, err := client.Do(req)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				msg := color.Red.Sprintf("%s: HTTP timeout after %s", name, formatDuration(time.Since(start)))
				log.Trace().Msgf("[checkNet:%s] %s", name, msg)
				resultCh <- msg
			} else {
				msg := color.Red.Sprintf("%s: HTTP request failed - %v", name, err)
				log.Trace().Msgf("[checkNet:%s] %s", name, msg)
				resultCh <- msg
			}
			return
		}
		defer func() {
			if cerr := resp.Body.Close(); cerr != nil {
				log.Trace().Msgf("[checkNet:%s] Error closing body: %v", name, cerr)
			}
		}()

		latency := time.Since(start)
		msg := name + ": " + color.Green.Sprintf("HTTP %s (latency: %s)", resp.Status, formatDuration(latency))
		log.Trace().Msgf("[checkNet:%s] %s", name, msg)
		resultCh <- msg
	}()

	// The main goroutine waits up to 3 seconds for the result.
	select {
	case result := <-resultCh:
		fmt.Fprintln(w, result)
		log.Trace().Msgf("[checkNet:%s] OK", name)

	case <-time.After(3 * time.Second):
		// We forcibly cancel the goroutine's context and move on.
		cancel()
		msg := fmt.Sprintf("%s: Timed out after 3 seconds (forcing cancel)", name)
		fmt.Fprintln(w, msg)
		log.Trace().Msg(msg)
	}

	log.Trace().Msgf("[checkNet:%s] check complete, returning from function", name)
}


type dirEntry struct {
	name    string
	isDir   bool
	size    int64
	modTime time.Time
	mode    os.FileMode
	symlink string
}

func FormatDirectoryListing(w io.Writer, dirPath string) error {
	absPath, err := filepath.Abs(dirPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for %s: %v", dirPath, err)
	}

	dirInfo, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("failed to access directory %s: %v", absPath, err)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("path %s is not a directory", absPath)
	}

	entries, err := os.ReadDir(absPath)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %v", absPath, err)
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

	fmt.Fprintln(w, "Path: " +absPath)

	// Create a table for the directory entries without borders and without the "Type" column.
	var tableBuffer bytes.Buffer
	table := tablewriter.NewWriter(&tableBuffer)
	table.SetHeader([]string{"Permissions", "Size", "Modified", "Name"})
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetBorder(false)

	for _, entry := range dirEntries {
		name := entry.name
		if entry.symlink != "" {
			name = fmt.Sprintf("%s → %s", entry.name, entry.symlink)
		}

		size := humanize.Bytes(uint64(entry.size))
		if entry.isDir {
			folderColor := color.HEXStyle("ebd3ad")
			size = folderColor.Sprint("<DIR>")
			name = folderColor.Sprint(name)
		} else {
			ext := strings.ToLower(filepath.Ext(entry.name))
			if isVideoFile(ext) {
				name = color.Blue.Sprint(name)
			}
			if isSubtitleFile(ext) {
				name = color.HEXStyle("90EE90").Sprint(name)
			}
		}

		table.Append([]string{
			entry.mode.Perm().String(),
			size,
			humanize.Time(entry.modTime),
			name,
		})
	}

	table.Render()
	if _, err := w.Write(tableBuffer.Bytes()); err != nil {
		return err
	}

	// Write a summary of the total number of items.
	if _, err := fmt.Fprintf(w, "\nTotal: %d items\n", len(dirEntries)); err != nil {
		return err
	}

	// Display any skipped entries due to errors.
	if len(skippedEntries) > 0 {
		if _, err := fmt.Fprintf(w, "\nSkipped entries due to errors:\n"); err != nil {
			return err
		}

		var errorBuffer bytes.Buffer
		errorTable := tablewriter.NewWriter(&errorBuffer)
		errorTable.SetHeader([]string{"Entry", "Error"})
		errorTable.SetAutoWrapText(false)
		errorTable.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
		errorTable.SetAlignment(tablewriter.ALIGN_LEFT)
		errorTable.SetBorder(false)

		for _, entry := range skippedEntries {
			parts := strings.SplitN(entry, " (error: ", 2)
			if len(parts) < 2 {
				continue
			}
			errorMsg := strings.TrimSuffix(parts[1], ")")
			errorTable.Append([]string{parts[0], errorMsg})
		}
		errorTable.Render()
		if _, err := w.Write(errorBuffer.Bytes()); err != nil {
			return err
		}
	}

	return nil
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

func writeLogs(w io.Writer, logBuffer *bytes.Buffer) error {
	if logBuffer != nil && logBuffer.Len() > 0 {
		n, err := io.Copy(w, logBuffer)
		if err != nil {
			return fmt.Errorf("failed to write logs: %w", err)
		}
		if n == 0 {
			fmt.Fprintln(w, "No logs")
		}
	} else {
		fmt.Fprintln(w, "No logs")
	}
	fmt.Fprintln(w)
	return nil
}


func isVideoFile(ext string) bool {
	videoExts := []string{
		".mp4", ".mkv", ".avi", ".mov", 
		".wmv", ".flv", ".webm", ".m4v",
		".mpg", ".mpeg", ".3gp", ".ts",
	}
	return slices.Contains(videoExts, ext)
}

func isSubtitleFile(ext string) bool {
	subtitleExts := []string{
		".srt", ".sub", ".sbv", ".ass",
		".ssa", ".vtt", ".ttml",
	}
	return slices.Contains(subtitleExts, ext)
}

func placeholder5435() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

