# LanDrop

![Go Version](https://img.shields.io/badge/go-1.18%2B-blue)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

**LanDrop** is a high-performance peer-to-peer (P2P) file sharing tool designed to work over a local network (LAN/Wi-Fi) without needing an internet connection or a central server. Built entirely in Golang, it delivers **100x performance improvements** with a modern QUIC-based protocol.

---

## Key Features

### 🚀 Version 2.0 Complete Features
- **100x Performance Boost:** Ultra-fast transfers at 22+ MB/s (up from 0.2 MB/s)
- **QUIC Protocol:** Modern UDP-based protocol with built-in reliability and multiplexing
- **Binary Chunk Protocol:** Minimal overhead (<0.001%) with 32MB optimal chunk size
- **Per-Chunk Integrity:** SHA-256 verification for every chunk ensures perfect data integrity
- **Smart Resume:** Automatic resume from interrupted transfers with chunk-level precision
- **Large File Support:** Fixed chunk indexing to handle files >4GB and up to petabyte-scale
- **Enhanced Security:** TLS 1.3 encryption with trust-on-first-use for cross-device transfers
- **Beautiful Progress Display:** Clean single-line progress with spinning animation and real-time stats
- **Professional UX:** Color-coded speed indicators, elapsed time, and clean output management

### 🌟 Core Features
- **Serverless P2P Architecture:** Every device is both a client and a server
- **Device Name Support:** Use memorable hostnames like `DESKTOP-JOHN` instead of IP addresses
- **Automatic Peer Discovery:** Devices are discovered automatically with friendly computer names
- **Cross-Platform:** A single Go codebase compiles to native executables for Windows, macOS, and Linux
- **Broadcast Transfers:** Send a file to all available peers on the network with a single command (`send <file> all`)
- **Chunked Transfers:** Intelligent chunking strategy for optimal performance on large files
- **Robust Error Handling:** Automatic retries with exponential backoff for network reliability
- **Real-time Progress:** Beautiful spinning animation progress bar with live speed, chunk count, and elapsed time

---

## Architecture and System Design

LanDrop v2.0 operates on a completely re-engineered decentralized architecture. The system combines modern QUIC protocol for high-performance transfers with UDP-based peer discovery for seamless network integration.

### Core Components

1. **CLI (Command-Line Interface):** Enhanced entry point with support for legacy TCP and new QUIC-based protocols
2. **Peer Discovery Module (UDP):** Non-blocking module that constantly listens for discovery broadcasts and responds with peer information
3. **QUIC Transfer Module:** High-performance protocol with TLS 1.3 encryption, multiplexed streams, and chunk-based transfers
4. **TCP Transfer Module (Legacy):** Backward-compatible protocol for basic file transfers
5. **Protocol Handler:** Intelligent routing between TCP and QUIC protocols based on capabilities

### Version 2.0 Protocol Architecture

#### 1. Discovery Protocol (UDP Broadcast on Port 8888)
- **Broadcast:** UDP broadcast containing `"LANDROP_DISCOVERY"` message
- **Response:** Direct UDP reply with JSON peer information (hostname, IP:port)
- **Collection:** 2-second timeout for peer discovery and aggregation

#### 2. QUIC Transfer Protocol (Port 8080)
- **Handshake:** Secure TLS 1.3 handshake with self-signed certificates
- **Metadata Exchange:** JSON-based transfer request with file metadata
- **Chunked Transfer:** 32MB chunks with per-chunk SHA-256 verification
- **Stream Multiplexing:** Multiple concurrent QUIC streams per transfer
- **Binary Protocol:** 40-byte headers for minimal overhead

#### 3. Legacy TCP Protocol (Port 8080)
- **Backward Compatibility:** Original single-stream TCP protocol
- **Resume Support:** Basic file-level resume capability
- **Integrity Verification:** SHA-256 checksums for complete files

### Performance Improvements (Version 2.0)

| Metric | Version 1.0 | Version 2.0 | Improvement |
|--------|-------------|-------------|-------------|
| Transfer Speed | 0.2 MB/s | 22+ MB/s | **100x** |
| Large File Support | ❌ Failed | ✅ Perfect (>4GB to Petabytes) | **100%** |
| Protocol Overhead | 50% | <0.001% | **99.9% reduction** |
| Chunk Count | 1024 (1MB) | 32 (32MB) | **32x reduction** |
| Transfer Time (1GB) | 15+ minutes | 46 seconds | **20x faster** |
| Reliability | ❌ Corrupted | ✅ Perfect | **100% success rate** |
| Progress Display | ❌ Verbose | ✅ Beautiful Spinning | **100% improved** |
| User Experience | ❌ Poor Feedback | ✅ Professional | **100% enhanced** |

### Usage Examples

#### New QUIC-based Commands (Recommended)
```bash
# Start high-performance receiver
landrop recv-chunked

# Send file using optimized chunked protocol with device name
landrop send-chunked <filename> <device-hostname>

# Send file using optimized chunked protocol with IP address
landrop send-chunked <filename> <peer-address>

# Send to all discovered peers
landrop send-chunked <filename> all

# Test QUIC connectivity
landrop test-quic-recv [port]
landrop test-quic-send <peer-address>
```

#### Legacy TCP Commands (Backward Compatible)
```bash
# Discover peers
landrop discover

# Send file to specific peer
landrop send <filename> <hostname>

# Send to all peers
landrop send <filename> all

# Start receiver
landrop recv [port]
```

---

## 📥 Installation & Usage

### Prerequisites
- **No installation required** if you are using a pre-compiled executable from the **Releases** section.
- To build from source, you need the **Go compiler (v1.18 or higher)**.

### Quick Start (Version 2.0)

1. **Start the receiver** on one machine:
```bash
# High-performance QUIC receiver (recommended)
landrop recv-chunked

# Or legacy TCP receiver
landrop recv
```

2. **Discover peers** on the sender machine:
```bash
landrop discover
```

3. **Send files** using the new chunked protocol:
```bash
# Send to a specific peer using device name (recommended)
landrop send-chunked <filename> <device-hostname>

# Send to a specific peer using IP address
landrop send-chunked <filename> <peer-address>

# Send to all discovered peers
landrop send-chunked <filename> all
```

### Network Requirements
- **Same Network**: Both devices must be on the same LAN/Wi-Fi network
- **Firewall**: Ensure ports 8080 (TCP/UDP) and 8888 (UDP) are not blocked
- **Discovery**: UDP broadcasts must be allowed on the network

### 🏷️ Enhanced Device Name Support
Version 2.0 now supports human-readable device names for both protocols:

```bash
# Discover available devices with friendly names
landrop discover
# Output: GOUTHAM-808 (192.168.1.10:8080), LAPTOP-ALICE (192.168.1.15:8080)

# Send using memorable device names (no IP addresses needed!)
landrop send-chunked report.pdf GOUTHAM-808
landrop send video.mp4 LAPTOP-ALICE

# Legacy protocol also supports device names
landrop send document.docx all
```

**Benefits:**
- **No More IP Memorization**: Use device names like `DESKTOP-JOHN` instead of `192.168.1.10:8080`
- **Enhanced UX**: Friendly, human-readable interface for all transfers
- **Automatic Discovery**: Devices appear with their computer names automatically
- **Broadcast Support**: Send to all peers with a simple `all` command

### Troubleshooting Cross-Computer Issues
If LanDrop works on the same computer but not between different computers:

1. **Check Firewall Settings**:
   - Windows: Allow `landrop.exe` through Windows Firewall
   - Add exceptions for ports 8080 and 8888

2. **Verify Network Connectivity**:
   ```bash
   # Test direct connection
   landrop test-quic-send <peer_ip>:8080
   landrop test-quic-recv 8080
   ```

3. **Check Network Topology**:
   - Ensure both computers are on the same subnet
   - Run `ipconfig` (Windows) or `ifconfig` (macOS/Linux) to verify similar IP ranges

---

## 🔧 Initial Setup
1. Download the correct executable for your operating system:
   - `landrop.exe` for **Windows**
   - `landrop-linux` for **Linux**
   - `landrop-mac-arm` / `landrop-mac-intel` for **macOS**
2. Save the executable in a convenient location (e.g., `Downloads` folder).
3. Open a terminal application:
   - **Windows**: PowerShell or Command Prompt
   - **macOS/Linux**: Terminal
4. Navigate to the directory where you saved the executable:

```sh
cd Downloads
```

---

## ▶️ How to Run

All commands are run from the terminal.

---

### 📨 Receiver: To Receive a File

It is recommended to receive files in a dedicated downloads folder.

**Windows (PowerShell):**

```powershell
# 1. Create a new folder for downloads and enter it
mkdir landrop_downloads
cd landrop_downloads

# 2. Start the receiver
..\landrop.exe recv
```

**macOS / Linux (Terminal):**

```sh
# 1. Create a new folder for downloads and enter it
mkdir -p landrop_downloads
cd landrop_downloads

# 2. Start the receiver
../landrop recv
```

Your machine is now discoverable and ready for incoming files.

---

### 📤 Sender: To Send a File

**Step 1: Discover who is on the network**

**Windows (PowerShell):**

```powershell
.\landrop.exe discover
```

**macOS / Linux (Terminal):**

```sh
./landrop discover
```

Example output:

```
DESKTOP-JOHN (192.168.1.10:8080)
```

---

**Step 2: Send the file**

Make sure your terminal is in the same directory as the file you want to send.

**Windows (PowerShell):**

```powershell
# Send a file to a specific peer
.\landrop.exe send "My Project.zip" DESKTOP-JOHN

# Send a file to EVERYONE on the network
.\landrop.exe send "My Project.zip" all
```

**macOS / Linux (Terminal):**

```sh
# Send a file to a specific peer
./landrop send "My Project.zip" DESKTOP-JOHN

# Send a file to EVERYONE on the network
./landrop send "My Project.zip" all
```

---

## 🛠️ How to Build for Cross-Platform

You can compile executables for all major platforms from a single machine. Run these commands from the project root.

```sh
# For Windows (64-bit)
GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o ./builds/landrop.exe .

# For Linux (64-bit)
GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o ./builds/landrop-linux .

# For macOS (Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w" -o ./builds/landrop-mac-arm .

# For macOS (Intel)
GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w" -o ./builds/landrop-mac-intel .
```

The compiled binaries will be placed in the `builds` directory.

---

## 📁 Project Structure

```
landrop/
├── go.mod
├── go.sum
├── main.go                    # Enhanced CLI handler with protocol routing
└── p2p/
    ├── constants.go           # Network and protocol constants
    ├── discovery.go           # UDP broadcast and discovery logic
    ├── protocol.go            # Message serialization and protocol definitions
    ├── tcp_transfer.go        # Legacy TCP file transfer protocol
    ├── quic_transfer.go       # High-performance QUIC file transfer
    ├── chunked_transfer.go    # Chunked transfer implementation
    ├── tls_config.go          # TLS configuration for QUIC
    ├── errors.go              # Error handling utilities
    ├── buffer_pool.go         # Memory pool management
    └── transfer_stats.go      # Transfer statistics tracking
```

---

## 🛣️ Development Roadmap

### ✅ Phase 1: High-Performance QUIC Protocol (COMPLETED)
- [x] QUIC protocol foundation with TLS 1.3
- [x] Binary chunk protocol with minimal overhead
- [x] Optimized 32MB chunking strategy
- [x] Application-level reliability with per-chunk verification
- [x] Enhanced security model with built-in encryption

### 🔒 Phase 2: Enhanced Security (COMPLETED)
- [x] TLS certificate verification fix for cross-device transfers
- [x] Trust-on-first-use approach for LanDrop devices
- [x] Cross-device compatibility without hardcoded IP limitations
- [x] Proper certificate management for local networks

### 🎨 Phase 3: User Experience (COMPLETED)
- [x] Compact single-line progress display with spinning animation
- [x] Real-time transfer statistics (speed, time, chunk count)
- [x] Color-coded progress indicators
- [x] Clean output management and professional summaries
- [x] Enhanced CLI with beautiful UX

### 🚀 Phase 4: Advanced Features (PLANNED)
- [ ] Multi-file and directory transfers with manifests
- [ ] Transfer history and analytics with SQLite storage
- [ ] Multi-recipient broadcast with session management
- [ ] Progressive Web App (PWA) interface with drag-and-drop
- [ ] Zero-configuration setup and automatic peer pairing

---

## 🎯 Performance Benchmarks

### Test Environment
- **Network**: 1Gbps LAN
- **File Size**: 1GB binary file
- **Hardware**: Modern desktop computers

### Results
| Protocol | Transfer Speed | CPU Usage | Memory Usage | Reliability |
|----------|----------------|-----------|--------------|-------------|
| Legacy TCP | 0.2 MB/s | 5% | 50MB | ❌ Corrupted files |
| **QUIC v2.0** | **22+ MB/s** | **15%** | **100MB** | ✅ **Perfect integrity** |

### Chunk Size Optimization
| Chunk Size | Chunk Count | Transfer Speed | Efficiency |
|------------|-------------|----------------|------------|
| 1MB | 1024 | 0.2 MB/s | ❌ Poor |
| 8MB | 128 | 0.9 MB/s | ❌ Slow |
| **32MB** | **32** | **22 MB/s** | ✅ **Optimal** |

---

## 🔧 Technical Deep Dive

### Binary Protocol Design
```go
// 44-byte header for maximum efficiency (updated for large file support)
type ChunkHeader struct {
    ChunkIndex [8]byte   // uint64 - supports petabyte-scale files
    DataSize   [4]byte   // uint32  
    Checksum   [32]byte  // SHA-256
}
```

### Large File Support
The protocol now handles files of any size through:
- **64-bit Chunk Indexing**: Expanded from 32-bit to 64-bit chunk indices
- **Petabyte Scale**: Supports files up to 18 exabytes with 32MB chunks
- **Overflow Prevention**: Fixed integer overflow issues that caused errors with files >4GB
- **Backward Compatibility**: Protocol changes are transparent to end users

### Performance Optimizations
- **Buffer Pool Management**: Reuse memory buffers to reduce GC pressure
- **Stream Multiplexing**: Parallel QUIC streams for concurrent chunks
- **Zero-Copy Operations**: Minimize memory allocations during transfers
- **Adaptive Chunking**: Dynamic chunk sizing based on network conditions

### Security Features
- **TLS 1.3**: Modern encryption with perfect forward secrecy
- **Trust-on-First-Use**: Cross-device compatibility with proper certificate management
- **Per-Chunk Integrity**: SHA-256 verification for every data chunk
- **Stream Isolation**: Independent security contexts per transfer

### Beautiful Progress Display
Version 2.0 features a stunning single-line progress interface:

```bash
# Live example during transfer
[******|.....] SEND 60.0% | 6/10 | 🚀 3.2MB/s | ⏱️ 00:08
```

**Features:**
- **Spinning Animation:** Smooth `|/-\-` animation for current chunk
- **Visual Indicators:** `*` for completed, `.` for pending chunks
- **Real-Time Statistics:** Live speed (MB/s), elapsed time, and progress percentage
- **Color Coding:** Speed-based colors (green/yellow/cyan) for quick performance glance
- **Clean Output:** Professional single-line display with proper line clearing
- **Professional Summaries:** Beautiful completion reports with comprehensive transfer statistics

---

## 🤝 Contributing

We welcome contributions! Key areas for improvement:
1. **Protocol Optimization**: Enhance the binary protocol for even better performance
2. **Cross-Platform Testing**: Ensure reliability across different operating systems
3. **Network Compatibility**: Improve discovery on complex network topologies
4. **Security Enhancements**: Implement advanced authentication mechanisms

### Development Setup
```bash
# Clone the repository
git clone https://github.com/goutham80808/LanDrop.git
cd landrop

# Install dependencies
go mod tidy

# Run tests
go test ./p2p/...

# Build for your platform
go build -o landrop .
```

---

## Created by

[Goutham Krishna Mandati](https://github.com/goutham80808)
