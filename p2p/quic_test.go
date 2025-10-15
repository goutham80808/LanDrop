package p2p

import (
	"fmt"
	"net"
	"testing"
	"time"
)

func TestQUICConnection(t *testing.T) {
	// Find an available port
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find available port: %v", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	// Test message
	testMessage := "Hello, QUIC!"

	// Start receiver in goroutine
	receiverDone := make(chan error, 1)
	go func() {
		err := ReceiveQUICMessage(fmt.Sprintf("%d", port))
		receiverDone <- err
	}()

	// Give receiver time to start
	time.Sleep(100 * time.Millisecond)

	// Send message
	senderErr := SendQUICMessage(fmt.Sprintf("127.0.0.1:%d", port), testMessage)
	if senderErr != nil {
		t.Fatalf("Sender failed: %v", senderErr)
	}

	// Wait for receiver to complete
	select {
	case err := <-receiverDone:
		if err != nil {
			t.Fatalf("Receiver failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Receiver timed out")
	}
}
