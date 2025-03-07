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
}

// ExecutionScope holds information specific to current processing
type ExecutionScope struct {
	StartTime     time.Time
	MediaInfoDump string
	ParentDirPath string
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
		fmt.Fprintln(&b, "Note: In all snapshots, Handler is sanitized into nil to avoid clogging dumps.\n")
		
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


func placeholder45654() {
	color.Redln(" 𝒻*** 𝓎ℴ𝓊 𝒸ℴ𝓂𝓅𝒾𝓁ℯ𝓇")
	pp.Println("𝓯*** 𝔂𝓸𝓾 𝓬𝓸𝓶𝓹𝓲𝓵𝓮𝓻")
}