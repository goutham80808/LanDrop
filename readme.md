# LanDrop

![Go Version](https://img.shields.io/badge/go-1.18%2B-blue)
![License](https://img.shields.io/badge/license-MIT-green)
![Platform](https://img.shields.io/badge/platform-Windows%20|%20Linux%20|%20macOS-lightgrey)

**LanDrop** is a command-line peer-to-peer (P2P) file sharing tool designed to work over a local network (LAN/Wi-Fi) without needing an internet connection or a central server. It's built entirely in Golang, resulting in a single, lightweight, and cross-platform executable.

---

## Key Features

- **Serverless P2P Architecture:** Every device is both a client and a server.
- **Automatic Peer Discovery:** No need to manually find and type IP addresses. Peers are discovered automatically on the local network.
- **Cross-Platform:** A single Go codebase compiles to native executables for Windows, macOS, and Linux.
- **Broadcast Transfers:** Send a file to all available peers on the network with a single command (`send <file> all`).
- **Resumable Transfers:** Interrupted transfers can be resumed from the exact point of failure, saving time and bandwidth.
- **Integrity Verification:** Uses SHA-256 checksums to ensure that received files are not corrupted.
- **Efficient & Lightweight:** Streams files directly without loading them fully into memory, making it efficient for very large files.

---

## Architecture and System Design

LanDrop operates on a decentralized, serverless model. The system is composed of two primary protocols working in tandem: a UDP-based protocol for discovery and a TCP-based protocol for reliable file transfer.

### Core Components

1. **CLI (Command-Line Interface):** The entry point for the user, responsible for parsing commands (`discover`, `send`, `recv`) and orchestrating the underlying modules.
2. **Peer Discovery Module (UDP):** A non-blocking module that constantly listens for discovery broadcasts and responds with its own information. It can also send broadcasts to find other peers.
3. **File Transfer Module (TCP):** A reliable, point-to-point module for handling the file transfer protocol, including metadata exchange, resumability, and integrity verification.

### Protocol Design

#### 1. Discovery Protocol (UDP Broadcast on Port 8888)

The discovery process is designed to be lightweight and connectionless.

- **Broadcast:** A peer wanting to discover others sends a UDP broadcast packet containing the message `"LANDROP_DISCOVERY"` to the network's broadcast address (`255.255.255.255:8888`).
- **Listen & Reply:** All `landrop` instances listen on UDP port 8888. Upon receiving the `"LANDROP_DISCOVERY"` message, they do not broadcast back. Instead, they send a direct UDP response to the original sender's address. This response is a JSON object containing their `hostname` and `IP:port` for TCP connections.
- **Collection:** The discovering peer collects all direct responses for a short period (2 seconds) and then displays the unique list of peers.

#### 2. File Transfer Protocol (TCP)

The file transfer protocol is a stateful handshake designed for reliability and resumability.

- **Handshake 1 (Metadata):**
    - The **Sender** connects to the Receiver's TCP socket.
    - It immediately sends a JSON object containing the file's `FileMetadata` (Filename, Total File Size, SHA-256 Hash of the *entire* original file), followed by a newline `\n` delimiter.
- **Handshake 2 (Resume):**
    - The **Receiver** parses the metadata. It checks if a file with the same name already exists locally.
    - If so, it determines the size of the partial file.
    - It sends a `ResumeResponse` JSON back to the sender, specifying the byte `offset` it has already received (e.g., `{"offset": 5000000}`). If the file is new, the offset is 0.
- **Data Stream:**
    - The **Sender** receives the offset, seeks to that byte position in the source file, and begins streaming only the remaining file data.
    - The **Receiver** appends the incoming data stream to its local file.
- **Handshake 3 (Verification & ACK):**
    - After the stream is complete, the **Receiver** closes the file, calculates the SHA-256 hash of its now-complete local file, and compares it to the hash from the initial metadata.
    - If the hashes match, it sends a final `"ACK\n"` (Acknowledge) message back to the sender.
    - If they mismatch, it sends `"ERR_CHECKSUM\n"`.
- **Connection Close:** The Sender waits for the final ACK/ERR before closing the connection and reporting the final status to the user.

---

## 📥 Installation & Usage

### Prerequisites
- **No installation required** if you are using a pre-compiled executable from the **Releases** section.
- To build from source, you need the **Go compiler (v1.18 or higher)**.

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
├── main.go               # CLI handler and main entry point
└── p2p/
    ├── discovery.go      # UDP broadcast and discovery logic
    └── tcp_transfer.go   # TCP file transfer protocol logic
```

---

## 🚀 Future Improvements

- **PWA Frontend:** Embed a web interface (HTML/CSS/JS) into the binary and serve it on a local port. This would allow for a drag-and-drop user experience from a web browser.
- **Chunk-Based Parallelism:** For very high-speed networks, split the file into chunks and send multiple chunks to the receiver in parallel over separate TCP connections to maximize throughput.
- **Encryption:** Implement end-to-end encryption for the TCP data stream using Go's `crypto/tls` library to secure transfers on untrusted networks.
