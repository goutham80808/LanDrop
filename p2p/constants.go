package p2p

import "time"

// Network constants
const (
	// DefaultPort is the default TCP port for file transfers
	DefaultPort = "8080"
	// DiscoveryPort is the UDP port for peer discovery
	DiscoveryPort = 8888
	// DiscoveryMsg is the broadcast message for peer discovery
	DiscoveryMsg = "LANDROP_DISCOVERY"
	// ReplyTimeout is the timeout for discovery responses
	ReplyTimeout = 2 * time.Second
)

// Chunked transfer constants
const (
	// DefaultChunkSize is the default size for file chunks (32MB)
	DefaultChunkSize = int64(32 * 1024 * 1024)
	// MaxRetries is the maximum number of retry attempts for failed chunks
	MaxRetries = 3
	// MaxConcurrentChunks is the maximum number of concurrent chunk transfers
	MaxConcurrentChunks = 3
	// StreamTimeout is the timeout for individual stream operations
	StreamTimeout = 30 * time.Second
	// ConnectionKeepalive is the keepalive interval for QUIC connections
	ConnectionKeepalive = 15 * time.Second
	// ChunkBufferSize is the size of the buffer for chunk transfers
	ChunkBufferSize = 32 * 1024 // 32KB
)

// Protocol constants
const (
	// ProtocolVersion is the current version of the LanDrop protocol
	ProtocolVersion = "1.0"
	// TLSServerName is the server name used for TLS connections
	TLSServerName = "landrop"
)

// TLS certificate constants
const (
	// CertificateValidityDays is the validity period for self-signed certificates
	CertificateValidityDays = 365
	// CertificateOrganization is the organization name for certificates
	CertificateOrganization = "LanDrop"
)
