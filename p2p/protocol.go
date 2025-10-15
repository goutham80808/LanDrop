package p2p

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// MessageType represents the type of protocol message
type MessageType string

const (
	MessageTransferRequest  MessageType = "TRANSFER_REQUEST"
	MessageTransferResponse MessageType = "TRANSFER_RESPONSE"
	MessageChunkData        MessageType = "CHUNK_DATA"
	MessageChunkAck         MessageType = "CHUNK_ACK"
)

// TransferRequest is sent from client to server to initiate a file transfer
type TransferRequest struct {
	Type      MessageType `json:"type"`
	Filename  string      `json:"filename"`
	FileSize  int64       `json:"filesize"`
	FileHash  string      `json:"filehash"`
	ChunkSize int64       `json:"chunk_size"`
}

// TransferResponse is sent from server to client to acknowledge a transfer request
type TransferResponse struct {
	Type         MessageType `json:"type"`
	Accepted     bool        `json:"accepted"`
	ResumeChunks []int       `json:"resume_chunks,omitempty"`
	RejectionMsg string      `json:"rejection_msg,omitempty"`
}

// ProtocolMessage represents any protocol message
type ProtocolMessage struct {
	TransferRequest  *TransferRequest
	TransferResponse *TransferResponse
}

// SerializeMessage serializes a protocol message to JSON
func SerializeMessage(msg interface{}) ([]byte, error) {
	data, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize message: %w", err)
	}
	return data, nil
}

// DeserializeTransferRequest deserializes a TRANSFER_REQUEST message
func DeserializeTransferRequest(data []byte) (*TransferRequest, error) {
	var req TransferRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return nil, fmt.Errorf("failed to deserialize transfer request: %w", err)
	}

	if req.Type != MessageTransferRequest {
		return nil, fmt.Errorf("invalid message type: expected %s, got %s", MessageTransferRequest, req.Type)
	}

	return &req, nil
}

// DeserializeTransferResponse deserializes a TRANSFER_RESPONSE message
func DeserializeTransferResponse(data []byte) (*TransferResponse, error) {
	var resp TransferResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("failed to deserialize transfer response: %w", err)
	}

	if resp.Type != MessageTransferResponse {
		return nil, fmt.Errorf("invalid message type: expected %s, got %s", MessageTransferResponse, resp.Type)
	}

	return &resp, nil
}

// NewTransferRequest creates a new transfer request message
func NewTransferRequest(filename string, fileSize int64, fileHash string, chunkSize int64) *TransferRequest {
	return &TransferRequest{
		Type:      MessageTransferRequest,
		Filename:  filename,
		FileSize:  fileSize,
		FileHash:  fileHash,
		ChunkSize: chunkSize,
	}
}

// NewTransferResponse creates a new transfer response message
func NewTransferResponse(accepted bool, resumeChunks []int, rejectionMsg string) *TransferResponse {
	return &TransferResponse{
		Type:         MessageTransferResponse,
		Accepted:     accepted,
		ResumeChunks: resumeChunks,
		RejectionMsg: rejectionMsg,
	}
}

// ChunkData represents a chunk of file data with metadata
type ChunkData struct {
	Type       MessageType `json:"type"`
	ChunkIndex int         `json:"chunk_index"`
	ChunkSize  int         `json:"chunk_size"`
	Data       []byte      `json:"data"`
	Checksum   string      `json:"checksum"`
}

// ChunkAck represents acknowledgment of a received chunk
type ChunkAck struct {
	Type       MessageType `json:"type"`
	ChunkIndex int         `json:"chunk_index"`
	Received   bool        `json:"received"`
	ErrorMsg   string      `json:"error_msg,omitempty"`
}

// NewChunkData creates a new chunk data message
func NewChunkData(chunkIndex int, data []byte) *ChunkData {
	hash := sha256.Sum256(data)
	return &ChunkData{
		Type:       MessageChunkData,
		ChunkIndex: chunkIndex,
		ChunkSize:  len(data),
		Data:       data,
		Checksum:   hex.EncodeToString(hash[:]),
	}
}

// NewChunkAck creates a new chunk acknowledgment message
func NewChunkAck(chunkIndex int, received bool, errorMsg string) *ChunkAck {
	return &ChunkAck{
		Type:       MessageChunkAck,
		ChunkIndex: chunkIndex,
		Received:   received,
		ErrorMsg:   errorMsg,
	}
}

// DeserializeChunkData deserializes a CHUNK_DATA message
func DeserializeChunkData(data []byte) (*ChunkData, error) {
	var chunk ChunkData
	if err := json.Unmarshal(data, &chunk); err != nil {
		return nil, fmt.Errorf("failed to deserialize chunk data: %w", err)
	}

	if chunk.Type != MessageChunkData {
		return nil, fmt.Errorf("invalid message type: expected %s, got %s", MessageChunkData, chunk.Type)
	}

	return &chunk, nil
}

// DeserializeChunkAck deserializes a CHUNK_ACK message
func DeserializeChunkAck(data []byte) (*ChunkAck, error) {
	var ack ChunkAck
	if err := json.Unmarshal(data, &ack); err != nil {
		return nil, fmt.Errorf("failed to deserialize chunk ack: %w", err)
	}

	if ack.Type != MessageChunkAck {
		return nil, fmt.Errorf("invalid message type: expected %s, got %s", MessageChunkAck, ack.Type)
	}

	return &ack, nil
}

// VerifyChecksum verifies the chunk data checksum
func (c *ChunkData) VerifyChecksum() bool {
	hash := sha256.Sum256(c.Data)
	return hex.EncodeToString(hash[:]) == c.Checksum
}
