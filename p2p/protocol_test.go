package p2p

import (
	"testing"
)

func TestTransferRequestSerialization(t *testing.T) {
	// Create a transfer request
	req := NewTransferRequest("test.txt", 1024, "abc123", 512)

	// Serialize
	data, err := SerializeMessage(req)
	if err != nil {
		t.Fatalf("Failed to serialize transfer request: %v", err)
	}

	// Deserialize
	deserializedReq, err := DeserializeTransferRequest(data)
	if err != nil {
		t.Fatalf("Failed to deserialize transfer request: %v", err)
	}

	// Verify
	if deserializedReq.Type != MessageTransferRequest {
		t.Errorf("Expected type %s, got %s", MessageTransferRequest, deserializedReq.Type)
	}

	if deserializedReq.Filename != "test.txt" {
		t.Errorf("Expected filename test.txt, got %s", deserializedReq.Filename)
	}

	if deserializedReq.FileSize != 1024 {
		t.Errorf("Expected file size 1024, got %d", deserializedReq.FileSize)
	}

	if deserializedReq.FileHash != "abc123" {
		t.Errorf("Expected file hash abc123, got %s", deserializedReq.FileHash)
	}

	if deserializedReq.ChunkSize != 512 {
		t.Errorf("Expected chunk size 512, got %d", deserializedReq.ChunkSize)
	}
}

func TestTransferResponseSerialization(t *testing.T) {
	// Create a transfer response
	resumeChunks := []int{0, 1, 3, 4}
	resp := NewTransferResponse(true, resumeChunks, "")

	// Serialize
	data, err := SerializeMessage(resp)
	if err != nil {
		t.Fatalf("Failed to serialize transfer response: %v", err)
	}

	// Deserialize
	deserializedResp, err := DeserializeTransferResponse(data)
	if err != nil {
		t.Fatalf("Failed to deserialize transfer response: %v", err)
	}

	// Verify
	if deserializedResp.Type != MessageTransferResponse {
		t.Errorf("Expected type %s, got %s", MessageTransferResponse, deserializedResp.Type)
	}

	if !deserializedResp.Accepted {
		t.Errorf("Expected accepted true, got %v", deserializedResp.Accepted)
	}

	if len(deserializedResp.ResumeChunks) != 4 {
		t.Errorf("Expected 4 resume chunks, got %d", len(deserializedResp.ResumeChunks))
	}

	for i, chunk := range deserializedResp.ResumeChunks {
		if chunk != resumeChunks[i] {
			t.Errorf("Expected resume chunk %d, got %d", resumeChunks[i], chunk)
		}
	}
}

func TestTransferResponseRejection(t *testing.T) {
	// Create a rejection response
	resp := NewTransferResponse(false, nil, "User rejected transfer")

	// Serialize
	data, err := SerializeMessage(resp)
	if err != nil {
		t.Fatalf("Failed to serialize transfer response: %v", err)
	}

	// Deserialize
	deserializedResp, err := DeserializeTransferResponse(data)
	if err != nil {
		t.Fatalf("Failed to deserialize transfer response: %v", err)
	}

	// Verify
	if deserializedResp.Accepted {
		t.Errorf("Expected accepted false, got %v", deserializedResp.Accepted)
	}

	if deserializedResp.RejectionMsg != "User rejected transfer" {
		t.Errorf("Expected rejection message 'User rejected transfer', got %s", deserializedResp.RejectionMsg)
	}

	if len(deserializedResp.ResumeChunks) != 0 {
		t.Errorf("Expected 0 resume chunks for rejection, got %d", len(deserializedResp.ResumeChunks))
	}
}
