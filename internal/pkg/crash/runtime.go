package crash

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"time"
	
	"github.com/dustin/go-humanize"
	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/disk"
	"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

// RuntimeInfo holds the runtime information and collection utilities.
type RuntimeInfo struct {
	StartTime time.Time
	builder   strings.Builder
}

// NewRuntimeInfo creates a new RuntimeInfo instance with the current time.
func NewRuntimeInfo() *RuntimeInfo {
	return &RuntimeInfo{
		StartTime: time.Now(),
	}
}

// String generates the complete runtime information report.
func (ri *RuntimeInfo) String() string {
	var memStats runtime.MemStats
	runtime.ReadMemStats(&memStats)
	
	// Host Information
	hostInfo, err := host.Info()
	if err == nil {
		ri.builder.WriteString(fmt.Sprintf("Host Information\n"))
		ri.builder.WriteString(fmt.Sprintf("––––––––––––––––\n"))
		ri.builder.WriteString(fmt.Sprintf("Hostname:\t%s\n", hostInfo.Hostname))
		ri.builder.WriteString(fmt.Sprintf("OS:\t\t%s\n", hostInfo.OS))
		ri.builder.WriteString(fmt.Sprintf("Platform:\t%s\n", hostInfo.Platform))
		ri.builder.WriteString(fmt.Sprintf("Platform Family:\t%s\n", hostInfo.PlatformFamily))
		ri.builder.WriteString(fmt.Sprintf("Platform Version:\t%s\n", hostInfo.PlatformVersion))
		ri.builder.WriteString(fmt.Sprintf("Kernel Version:\t%s\n", hostInfo.KernelVersion))
		ri.builder.WriteString(fmt.Sprintf("System Uptime:\t%s\n", time.Duration(hostInfo.Uptime)*time.Second))
		ri.builder.WriteString("\n")
	}

	// Memory Information
	if vmem, err := mem.VirtualMemory(); err == nil {
		ri.builder.WriteString(fmt.Sprintf("Memory Information\n"))
		ri.builder.WriteString(fmt.Sprintf("––––––––––––––––––\n"))
		ri.builder.WriteString(fmt.Sprintf("Total:\t\t%s\n", humanize.Bytes(vmem.Total)))
		ri.builder.WriteString(fmt.Sprintf("Available:\t%s\n", humanize.Bytes(vmem.Available)))
		ri.builder.WriteString(fmt.Sprintf("Used:\t\t%s (%.1f%%)\n", humanize.Bytes(vmem.Used), vmem.UsedPercent))
		ri.builder.WriteString(fmt.Sprintf("Free:\t\t%s\n\n", humanize.Bytes(vmem.Free)))
		
		if vmem.SwapTotal > 0 {
			swapUsed := vmem.SwapTotal - vmem.SwapFree
			swapUsedPercent := float64(swapUsed) / float64(vmem.SwapTotal) * 100
			ri.builder.WriteString(fmt.Sprintf("Swap Total:\t%s\n", humanize.Bytes(vmem.SwapTotal)))
			ri.builder.WriteString(fmt.Sprintf("Swap Used:\t%s (%.1f%%)\n", 
				humanize.Bytes(swapUsed), 
				swapUsedPercent))
		}
		ri.builder.WriteString("\n")
	}

	// CPU Information
	if cpuInfo, err := cpu.Info(); err == nil && len(cpuInfo) > 0 {
		ri.builder.WriteString(fmt.Sprintf("CPU Information\n"))
		ri.builder.WriteString(fmt.Sprintf("–––––––––––––––\n"))
		ri.builder.WriteString(fmt.Sprintf("Model:\t\t%s\n", cpuInfo[0].ModelName))
		ri.builder.WriteString(fmt.Sprintf("Cores:\t\t%d Physical, %d Logical\n", cpuInfo[0].Cores, runtime.NumCPU()))
		if cpuPercent, err := cpu.Percent(0, false); err == nil {
			ri.builder.WriteString(fmt.Sprintf("CPU Usage:\t%.1f%%\n", cpuPercent[0]))
		}
		ri.builder.WriteString("\n")
	}

	// Disk Information
	if partitions, err := disk.Partitions(false); err == nil {
		ri.builder.WriteString(fmt.Sprintf("Disk Information\n"))
		ri.builder.WriteString(fmt.Sprintf("––––––––––––––––\n"))
		for _, partition := range partitions {
			usage, err := disk.Usage(partition.Mountpoint)
			if err != nil {
				continue
			}
			ri.builder.WriteString(fmt.Sprintf("Path:\t\t%s\n", partition.Mountpoint))
			ri.builder.WriteString(fmt.Sprintf("Filesystem:\t%s\n", partition.Fstype))
			ri.builder.WriteString(fmt.Sprintf("Total:\t\t%s\n", humanize.Bytes(usage.Total)))
			ri.builder.WriteString(fmt.Sprintf("Used:\t\t%s (%.1f%%)\n", humanize.Bytes(usage.Used), usage.UsedPercent))
			ri.builder.WriteString(fmt.Sprintf("Free:\t\t%s\n", humanize.Bytes(usage.Free)))
			ri.builder.WriteString("\n")
		}
	}

	// Process Information
	proc, _ := process.NewProcess(int32(os.Getpid()))
	ri.builder.WriteString(fmt.Sprintf("Process Information\n"))
	ri.builder.WriteString(fmt.Sprintf("–––––––––––––––––––\n"))
	executable, _ := os.Executable()
	ri.builder.WriteString(fmt.Sprintf("Executable:\t%s\n", filepath.Base(executable)))
	ri.builder.WriteString(fmt.Sprintf("PID:\t\t%d\n", os.Getpid()))
	ri.builder.WriteString(fmt.Sprintf("PPID:\t\t%d\n", os.Getppid()))
	if wd, err := os.Getwd(); err == nil {
		ri.builder.WriteString(fmt.Sprintf("Working Dir:\t%s\n", wd))
	}
	if createTime, err := proc.CreateTime(); err == nil {
		startTime := time.Unix(createTime/1000, 0)
		ri.builder.WriteString(fmt.Sprintf("Started:\t%s\n", startTime.Format(time.RFC3339)))
		ri.builder.WriteString(fmt.Sprintf("Uptime:\t\t%s\n", time.Since(startTime).Round(time.Second)))
	}
	if memInfo, err := proc.MemoryInfo(); err == nil {
		ri.builder.WriteString(fmt.Sprintf("Memory RSS:\t%s\n", humanize.Bytes(memInfo.RSS)))
		ri.builder.WriteString(fmt.Sprintf("Memory VMS:\t%s\n", humanize.Bytes(memInfo.VMS)))
	}
	if cpuPercent, err := proc.CPUPercent(); err == nil {
		ri.builder.WriteString(fmt.Sprintf("CPU Usage:\t%.1f%%\n", cpuPercent))
	}
	ri.builder.WriteString("\n")

	// Runtime Information
	ri.builder.WriteString(fmt.Sprintf("Runtime Information\n"))
	ri.builder.WriteString(fmt.Sprintf("–––––––––––––––––––\n"))
	ri.builder.WriteString(fmt.Sprintf("Go Version:\t%s\n", runtime.Version()))
	ri.builder.WriteString(fmt.Sprintf("OS/Arch:\t%s/%s\n", runtime.GOOS, runtime.GOARCH))
	ri.builder.WriteString(fmt.Sprintf("GOMAXPROCS:\t%d\n", runtime.GOMAXPROCS(0)))
	ri.builder.WriteString(fmt.Sprintf("Goroutines:\t%d\n", runtime.NumGoroutine()))
	ri.builder.WriteString(fmt.Sprintf("CGO Calls:\t%d\n", runtime.NumCgoCall()))
	ri.builder.WriteString("\n")

	// Build Information
	if bi, ok := debug.ReadBuildInfo(); ok {
		ri.builder.WriteString(fmt.Sprintf("Build Information\n"))
		ri.builder.WriteString(fmt.Sprintf("–––––––––––––––––\n"))
		ri.builder.WriteString(fmt.Sprintf("Go Version:\t%s\n", bi.GoVersion))
		ri.builder.WriteString(fmt.Sprintf("Main Path:\t%s\n", bi.Path))
		if bi.Main.Version != "" {
			ri.builder.WriteString(fmt.Sprintf("Main Version:\t%s\n", bi.Main.Version))
		}
		if bi.Main.Sum != "" {
			ri.builder.WriteString(fmt.Sprintf("Main Sum:\t%s\n", bi.Main.Sum))
		}
		ri.builder.WriteString("\n")
	}

	// Garbage Collector
	ri.builder.WriteString(fmt.Sprintf("Garbage Collector\n"))
	ri.builder.WriteString(fmt.Sprintf("–––––––––––––––––\n"))
	ri.builder.WriteString(fmt.Sprintf("GC Cycles:\t%d\n", memStats.NumGC))
	if memStats.LastGC > 0 {
		ri.builder.WriteString(fmt.Sprintf("Last GC:\t%s ago\n", humanize.Time(time.Unix(0, int64(memStats.LastGC)))))
	}
	ri.builder.WriteString(fmt.Sprintf("GC CPU Fraction:\t%.2f%%\n", memStats.GCCPUFraction*100))
	ri.builder.WriteString(fmt.Sprintf("Next GC Target:\t%s\n", humanize.Bytes(memStats.NextGC)))

	return ri.builder.String()
}
