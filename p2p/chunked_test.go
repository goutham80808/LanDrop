package p2p

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"testing"
	"time"
)

func TestChunkedTransferIntegration(t *testing.T) {
	// Set test mode to avoid user input prompts
	os.Setenv("LANDROP_TEST_MODE", "1")
	defer os.Unsetenv("LANDROP_TEST_MODE")

	// Create a temporary test file
	testContent := "This is a test file for chunked transfer. " +
		"It contains enough data to be split into multiple chunks " +
		"to properly test the chunked transfer functionality. " +
		"Lorem ipsum dolor sit amet, consectetur adipiscing elit. " +
		"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua."

	testFile := "test_chunked_file.txt"

	// Write test content to file
	err := ioutil.WriteFile(testFile, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}
	defer os.Remove(testFile)

	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Test file details
	receivedFile := "received_" + testFile
	portStr := fmt.Sprintf("%d", port)
	peerAddr := fmt.Sprintf("127.0.0.1:%d", port)

	// Start receiver in goroutine
	receiverDone := make(chan error, 1)
	go func() {
		err := ReceiveFileChunked(portStr)
		if err != nil {
			receiverDone <- err
			return
		}

		// Verify the received file exists and has correct content
		receivedContent, err := ioutil.ReadFile(receivedFile)
		if err != nil {
			receiverDone <- fmt.Errorf("failed to read received file: %w", err)
			return
		}

		if string(receivedContent) != testContent {
			receiverDone <- fmt.Errorf("file content mismatch")
			return
		}

		receiverDone <- nil
	}()

	// Give receiver time to start
	time.Sleep(100 * time.Millisecond)

	// Send file
	senderErr := SendFileChunked(testFile, peerAddr)
	if senderErr != nil {
		t.Fatalf("Sender failed: %v", senderErr)
	}

	// Wait for receiver to complete
	select {
	case err := <-receiverDone:
		if err != nil {
			t.Fatalf("Receiver failed: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Test timed out")
	}

	// Clean up received file
	defer os.Remove(receivedFile)

	// Verify file integrity
	originalHash, err := calculateFileHash(testFile)
	if err != nil {
		t.Fatalf("Failed to calculate original file hash: %v", err)
	}

	if !verifyFileIntegrity(receivedFile, originalHash) {
		t.Fatal("File integrity verification failed")
	}
}

func TestGetRequiredChunks(t *testing.T) {
	// Test with non-existent file
	chunks := getRequiredChunks("nonexistent.txt", 3072, 1024)
	if len(chunks) != 3 {
		t.Errorf("Expected 3 chunks for non-existent file, got %d", len(chunks))
	}

	// Test with partial file (1.5 chunks worth)
	partialFile := "partial_test.txt"
	partialContent := string(make([]byte, 1536)) // 1.5 chunks worth of data
	for i := range partialContent {
		partialContent = partialContent[:i] + "A" + partialContent[i+1:]
	}
	err := ioutil.WriteFile(partialFile, []byte(partialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create partial test file: %v", err)
	}
	defer os.Remove(partialFile)

	// Create the "received_" version that the function looks for
	receivedPartialFile := "received_" + partialFile
	err = ioutil.WriteFile(receivedPartialFile, []byte(partialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create received partial test file: %v", err)
	}
	defer os.Remove(receivedPartialFile)

	chunks = getRequiredChunks(partialFile, 3072, 1024)
	if len(chunks) != 2 {
		t.Errorf("Expected 2 chunks for partial file, got %d", len(chunks))
	}

	// Should request chunks 1 and 2 (chunk 0 already exists)
	for i, chunk := range chunks {
		expected := i + 1
		if chunk != expected {
			t.Errorf("Expected chunk %d, got %d", expected, chunk)
		}
	}
}
