package crash

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"strings"
	"time"
	
	"github.com/gookit/color"
	"github.com/k0kubun/pp"
)

var (
	Reporter	*ReporterInstance
	once		sync.Once
)

type SnapshotType int

const (
	GlobalSnapshot SnapshotType = iota
	ExecutionSnapshot
)

// GlobalScope holds program-wide information
type GlobalScope struct {
    StartTime     time.Time
    FFmpegPath    string
    FFmpegVersion string
    MediaInfoVer  string

    DockerStatus  string          // Docker availability status
    CommandLine   []string        // Command line arguments
    GPU           []string        // Detected GPU(s)

    // Anki-specific information (only populated when running in Anki mode)
    AnkiInfo      *AnkiInfo       // nil if not in Anki mode
}

// AnkiInfo holds Anki environment information for debug reports
type AnkiInfo struct {
    AnkiVersion         string
    VideoDriver         string
    QtVersion           string
    PyQtVersion         string
    PythonVersion       string
    Platform            string
    LangkitAddonVersion string
    ScreenResolution    string
    ScreenRefreshRate   float64
    ActiveAddons        []string
    InactiveAddons      []string
}

// ExecutionScope holds information specific to current processing
type ExecutionScope struct {
    StartTime            time.Time
    MediaInfoDump        string
    ParentDirPath        string
    
    CurrentFilePath      string            // Current file being processed
    CurrentFileIndex     int               // Current index in bulk processing
    TotalFileCount       int               // Total number of files in bulk mode
    BulkProcessingDir    string            // Directory for bulk processing
    ExpectedFileCount    int               // Expected number of files to process
    
    WorkerPoolSize       int               // Number of workers in pool
    ItemCount            int               // Number of subtitle items
    CurrentItemIndex     int               // Current item being processed
    CurrentItemTimecode  string            // Timecode of current item
    FailedSubtitleIndex  int               // Index of failed subtitle
    FailedSubtitleText   string            // Text of failed subtitle
    FailedSubtitleTimecode string          // Timecode of failed subtitle
    
    SelectedAudioTrack   int               // Selected audio track index
    AudioTrackLanguage   string            // Language of selected audio track
    SeparationProvider   string            // Voice separation provider
    ProviderAvailability string            // Provider availability status
    
    LastErrorOperation   string            // Last operation that caused an error
    LastErrorProvider    string            // Provider involved in last error
    
    CurrentSTTOperation  string            // Current STT operation
    
    TransliterationType  string            // Type of transliteration
    TransliterationLanguage string         // Language being transliterated
    CurrentTranslitProvider string         // Current transliteration provider
    TransliterationStats string            // Transliteration statistics
}
type ReporterInstance struct {
	mu sync.RWMutex

	ctx       context.Context
	startTime time.Time
	
	globalSnapshots   []executionSnapshot
	currentSnapshots  []executionSnapshot

	globalScope    GlobalScope
	executionScope ExecutionScope
}

type executionSnapshot struct {
	Timestamp time.Time
	Step      string  // What's being done
	State     string  // Current data/state dump
}

// Reporter returns the global reporter instance
func InitReporter(ctx context.Context) {
	once.Do(func() {
		Reporter = &ReporterInstance{
			ctx:       ctx,
			startTime: time.Now(),
			globalScope: GlobalScope{
				StartTime: time.Now(),
			},
		}
	})
}

// Snapshots are assumed to belong to global scope by default
func (r *ReporterInstance) SaveSnapshot(step string, state string) {
	r.saveSnapshot(GlobalSnapshot, step, state)
}

func (r *ReporterInstance) SaveExecSnapshot(step string, state string) {
	r.saveSnapshot(ExecutionSnapshot, step, state)
}

func (r *ReporterInstance) saveSnapshot(snapshotType SnapshotType, step string, state string) {
	select {
	case <-r.ctx.Done():
		return
	default:
		r.mu.Lock()
		defer r.mu.Unlock()

		snapshot := executionSnapshot{
			Timestamp: time.Now(),
			Step:      step,
			State:     state,
		}

		switch snapshotType {
		case GlobalSnapshot:
			r.globalSnapshots = append(r.globalSnapshots, snapshot)
		case ExecutionSnapshot:
			r.currentSnapshots = append(r.currentSnapshots, snapshot)
		}
	}
}

// Record updates either global or execution scope information
func (r *ReporterInstance) Record(update func(*GlobalScope, *ExecutionScope)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	update(&r.globalScope, &r.executionScope)
}

// GetScopes returns both global and execution scopes
func (r *ReporterInstance) GetScopes() (GlobalScope, ExecutionScope) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.globalScope, r.executionScope
}

// GetSnapshotsString returns a formatted string of all relevant snapshots
func (r *ReporterInstance) GetSnapshotsString() string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var b bytes.Buffer

	// Write global snapshots first
	if len(r.globalSnapshots) > 0 {
		fmt.Fprintf(&b, "GLOBAL SNAPSHOTS\n")
		fmt.Fprintf(&b, "================\n")
		fmt.Fprint(&b, "Note: In all snapshots, Handler is sanitized into nil to avoid clogging dumps.\n\n")
		
		for i, snapshot := range r.globalSnapshots {
			fmt.Fprintf(&b, "GLOBAL Snapshot #%d - %s\n", i+1, snapshot.Timestamp.Format(time.RFC3339))
			fmt.Fprintf(&b, "Step: %s\n", strings.ToUpper(snapshot.Step))
			fmt.Fprintf(&b, "State:\n%s\n", snapshot.State)
			fmt.Fprintf(&b, "-------------------\n")
		}
		fmt.Fprintf(&b, "\n")
	}

	// Write current execution snapshots
	if len(r.currentSnapshots) > 0 {
		fmt.Fprintf(&b, "EXECUTION SNAPSHOTS\n")
		fmt.Fprintf(&b, "===================\n")
		for i, snapshot := range r.currentSnapshots {
			fmt.Fprintf(&b, "EXEC Snapshot #%d - %s\n", i+1, snapshot.Timestamp.Format(time.RFC3339))
			fmt.Fprintf(&b, "Step: %s\n", strings.ToUpper(snapshot.Step))
			fmt.Fprintf(&b, "State:\n%s\n", snapshot.State)
			fmt.Fprintf(&b, "-------------------\n")
		}
	}

	return b.String()
}

func (r *ReporterInstance) ClearExecutionRecords() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.currentSnapshots = nil
	r.executionScope = ExecutionScope{
		StartTime: time.Now(),
	}
}

func (r *ReporterInstance) ClearAllRecords() {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.startTime = time.Now()
	r.globalSnapshots = nil
	r.currentSnapshots = nil
	r.globalScope = GlobalScope{
		StartTime: time.Now(),
	}
	r.executionScope = ExecutionScope{
		StartTime: time.Now(),
	}
}

// GetUptime returns the duration since reporter was started
func (r *ReporterInstance) GetUptime() time.Duration {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return time.Since(r.startTime)
}

// GetSnapshot returns the state for a specific snapshot step
func (r *ReporterInstance) GetSnapshot(step string) string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	// Check global snapshots first
	for i := len(r.globalSnapshots) - 1; i >= 0; i-- {
		if r.globalSnapshots[i].Step == step {
			return r.globalSnapshots[i].State
		}
	}
	
	// If not found in global, check execution snapshots
	for i := len(r.currentSnapshots) - 1; i >= 0; i-- {
		if r.currentSnapshots[i].Step == step {
			return r.currentSnapshots[i].State
		}
	}
	
	return "" // Return empty string if snapshot not found
}


func placeholder45654() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}