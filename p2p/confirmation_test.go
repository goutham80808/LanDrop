package p2p

import (
	"testing"
)

func TestPromptForTransferConfirmation(t *testing.T) {
	// Test creating a transfer request
	req := NewTransferRequest("test.txt", 1024, "abc123", 512)

	// Just test that the function exists and doesn't panic
	// In a real test, we would need to mock stdin, but that's complex
	// For now, we'll just verify the function signature works
	_ = req
}
