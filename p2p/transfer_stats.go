package p2p

import (
	"fmt"
	"strings"
	"time"
)

// TransferStats contains comprehensive transfer statistics
type TransferStats struct {
	Filename          string
	FileSize          int64
	TotalChunks       int
	SentChunks        int
	ReceivedChunks    int
	StartTime         time.Time
	EndTime           time.Time
	Duration          time.Duration
	AverageSpeed      float64 // in MB/s
	PeerAddress       string
	TransferDirection string // "sent" or "received"
	Status            string // "completed", "failed", "rejected"
	ChunksRetried     int    // Number of chunks that required retries
	TotalRetries      int    // Total number of retry attempts

	// Progress tracking for enhanced UI
	progressTracker   *ProgressTracker
	quiet             bool     // Disable output for testing
	lastProgressTime  time.Time
	bytesTransferred  int64    // Actual bytes transferred
}

// NewTransferStats creates a new transfer stats instance
func NewTransferStats(filename string, fileSize int64, totalChunks int, peerAddress string, direction string) *TransferStats {
	// Create progress tracker with simple style for compact real-time display
	progressTracker := NewProgressTracker(filename, fileSize, totalChunks, direction, ProgressStyleSimple)

	return &TransferStats{
		Filename:          filename,
		FileSize:          fileSize,
		TotalChunks:       totalChunks,
		SentChunks:        0,
		ReceivedChunks:    0,
		StartTime:         time.Now(),
		PeerAddress:       peerAddress,
		TransferDirection: direction,
		Status:            "in_progress",
		ChunksRetried:     0,
		TotalRetries:      0,
		progressTracker:   progressTracker,
		quiet:             false,
		lastProgressTime:  time.Now(),
		bytesTransferred:  0,
	}
}

// MarkCompleted marks the transfer as completed and calculates final stats
func (ts *TransferStats) MarkCompleted() {
	ts.EndTime = time.Now()
	ts.Duration = ts.EndTime.Sub(ts.StartTime)
	ts.Status = "completed"

	// Calculate average speed in MB/s
	if ts.Duration.Seconds() > 0 {
		bytesTransferred := float64(ts.FileSize)
		ts.AverageSpeed = bytesTransferred / ts.Duration.Seconds() / (1024 * 1024)
	}
}

// MarkFailed marks the transfer as failed
func (ts *TransferStats) MarkFailed(reason string) {
	ts.EndTime = time.Now()
	ts.Duration = ts.EndTime.Sub(ts.StartTime)
	ts.Status = "failed"
}

// MarkRejected marks the transfer as rejected
func (ts *TransferStats) MarkRejected(reason string) {
	ts.EndTime = time.Now()
	ts.Duration = ts.EndTime.Sub(ts.StartTime)
	ts.Status = "rejected"
}

// IncrementSentChunks increments the count of sent chunks
func (ts *TransferStats) IncrementSentChunks() {
	ts.SentChunks++
}

// IncrementReceivedChunks increments the count of received chunks
func (ts *TransferStats) IncrementReceivedChunks() {
	ts.ReceivedChunks++
}

// AddRetry adds retry statistics
func (ts *TransferStats) AddRetry(chunkIndex int, retryCount int) {
	if retryCount > 1 {
		ts.ChunksRetried++
		ts.TotalRetries += (retryCount - 1)
	}
}

// GetProgressPercentage returns the progress percentage
func (ts *TransferStats) GetProgressPercentage() float64 {
	if ts.TransferDirection == "sent" {
		if ts.TotalChunks == 0 {
			return 0
		}
		return float64(ts.SentChunks) / float64(ts.TotalChunks) * 100
	} else {
		if ts.TotalChunks == 0 {
			return 0
		}
		return float64(ts.ReceivedChunks) / float64(ts.TotalChunks) * 100
	}
}

// PrintSummary prints a detailed summary of the transfer
func (ts *TransferStats) PrintSummary() {
	if ts.progressTracker != nil {
		// Use the beautiful progress tracker summary
		ts.progressTracker.PrintSummary(ts.Status, "")
	} else {
		// Fallback to basic summary
		fmt.Println("\n" + strings.Repeat("=", 60))
		fmt.Printf("📊 TRANSFER SUMMARY - %s\n", ts.getDirectionEmoji())
		fmt.Println(strings.Repeat("=", 60))

		fmt.Printf("📁 File:           %s\n", ts.Filename)
		fmt.Printf("📦 Size:           %.2f MB\n", float64(ts.FileSize)/(1024*1024))
		fmt.Printf("🔢 Chunks:         %d total", ts.TotalChunks)

		if ts.TransferDirection == "sent" {
			fmt.Printf(" (%d sent)\n", ts.SentChunks)
		} else {
			fmt.Printf(" (%d received)\n", ts.ReceivedChunks)
		}

		fmt.Printf("🌐 Peer:           %s\n", ts.PeerAddress)
		fmt.Printf("⏱️  Duration:       %.2f seconds\n", ts.Duration.Seconds())
		fmt.Printf("🚀 Average Speed:  %.2f MB/s\n", ts.AverageSpeed)
		fmt.Printf("✅ Status:         %s\n", ts.getStatusEmoji()+" "+ts.Status)

		if ts.ChunksRetried > 0 {
			fmt.Printf("🔄 Retries:        %d chunks retried (%d total attempts)\n", ts.ChunksRetried, ts.TotalRetries)
		}

		fmt.Println(strings.Repeat("=", 60))
	}
}

// getDirectionEmoji returns appropriate emoji for transfer direction
func (ts *TransferStats) getDirectionEmoji() string {
	if ts.TransferDirection == "sent" {
		return "📤 SENT"
	}
	return "📥 RECEIVED"
}

// getStatusEmoji returns appropriate emoji for status
func (ts *TransferStats) getStatusEmoji() string {
	switch ts.Status {
	case "completed":
		return "✅"
	case "failed":
		return "❌"
	case "rejected":
		return "🚫"
	default:
		return "⏳"
	}
}

// PrintProgress prints current progress with enhanced UI
func (ts *TransferStats) PrintProgress() {
	if ts.quiet || ts.progressTracker == nil {
		return
	}

	// Throttle progress updates to avoid flickering
	now := time.Now()
	if now.Sub(ts.lastProgressTime) < 100*time.Millisecond {
		return
	}
	ts.lastProgressTime = now

	completedChunks := ts.SentChunks
	if ts.TransferDirection == "received" {
		completedChunks = ts.ReceivedChunks
	}

	// Use the progress tracker for beautiful output
	ts.progressTracker.PrintProgress(completedChunks, ts.bytesTransferred)
}

// SetQuiet disables progress output
func (ts *TransferStats) SetQuiet(quiet bool) {
	ts.quiet = quiet
	if ts.progressTracker != nil {
		ts.progressTracker.SetQuiet(quiet)
	}
}

// UpdateBytesTransferred updates the actual bytes transferred
func (ts *TransferStats) UpdateBytesTransferred(bytes int64) {
	ts.bytesTransferred = bytes
}

// GetProgressTracker returns the internal progress tracker
func (ts *TransferStats) GetProgressTracker() *ProgressTracker {
	return ts.progressTracker
}
