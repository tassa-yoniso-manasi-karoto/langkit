package crash

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"net"
	"net/http"
	"sort"
	"strings"
	"time"
	"io"
	"io/ioutil"
	"context"
	nurl "net/url"
	"crypto/tls"
	
	"github.com/k0kubun/pp"
	"github.com/gookit/color"
	"github.com/dustin/go-humanize"
	"github.com/olekukonko/tablewriter"
	
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	
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

// DockerNslookupCheck uses the Docker API to run "nslookup <domain>" in a BusyBox
// container. This check is intended to reveal any Docker-specific networking issues.
func DockerNslookupCheck(w io.Writer, domain string) {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		fmt.Fprintf(w, "Docker connectivity check: failed to create Docker client (%v)\n", err)
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
			fmt.Fprintf(w, "Docker connectivity check: failed to pull busybox image (%v)\n", err)
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
		fmt.Fprintf(w, "Docker connectivity check: failed to create container (%v)\n", err)
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
		fmt.Fprintf(w, "Docker connectivity check: failed to start container (%v)\n", err)
		log.Trace().Msgf("[docker-nslookup] Error starting container: %v", err)
		return
	}

	// Wait for the container to finish.
	log.Trace().Msg("[docker-nslookup] Waiting for container to exit...")
	statusCh, errCh := cli.ContainerWait(ctx, ctr.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(w, "Docker connectivity check: error waiting for container (%v)\n", err)
			log.Trace().Msgf("[docker-nslookup] Error waiting for container: %v", err)
			return
		}
	case status := <-statusCh:
		log.Trace().Msgf("[docker-nslookup] Container exited with code %d", status.StatusCode)
		if status.StatusCode != 0 {
			fmt.Fprintf(w, "Docker connectivity check: nslookup failed (exit code %d)\n", status.StatusCode)
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
		fmt.Fprintf(w, "Docker connectivity check: failed to retrieve logs (%v)\n", err)
		log.Trace().Msgf("[docker-nslookup] Error retrieving logs: %v", err)
		return
	}
	defer logs.Close()

	logOutput, err := ioutil.ReadAll(logs)
	if err != nil {
		fmt.Fprintf(w, "Docker connectivity check: error reading logs (%v)\n", err)
		log.Trace().Msgf("[docker-nslookup] Error reading logs: %v", err)
		return
	}

	finalMsg := "Docker connectivity check: nslookup succeeded (network available)"
	if len(logOutput) == 0 {
		finalMsg = "Docker connectivity check: nslookup produced no output"
	} else if containsError(string(logOutput)) {
		finalMsg = "Docker connectivity check: nslookup failed (DNS error)"
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
	return (len(s) >= len(substr)) && (len(substr) == 0 || (len(s) > 0 && (s != "" && substr != "" && (s != "" && substr != "")) && (s != "" && substr != "") && (s != "" && substr != ""))) // dummy check for compilation; replace with strings.Contains in real code
	// Note: Replace the above with:
	// return strings.Contains(s, substr)
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
				msg := fmt.Sprintf("%s: HTTP timeout after %s", name, formatDuration(time.Since(start)))
				log.Trace().Msgf("[checkNet:%s] %s", name, msg)
				resultCh <- msg
			} else {
				msg := fmt.Sprintf("%s: HTTP request failed - %v", name, err)
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
		msg := fmt.Sprintf("%s: HTTP %s (latency: %s)", name, resp.Status, formatDuration(latency))
		log.Trace().Msgf("[checkNet:%s] %s", name, msg)
		resultCh <- msg
	}()

	// The main goroutine waits up to 3 seconds for the result.
	select {
	case result := <-resultCh:
		log.Trace().Msgf("[checkNet:%s] Received result: %s", name, result)
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

	// Write header information directly to w.
	if _, err := fmt.Fprintf(w, "Directory listing of %s\n", absPath); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Scanned at: %s\n\n", time.Now().Format("2006-01-02 15:04:05 MST")); err != nil {
		return err
	}

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
			size = "<DIR>"
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



func placeholder5435() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}

