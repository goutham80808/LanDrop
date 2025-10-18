package p2p

import (
	"fmt"
	"strings"
	"time"
)

// ProgressStyle defines different progress bar styles
type ProgressStyle int

const (
	ProgressStyleSimple ProgressStyle = iota
	ProgressStyleDetailed
	ProgressStyleMinimal
)

// ProgressColors for terminal output
type ProgressColors struct {
	Reset     string
	Red       string
	Green     string
	Yellow    string
	Blue      string
	Magenta   string
	Cyan      string
	White     string
	Gray      string
	Bold      string
}

// Terminal colors (ANSI escape codes)
var Colors = ProgressColors{
	Reset:   "\033[0m",
	Red:     "\033[31m",
	Green:   "\033[32m",
	Yellow:  "\033[33m",
	Blue:    "\033[34m",
	Magenta: "\033[35m",
	Cyan:    "\033[36m",
	White:   "\033[37m",
	Gray:    "\033[90m",
	Bold:    "\033[1m",
}

// ProgressTracker manages real-time progress tracking for file transfers
type ProgressTracker struct {
	filename      string
	totalSize     int64
	totalChunks   int
	direction     string // "sent" or "received"
	startTime     time.Time
	style         ProgressStyle
	quiet         bool
	lastUpdate    time.Time
	updateInterval time.Duration
	spinIndex     int    // For spinning animation
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker(filename string, totalSize int64, totalChunks int, direction string, style ProgressStyle) *ProgressTracker {
	return &ProgressTracker{
		filename:       filename,
		totalSize:      totalSize,
		totalChunks:    totalChunks,
		direction:      direction,
		startTime:      time.Now(),
		style:          style,
		quiet:          false,
		lastUpdate:     time.Now(),
		updateInterval: 50 * time.Millisecond,  // Update every 50ms for smoother animation
	}
}

// SetQuiet disables progress output
func (pt *ProgressTracker) SetQuiet(quiet bool) {
	pt.quiet = quiet
}

// SetUpdateInterval sets the minimum interval between progress updates
func (pt *ProgressTracker) SetUpdateInterval(interval time.Duration) {
	pt.updateInterval = interval
}

// PrintProgress displays the current progress with different styles
func (pt *ProgressTracker) PrintProgress(completedChunks int, bytesTransferred int64) {
	if pt.quiet {
		return
	}

	// Throttle updates to avoid flickering
	now := time.Now()
	if now.Sub(pt.lastUpdate) < pt.updateInterval && completedChunks < pt.totalChunks {
		return
	}
	pt.lastUpdate = now

	percentage := float64(completedChunks) / float64(pt.totalChunks) * 100
	elapsed := now.Sub(pt.startTime)

	var speed float64
	if elapsed > 0 {
		speed = float64(bytesTransferred) / elapsed.Seconds() / (1024 * 1024) // MB/s
	}

	switch pt.style {
	case ProgressStyleDetailed:
		pt.printDetailedProgress(completedChunks, percentage, speed, "")
	case ProgressStyleMinimal:
		pt.printMinimalProgress(completedChunks, percentage)
	default:
		pt.printSimpleProgress(completedChunks, percentage, speed, "")
	}
}

// printSimpleProgress shows a clean, informative progress bar
func (pt *ProgressTracker) printSimpleProgress(completedChunks int, percentage float64, speed float64, eta string) {
	// Calculate elapsed time
	elapsed := time.Since(pt.startTime)

	// Color coding based on speed
	speedColor := Colors.Green
	if speed < 1 {
		speedColor = Colors.Yellow
	} else if speed > 20 {
		speedColor = Colors.Cyan
	}

	// Build progress bar: * for completed, spin char for current
	var progressBar strings.Builder
	for i := 0; i < pt.totalChunks; i++ {
		if i < completedChunks {
			progressBar.WriteString("*")
		} else if i == completedChunks && completedChunks < pt.totalChunks {
			spinChars := []string{"|", "/", "-", "\\"}
			progressBar.WriteString(spinChars[pt.spinIndex])
			pt.spinIndex = (pt.spinIndex + 1) % len(spinChars)
		} else {
			progressBar.WriteString(".")
		}
	}

	// Format time as mm:ss
	timeStr := fmt.Sprintf("%02d:%02d",
		int(elapsed.Minutes()),
		int(elapsed.Seconds())%60)

	// Direction indicator
	direction := "SEND"
	if pt.direction == "received" {
		direction = "RECV"
	}

	fmt.Printf("\r%s[%s%s%s] %s %.1f%% | %s%d/%d | 🚀 %s%.2fMB/s | ⏱️ %s%s%s",
		Colors.Bold,
		Colors.Cyan,
		progressBar.String(),
		Colors.Reset,
		direction,
		percentage,
		Colors.Blue,
		completedChunks,
		pt.totalChunks,
		speedColor,
		speed,
		Colors.Yellow,
		timeStr,
		Colors.Reset)
}

// printDetailedProgress shows comprehensive transfer information
func (pt *ProgressTracker) printDetailedProgress(completedChunks int, percentage float64, speed float64, eta string) {
	width := 60
	filled := int(percentage / 100 * float64(width))
	bar := strings.Repeat("━", filled) + strings.Repeat("─", width-filled)

	elapsed := time.Since(pt.startTime)

	fmt.Printf("\n%s%s Transfer Progress - %s%s\n", Colors.Bold, Colors.Cyan, pt.filename, Colors.Reset)
	fmt.Printf("%s\n", strings.Repeat("═", 80))
	fmt.Printf("  📁 File:      %s%s\n", Colors.Yellow, pt.filename, Colors.Reset)
	fmt.Printf("  📦 Size:      %s%.2f MB%s\n", Colors.Yellow, float64(pt.totalSize)/(1024*1024), Colors.Reset)
	fmt.Printf("  📊 Progress:  [%s] %s%.1f%%%s\n", bar, Colors.Bold, percentage, Colors.Reset)
	fmt.Printf("  📈 Speed:     %s%.2f MB/s%s\n", Colors.Green, speed, Colors.Reset)
	fmt.Printf("  ⏱️  Duration:  %s%v%s\n", Colors.Blue, elapsed.Round(time.Second), Colors.Reset)
	if eta != "" {
		fmt.Printf("  ⏳ ETA:       %s%s%s\n", Colors.Magenta, eta, Colors.Reset)
	}
	fmt.Printf("  📦 Chunks:    %s%d/%d%s\n", Colors.Cyan, completedChunks, pt.totalChunks, Colors.Reset)
	fmt.Printf("%s\n", strings.Repeat("═", 80))
}

// printMinimalProgress shows a compact progress indicator
func (pt *ProgressTracker) printMinimalProgress(completedChunks int, percentage float64) {
	width := 20
	filled := int(percentage / 100 * float64(width))
	bar := strings.Repeat("●", filled) + strings.Repeat("○", width-filled)

	fmt.Printf("\r%s%s %s %s%.1f%%%s",
		Colors.Bold,
		pt.filename,
		bar,
		Colors.Green,
		percentage,
		Colors.Reset)
}

// PrintSummary displays the final transfer summary
func (pt *ProgressTracker) PrintSummary(status string, errorMessage string) {
	if pt.quiet {
		return
	}

	elapsed := time.Since(pt.startTime)
	direction := "SENT"
	if pt.direction == "received" {
		direction = "RECEIVED"
	}

	statusColor := Colors.Green
	statusIcon := "✅"
	if status == "failed" || status == "rejected" {
		statusColor = Colors.Red
		statusIcon = "❌"
	}

	fmt.Printf("\n\n%s============================================================%s\n", Colors.Bold, Colors.Reset)
	fmt.Printf("%s📊 TRANSFER SUMMARY - 📤 %s%s\n", Colors.Bold, direction, Colors.Reset)
	fmt.Printf("%s============================================================%s\n", Colors.Bold, Colors.Reset)
	fmt.Printf("📁 File:           %s%s\n", Colors.Yellow, pt.filename, Colors.Reset)
	fmt.Printf("📦 Size:           %s%.2f MB%s\n", Colors.Yellow, float64(pt.totalSize)/(1024*1024), Colors.Reset)
	fmt.Printf("🔢 Chunks:         %s%d total%s\n", Colors.Cyan, pt.totalChunks, Colors.Reset)
	fmt.Printf("⏱️  Duration:       %s%v%s\n", Colors.Blue, elapsed.Round(time.Millisecond*100), Colors.Reset)
	if status == "completed" {
		speed := float64(pt.totalSize) / elapsed.Seconds() / (1024 * 1024)
		fmt.Printf("🚀 Average Speed:  %s%.2f MB/s%s\n", Colors.Green, speed, Colors.Reset)
	}
	fmt.Printf("✅ Status:         %s%s %s%s\n", statusColor, statusIcon, status, Colors.Reset)
	fmt.Printf("%s============================================================%s\n", Colors.Bold, Colors.Reset)

	if errorMessage != "" {
		fmt.Printf("❌ Error: %s%s%s\n", Colors.Red, errorMessage, Colors.Reset)
	}
}

// GetTransferStats returns current transfer statistics
func (pt *ProgressTracker) GetTransferStats() TransferStats {
	elapsed := time.Since(pt.startTime)
	var speed float64
	if elapsed > 0 {
		speed = float64(pt.totalSize) / elapsed.Seconds() / (1024 * 1024) // MB/s
	}

	return TransferStats{
		Filename:          pt.filename,
		FileSize:          pt.totalSize,
		TotalChunks:       pt.totalChunks,
		SentChunks:        pt.totalChunks, // Will be updated by caller
		StartTime:         pt.startTime,
		EndTime:           time.Time{},
		Duration:          elapsed,
		AverageSpeed:      speed,
		PeerAddress:       "",
		TransferDirection: pt.direction,
		Status:            "in_progress",
		ChunksRetried:     0,
		TotalRetries:      0,
		progressTracker:   pt,
		quiet:             false,
		lastProgressTime:  time.Now(),
		bytesTransferred:  0,
	}
}

// IsComplete checks if the transfer is complete
func (pt *ProgressTracker) IsComplete(completedChunks int) bool {
	return completedChunks >= pt.totalChunks
}