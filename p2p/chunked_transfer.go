package p2p

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/quic-go/quic-go"
)



// createStreamContext creates a context with timeout for stream operations
func createStreamContext(parentCtx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parentCtx, StreamTimeout)
}

// sendChunkWithRetry sends a single chunk using the reliable protocol
func sendChunkWithRetry(ctx context.Context, conn quic.Connection, file *os.File, chunkIndex int, offset, size int64) error {
	var lastErr error

	for attempt := 0; attempt < MaxRetries; attempt++ {
		if attempt > 0 {
			fmt.Printf("\nRetrying chunk %d (attempt %d/%d)...", chunkIndex, attempt+1, MaxRetries)
		}

		// Seek to chunk position
		_, err := file.Seek(offset, io.SeekStart)
		if err != nil {
			lastErr = fmt.Errorf("failed to seek to chunk %d: %w", chunkIndex, err)
			continue
		}

		// Read chunk data from file using buffer pool for large chunks
		var chunkData []byte
		if size <= ChunkBufferSize {
			chunkData = ChunkBufferPool.Get()
			defer ChunkBufferPool.Put(chunkData)
		} else {
			chunkData = make([]byte, size)
		}

		bytesRead, err := io.ReadFull(file, chunkData[:size])
		if err != nil {
			lastErr = fmt.Errorf("failed to read chunk %d from file: %w", chunkIndex, err)
			continue
		}

		// Verify we read exactly what we expected
		if int64(bytesRead) != size {
			lastErr = fmt.Errorf("chunk %d file read mismatch: expected %d, got %d", chunkIndex, size, bytesRead)
			continue
		}

		// Send chunk using reliable protocol
		err = sendChunkReliably(ctx, conn, chunkIndex, chunkData[:bytesRead])
		if err != nil {
			lastErr = fmt.Errorf("failed to send chunk %d reliably: %w", chunkIndex, err)
			continue
		}

		// Successfully sent chunk
		return nil
	}

	return lastErr
}

// sendChunkReliably sends a chunk using fast binary protocol
func sendChunkReliably(ctx context.Context, conn quic.Connection, chunkIndex int, data []byte) error {
	// Open stream for this chunk
	streamCtx, streamCancel := createStreamContext(ctx)
	chunkStream, err := conn.OpenStreamSync(streamCtx)
	if err != nil {
		streamCancel()
		return fmt.Errorf("failed to open chunk stream: %w", err)
	}
	defer chunkStream.Close()
	defer streamCancel()

	// Create simple binary header: [chunkIndex(4 bytes)][dataSize(4 bytes)][checksum(32 bytes)]
	header := make([]byte, 40)
	binary.BigEndian.PutUint32(header[0:4], uint32(chunkIndex))
	binary.BigEndian.PutUint32(header[4:8], uint32(len(data)))

	// Calculate SHA-256 checksum
	hash := sha256.Sum256(data)
	copy(header[8:40], hash[:])

	// Send header
	_, err = chunkStream.Write(header)
	if err != nil {
		return fmt.Errorf("failed to write chunk header: %w", err)
	}

	// Send data directly (no JSON overhead)
	_, err = chunkStream.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write chunk data: %w", err)
	}

	// Wait for simple acknowledgment (1 byte: 1=success, 0=failure)
	ack := make([]byte, 1)
	_, err = chunkStream.Read(ack)
	if err != nil {
		return fmt.Errorf("failed to read chunk acknowledgment: %w", err)
	}

	// Check if chunk was received successfully
	if ack[0] != 1 {
		return fmt.Errorf("chunk %d was not received successfully", chunkIndex)
	}

	return nil
}

// receiveChunkReliably receives a chunk using fast binary protocol
func receiveChunkReliably(ctx context.Context, chunkStream quic.Stream, expectedChunkIndex int) (*ChunkData, error) {
	// Read binary header (40 bytes)
	header := make([]byte, 40)
	_, err := io.ReadFull(chunkStream, header)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk header: %w", err)
	}

	// Parse header
	receivedChunkIndex := int(binary.BigEndian.Uint32(header[0:4]))
	dataSize := int(binary.BigEndian.Uint32(header[4:8]))
	receivedChecksum := header[8:40]

	// Verify chunk index matches expected
	if receivedChunkIndex != expectedChunkIndex {
		return nil, fmt.Errorf("received chunk index %d, expected %d", receivedChunkIndex, expectedChunkIndex)
	}

	// Read data
	data := make([]byte, dataSize)
	_, err = io.ReadFull(chunkStream, data)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk data: %w", err)
	}

	// Verify checksum
	hash := sha256.Sum256(data)
	if !bytes.Equal(hash[:], receivedChecksum) {
		return nil, fmt.Errorf("chunk %d checksum verification failed", expectedChunkIndex)
	}

	// Send success acknowledgment (1 byte)
	_, err = chunkStream.Write([]byte{1})
	if err != nil {
		// Non-fatal error, just log it
		fmt.Printf("Warning: failed to send acknowledgment for chunk %d: %v\n", expectedChunkIndex, err)
	}

	// Return chunk data in the expected format for compatibility
	return &ChunkData{
		Type:       MessageChunkData,
		ChunkIndex: receivedChunkIndex,
		ChunkSize:  dataSize,
		Data:       data,
		Checksum:   hex.EncodeToString(receivedChecksum),
	}, nil
}

// SendFileChunked sends a file using the new chunked QUIC protocol
func SendFileChunked(filename string, peerAddr string) error {
	// Longer timeout for large files and network delays
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	// Get file info and calculate hash
	file, err := os.Open(filename)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("failed to get file info: %w", err)
	}

	// Calculate file hash
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("failed to calculate file hash: %w", err)
	}
	file.Seek(0, 0) // Reset for reading

	fileHash := hex.EncodeToString(hash.Sum(nil))
	chunkSize := DefaultChunkSize
	totalChunks := (fileInfo.Size() + chunkSize - 1) / chunkSize

	fmt.Printf("Preparing to send '%s' (%.2f MB, %d chunks) to %s\n",
		fileInfo.Name(),
		float64(fileInfo.Size())/(1024*1024),
		totalChunks,
		peerAddr)

	// Initialize transfer statistics
	stats := NewTransferStats(fileInfo.Name(), fileInfo.Size(), int(totalChunks), peerAddr, "sent")

	// Get client TLS config
	tlsConfig := GetClientTLSConfig()

	// Dial QUIC connection
	conn, err := quic.DialAddr(ctx, peerAddr, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC: %w", err)
	}
	defer conn.CloseWithError(0, "")

	// Open control stream for metadata exchange
	controlStream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("failed to open control stream: %w", err)
	}

	// Send transfer request
	request := NewTransferRequest(
		filepath.Base(filename),
		fileInfo.Size(),
		fileHash,
		chunkSize,
	)

	requestData, err := SerializeMessage(request)
	if err != nil {
		return fmt.Errorf("failed to serialize transfer request: %w", err)
	}

	_, err = controlStream.Write(requestData)
	if err != nil {
		return fmt.Errorf("failed to send transfer request: %w", err)
	}

	// Ensure the request is sent immediately
	if flusher, ok := controlStream.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return fmt.Errorf("failed to flush transfer request: %w", err)
		}
	}

	fmt.Println("Transfer request sent, waiting for response...")

	// Read response from control stream with dynamic buffering
	var responseBuffer []byte
	buf := make([]byte, 4096)
	for {
		n, err := controlStream.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read transfer response: %w", err)
		}
		responseBuffer = append(responseBuffer, buf[:n]...)

		// Try to parse the response to see if we have a complete message
		if _, err := DeserializeTransferResponse(responseBuffer); err == nil {
			break // Complete message received
		}
	}

	response, err := DeserializeTransferResponse(responseBuffer)
	if err != nil {
		return fmt.Errorf("failed to deserialize transfer response: %w", err)
	}

	if !response.Accepted {
		stats.MarkRejected(response.RejectionMsg)
		stats.PrintSummary()
		fmt.Printf("Transfer rejected: %s\n", response.RejectionMsg)
		return nil // Return nil instead of error since rejection is a normal outcome
	}

	fmt.Printf("Transfer accepted! Need to send %d chunks.\n", len(response.ResumeChunks))
	stats.TotalChunks = len(response.ResumeChunks) // Update to only required chunks

	// Send required chunks with improved error handling and progress tracking
	for i, chunkIndex := range response.ResumeChunks {
		offset := int64(chunkIndex) * chunkSize
		remaining := fileInfo.Size() - offset
		if remaining <= 0 {
			stats.IncrementSentChunks() // Skip empty chunks
			continue
		}

		if remaining > chunkSize {
			remaining = chunkSize
		}

		// Debug logging for first few chunks
		if i < 3 {
			fmt.Printf("DEBUG SENDER: Chunk %d - offset: %d, remaining: %d, fileInfo.Size: %d\n",
				chunkIndex, offset, remaining, fileInfo.Size())
		}

		// Send chunk with retry logic using array index for synchronization
		err := sendChunkWithRetry(ctx, conn, file, i, offset, remaining)
		if err != nil {
			stats.MarkFailed(fmt.Sprintf("failed to send chunk %d: %v", chunkIndex, err))
			stats.PrintSummary()
			return fmt.Errorf("failed to send chunk %d: %w", chunkIndex, err)
		}

		// Increment sent chunks and print progress
		stats.IncrementSentChunks()
		stats.PrintProgress()

		// Optimize transfer speed consistency with adaptive pacing
		if (i+1)%50 == 0 {
			fmt.Printf("\n🔄 Connection health check at chunk %d/%d", i+1, len(response.ResumeChunks))
			time.Sleep(50 * time.Millisecond) // Reduced pause for better throughput
		} else if (i+1)%10 == 0 {
			// Very brief pause every 10 chunks to allow system I/O to stabilize
			time.Sleep(2 * time.Millisecond)
		}
		// No delay for other chunks to maintain consistent speed
	}

	// Give the receiver time to process the last chunk
	time.Sleep(100 * time.Millisecond)

	fmt.Printf("\nTransfer completed successfully!\n")

	// Mark transfer as completed and print final statistics
	stats.MarkCompleted()
	fmt.Println() // New line after progress
	stats.PrintSummary()

	return nil
}

// ReceiveFileChunked receives a file using the new chunked QUIC protocol
func ReceiveFileChunked(port string) error {
	// Get server TLS config
	tlsConfig := GetServerTLSConfig()
	if tlsConfig == nil {
		return fmt.Errorf("failed to get server TLS config")
	}

	// Create UDP listener
	udpAddr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	udpConn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %w", err)
	}
	defer udpConn.Close()

	fmt.Printf("Listening for chunked QUIC transfers on port %s...\n", port)

	// Create QUIC listener
	listener, err := quic.Listen(udpConn, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to create QUIC listener: %w", err)
	}
	defer listener.Close()

	// Accept connection with longer timeout for large files
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Minute)
	defer cancel()

	conn, err := listener.Accept(ctx)
	if err != nil {
		return fmt.Errorf("failed to accept QUIC connection: %w", err)
	}
	defer conn.CloseWithError(0, "")

	// Accept control stream
	controlStream, err := conn.AcceptStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to accept control stream: %w", err)
	}

	// Read transfer request with dynamic buffering
	var requestBuffer []byte
	buf := make([]byte, 4096)
	for {
		n, err := controlStream.Read(buf)
		if err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("failed to read transfer request: %w", err)
		}
		requestBuffer = append(requestBuffer, buf[:n]...)

		// Try to parse the request to see if we have a complete message
		if _, err := DeserializeTransferRequest(requestBuffer); err == nil {
			break // Complete message received
		}
	}

	request, err := DeserializeTransferRequest(requestBuffer)
	if err != nil {
		return fmt.Errorf("failed to deserialize transfer request: %w", err)
	}

	fmt.Printf("Received transfer request for '%s' (%.2f MB)\n",
		request.Filename,
		float64(request.FileSize)/(1024*1024))

	// Prompt user for confirmation
	accepted, rejectionMsg := promptForTransferConfirmation(request)

	response := NewTransferResponse(accepted, getRequiredChunks(request.Filename, request.FileSize, request.ChunkSize), rejectionMsg)

	// Initialize transfer statistics
	peerAddr := conn.RemoteAddr().String()
	totalChunks := int((request.FileSize + request.ChunkSize - 1) / request.ChunkSize)
	stats := NewTransferStats(request.Filename, request.FileSize, totalChunks, peerAddr, "received")

	responseData, err := SerializeMessage(response)
	if err != nil {
		return fmt.Errorf("failed to serialize transfer response: %w", err)
	}

	// Send response with proper flushing
	_, err = controlStream.Write(responseData)
	if err != nil {
		return fmt.Errorf("failed to send transfer response: %w", err)
	}

	// Ensure the response is sent immediately
	if flusher, ok := controlStream.(interface{ Flush() error }); ok {
		if err := flusher.Flush(); err != nil {
			return fmt.Errorf("failed to flush transfer response: %w", err)
		}
	}

	// Wait a moment to ensure the response is sent and received
	time.Sleep(50 * time.Millisecond)

	if !accepted {
		stats.MarkRejected(rejectionMsg)
		stats.PrintSummary()
		return fmt.Errorf("transfer rejected: %s", rejectionMsg)
	}

	fmt.Printf("Accepting transfer with %d chunks to receive\n", len(response.ResumeChunks))
	stats.TotalChunks = len(response.ResumeChunks) // Update to only required chunks

	// Create output file with prefix to avoid conflicts
	outputFilename := "received_" + request.Filename
	outputFile, err := os.OpenFile(outputFilename, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outputFile.Close()

	// Receive chunks using the reliable chunk protocol
	for i := 0; i < len(response.ResumeChunks); i++ {
		chunkIndex := response.ResumeChunks[i]

		// Accept chunk stream with timeout
		streamCtx, streamCancel := createStreamContext(ctx)
		chunkStream, err := conn.AcceptStream(streamCtx)
		if err != nil {
			stats.MarkFailed(fmt.Sprintf("failed to accept chunk stream %d: %v", i, err))
			stats.PrintSummary()
			streamCancel()
			return fmt.Errorf("failed to accept chunk stream %d: %w", i, err)
		}
		streamCancel()

		// Receive chunk reliably using array index for synchronization
		receivedChunk, err := receiveChunkReliably(ctx, chunkStream, i)
		if err != nil {
			stats.MarkFailed(fmt.Sprintf("failed to receive chunk %d: %v", chunkIndex, err))
			stats.PrintSummary()
			return fmt.Errorf("failed to receive chunk %d: %w", chunkIndex, err)
		}

		// Calculate offset for this chunk
		offset := int64(chunkIndex) * request.ChunkSize

		// Write chunk to file
		_, err = outputFile.WriteAt(receivedChunk.Data, offset)
		if err != nil {
			stats.MarkFailed(fmt.Sprintf("failed to write chunk %d: %v", chunkIndex, err))
			stats.PrintSummary()
			return fmt.Errorf("failed to write chunk %d: %w", chunkIndex, err)
		}

		// Close chunk stream
		chunkStream.Close()

		// Increment received chunks and print progress
		stats.IncrementReceivedChunks()
		stats.PrintProgress()

		// Optimize receiver speed with adaptive pacing
		if (i+1)%50 == 0 {
			fmt.Printf("\n🔄 Connection health check at chunk %d/%d", i+1, len(response.ResumeChunks))
			time.Sleep(50 * time.Millisecond) // Reduced pause for better throughput
		} else if (i+1)%10 == 0 {
			// Very brief pause every 10 chunks to allow system I/O to stabilize
			time.Sleep(2 * time.Millisecond)
		}
		// No delay for other chunks to maintain consistent speed
	}

	fmt.Printf("\nFile transfer completed: %s\n", outputFilename)

	// Verify file integrity
	fmt.Println("Verifying file integrity...")
	outputFile.Close() // Close before reading for hash verification

	if verifyFileIntegrity(outputFilename, request.FileHash) {
		// Mark transfer as completed and print final statistics
		stats.MarkCompleted()
		fmt.Println() // New line after progress
		stats.PrintSummary()
		fmt.Println("✅ File integrity verified - transfer successful!")
	} else {
		stats.MarkFailed("file integrity verification failed")
		stats.PrintSummary()
		fmt.Printf("❌ File integrity check failed!\n")
		return fmt.Errorf("file integrity verification failed")
	}

	return nil
}

// getRequiredChunks determines which chunks need to be received based on existing file
func getRequiredChunks(filename string, fileSize int64, chunkSize int64) []int {
	totalChunks := (fileSize + chunkSize - 1) / chunkSize
	requiredChunks := make([]int, 0, totalChunks)

	// Check if file exists and get its size (check for "received_" prefix version)
	receivedFilename := "received_" + filename
	if info, err := os.Stat(receivedFilename); err == nil {
		existingSize := info.Size()
		existingChunks := existingSize / chunkSize

		// Only include chunks that are not already present
		for i := int64(0); i < totalChunks; i++ {
			if i >= existingChunks {
				requiredChunks = append(requiredChunks, int(i))
			}
		}
	} else {
		// File doesn't exist, need all chunks
		for i := int64(0); i < totalChunks; i++ {
			requiredChunks = append(requiredChunks, int(i))
		}
	}

	return requiredChunks
}

// verifyFileIntegrity calculates the SHA256 hash of a file and compares it with expected hash
func verifyFileIntegrity(filename string, expectedHash string) bool {
	file, err := os.Open(filename)
	if err != nil {
		return false
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return false
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))
	return actualHash == expectedHash
}

// promptForTransferConfirmation asks the user to accept or reject a file transfer
func promptForTransferConfirmation(request *TransferRequest) (bool, string) {
	// Check if we're in test mode (environment variable)
	if os.Getenv("LANDROP_TEST_MODE") == "1" {
		fmt.Println("(Test mode: automatically accepting transfer)")
		return true, ""
	}
	// Get sender's hostname (this is a simplified approach)
	hostname := "Unknown"
	if h, err := os.Hostname(); err == nil {
		hostname = h
	}

	fmt.Printf("\n--- Incoming Transfer Request ---\n")
	fmt.Printf("From: %s\n", hostname)
	fmt.Printf("File: %s\n", request.Filename)
	fmt.Printf("Size: %.2f MB\n", float64(request.FileSize)/(1024*1024))
	fmt.Printf("Hash: %s\n", request.FileHash)
	fmt.Println("--------------------------------")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Accept this transfer? (yes/no): ")

	response, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return false, "Error reading user response"
	}

	response = strings.TrimSpace(strings.ToLower(response))

	switch response {
	case "yes", "y":
		fmt.Println("Transfer accepted.")
		return true, ""
	case "no", "n":
		fmt.Println("Transfer rejected.")
		return false, "User rejected the transfer"
	default:
		fmt.Println("Invalid response. Transfer rejected.")
		return false, "User provided invalid response"
	}
}
