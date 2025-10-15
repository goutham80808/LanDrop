package p2p

import "fmt"

// Error types for better error handling and debugging
var (
	// Network and connection errors
	ErrConnectionFailed    = fmt.Errorf("connection failed")
	ErrConnectionTimeout   = fmt.Errorf("connection timeout")
	ErrConnectionClosed    = fmt.Errorf("connection closed")
	ErrNetworkUnreachable  = fmt.Errorf("network unreachable")
	ErrAddressResolution   = fmt.Errorf("address resolution failed")
	
	// File operation errors
	ErrFileNotFound        = fmt.Errorf("file not found")
	ErrFileAccessDenied    = fmt.Errorf("file access denied")
	ErrFileCorrupted       = fmt.Errorf("file corrupted")
	ErrInsufficientSpace   = fmt.Errorf("insufficient disk space")
	ErrFileTooLarge        = fmt.Errorf("file too large")
	
	// Transfer errors
	ErrTransferInterrupted = fmt.Errorf("transfer interrupted")
	ErrTransferTimeout     = fmt.Errorf("transfer timeout")
	ErrChecksumMismatch    = fmt.Errorf("checksum mismatch")
	ErrChunkMissing        = fmt.Errorf("chunk missing")
	ErrChunkCorrupted      = fmt.Errorf("chunk corrupted")
	ErrTransferRejected    = fmt.Errorf("transfer rejected")
	
	// Protocol errors
	ErrInvalidMessage      = fmt.Errorf("invalid message")
	ErrProtocolMismatch    = fmt.Errorf("protocol mismatch")
	ErrUnsupportedVersion  = fmt.Errorf("unsupported protocol version")
	ErrHandshakeFailed     = fmt.Errorf("handshake failed")
	
	// Discovery errors
	ErrDiscoveryFailed     = fmt.Errorf("peer discovery failed")
	ErrNoPeersFound        = fmt.Errorf("no peers found")
	ErrPeerUnavailable     = fmt.Errorf("peer unavailable")
	
	// TLS/Security errors
	ErrTLSConfiguration    = fmt.Errorf("TLS configuration error")
	ErrCertificateInvalid  = fmt.Errorf("certificate invalid")
	ErrEncryptionFailed    = fmt.Errorf("encryption failed")
)

// TransferError represents a transfer-specific error with context
type TransferError struct {
	Type        error
	Filename    string
	PeerAddress string
	ChunkIndex  int
	Reason      string
}

// Error implements the error interface
func (te *TransferError) Error() string {
	if te.ChunkIndex >= 0 {
		return fmt.Sprintf("transfer error for file '%s' to %s (chunk %d): %v: %s",
			te.Filename, te.PeerAddress, te.ChunkIndex, te.Type, te.Reason)
	}
	return fmt.Sprintf("transfer error for file '%s' to %s: %v: %s",
		te.Filename, te.PeerAddress, te.Type, te.Reason)
}

// Unwrap returns the underlying error type
func (te *TransferError) Unwrap() error {
	return te.Type
}

// NewTransferError creates a new transfer error with context
func NewTransferError(errType error, filename, peerAddress string, chunkIndex int, reason string) *TransferError {
	return &TransferError{
		Type:        errType,
		Filename:    filename,
		PeerAddress: peerAddress,
		ChunkIndex:  chunkIndex,
		Reason:      reason,
	}
}

// IsTransferError checks if an error is a TransferError
func IsTransferError(err error) bool {
	_, ok := err.(*TransferError)
	return ok
}
