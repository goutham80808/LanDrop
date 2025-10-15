package p2p

import "sync"

// BufferPool provides a pool of reusable buffers to reduce memory allocations
type BufferPool struct {
	pool sync.Pool
}

// NewBufferPool creates a new buffer pool with the specified buffer size
func NewBufferPool(bufferSize int) *BufferPool {
	return &BufferPool{
		pool: sync.Pool{
			New: func() interface{} {
				return make([]byte, bufferSize)
			},
		},
	}
}

// Get retrieves a buffer from the pool
func (bp *BufferPool) Get() []byte {
	return bp.pool.Get().([]byte)
}

// Put returns a buffer to the pool for reuse
func (bp *BufferPool) Put(buffer []byte) {
	// Reset buffer length but keep capacity
	if cap(buffer) == len(buffer) {
		bp.pool.Put(buffer[:0])
	}
}

// Global buffer pools for different use cases
var (
	// ChunkBufferPool is used for chunk data transfers
	ChunkBufferPool = NewBufferPool(ChunkBufferSize)
	// DiscoveryBufferPool is used for peer discovery messages
	DiscoveryBufferPool = NewBufferPool(1024)
	// MessageBufferPool is used for general protocol messages
	MessageBufferPool = NewBufferPool(4096)
)
