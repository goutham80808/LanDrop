package p2p

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// FileMetadata remains the same.
type FileMetadata struct {
	Filename string `json:"filename"`
	FileSize int64  `json:"filesize"`
	FileHash string `json:"filehash"`
}

// ResumeResponse is sent from the receiver to the sender.
type ResumeResponse struct {
	Offset int64 `json:"offset"`
}

// SendFile handles the logic for sending a file with resume capability.
func SendFile(filename string, peerAddr string) {
	// 1. Get file info and calculate total file hash.
	file, err := os.Open(filename)
	if err != nil {
		fmt.Printf("Error opening file: %s\n", err)
		return
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		fmt.Printf("Error getting file info: %s\n", err)
		return
	}

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		fmt.Printf("Error calculating file hash: %s\n", err)
		return
	}
	file.Seek(0, 0) // Reset for sending

	metadata := FileMetadata{
		Filename: filepath.Base(filename),
		FileSize: fileInfo.Size(),
		FileHash: hex.EncodeToString(hash.Sum(nil)),
	}

	// 2. Connect and send initial metadata.
	conn, err := net.Dial("tcp", peerAddr)
	if err != nil {
		fmt.Printf("Error connecting to peer: %s\n", err)
		return
	}
	defer conn.Close()

	writer := bufio.NewWriter(conn)
	reader := bufio.NewReader(conn)

	metadataBytes, _ := json.Marshal(metadata)
	writer.Write(metadataBytes)
	writer.WriteByte('\n')
	writer.Flush()

	// 3. Wait for the receiver's resume response.
	responseBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Printf("Error receiving resume response: %s\n", err)
		return
	}

	var response ResumeResponse
	if err := json.Unmarshal(responseBytes, &response); err != nil {
		fmt.Printf("Error parsing resume response: %s\n", err)
		return
	}

	// 4. Seek to the required offset and start streaming.
	if response.Offset > 0 {
		fmt.Printf("Peer has %.2f MB already. Resuming transfer...\n", float64(response.Offset)/(1024*1024))
		_, err = file.Seek(response.Offset, io.SeekStart)
		if err != nil {
			fmt.Printf("Error seeking file: %s\n", err)
			return
		}
	}

	fmt.Printf("Sending file '%s'...\n", metadata.Filename)
	startTime := time.Now()

	bytesSent, err := io.Copy(writer, file)
	if err != nil {
		fmt.Printf("Error sending file data: %s\n", err)
		return
	}
	writer.Flush()

	duration := time.Since(startTime)
	speed := float64(bytesSent) / duration.Seconds() / (1024 * 1024)

	// 5. Wait for final ACK from receiver.
	status, err := reader.ReadString('\n')
	if err != nil {
		fmt.Printf("Error waiting for final ack: %s\n", err)
		return
	}

	fmt.Println("\n--- Transfer Result ---")
	fmt.Printf("File: %s\n", metadata.Filename)
	fmt.Printf("Speed: %.2f MB/s\n", speed)
	if strings.TrimSpace(status) == "ACK" {
		fmt.Println("Status: SUCCESS (Verified by peer)")
	} else {
		fmt.Printf("Status: FAILED (Peer reported error)\n")
	}
	fmt.Println("-----------------------")
}

// ReceiveFile handles listening and receiving a file with resume capability.
func ReceiveFile(port string) {
	// Start discovery listener in background
	go ListenForDiscovery(port)
	
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		fmt.Printf("Error listening on port %s: %s\n", port, err)
		return
	}
	defer listener.Close()

	fmt.Printf("Listening for incoming files on port %s...\n", port)

	conn, err := listener.Accept()
	if err != nil {
		fmt.Printf("Error accepting connection: %s\n", err)
		return
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// 1. Read initial metadata.
	metadataBytes, err := reader.ReadBytes('\n')
	if err != nil {
		fmt.Printf("Error reading metadata: %s\n", err)
		return
	}

	var metadata FileMetadata
	json.Unmarshal(metadataBytes, &metadata)

	// 2. Check for existing partial file and determine offset.
	var offset int64
	if _, err := os.Stat(metadata.Filename); err == nil {
		// File exists, get its size.
		fileInfo, _ := os.Stat(metadata.Filename)
		offset = fileInfo.Size()
		fmt.Printf("Partial file '%s' found with size %.2f MB. Requesting resume.\n", metadata.Filename, float64(offset)/(1024*1024))
	}

	// 3. Send the resume response back to the sender.
	response := ResumeResponse{Offset: offset}
	responseBytes, _ := json.Marshal(response)
	writer.Write(responseBytes)
	writer.WriteByte('\n')
	writer.Flush()

	// 4. Open file for appending/writing.
	// O_CREATE: create if not exists, O_APPEND|O_WRONLY: append in write-only mode.
	file, err := os.OpenFile(metadata.Filename, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error opening file for writing: %s\n", err)
		return
	}
	defer file.Close()

	// 5. Read the rest of the file data.
	bytesToReceive := metadata.FileSize - offset
	fmt.Printf("Receiving '%.2f' MB...\n", float64(bytesToReceive)/(1024*1024))
	startTime := time.Now()

	bytesReceived, err := io.CopyN(file, reader, bytesToReceive)
	if err != nil {
		fmt.Printf("Error receiving file data: %s\n", err)
		return
	}

	duration := time.Since(startTime)
	speed := float64(bytesReceived) / duration.Seconds() / (1024 * 1024)

	// 6. Verify hash of the completed file.
	fmt.Println("Verifying integrity...")
	// We MUST re-open the file in read mode to hash it from the beginning.
	// Note: file.Close() is handled by defer at function exit
	receivedHash, _ := calculateFileHash(metadata.Filename)

	// 7. Send final ACK/ERR and log results.
	if receivedHash == metadata.FileHash {
		writer.WriteString("ACK\n")
		writer.Flush()
		fmt.Println("\n--- Transfer Complete ---")
		fmt.Printf("File: %s\n", metadata.Filename)
		fmt.Printf("Time: %.2fs (%.2f MB/s)\n", duration.Seconds(), speed)
		fmt.Println("Integrity: SUCCESS ✅")
	} else {
		writer.WriteString("ERR_CHECKSUM\n")
		writer.Flush()
		fmt.Println("Integrity: FAILED ❌")
	}
	fmt.Println("-------------------------")
}

// calculateFileHash helper remains unchanged.
func calculateFileHash(filename string) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}
