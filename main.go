// LanDrop - Peer-to-peer file transfer over LAN
package main

import (
	"fmt"
	"landrop/p2p"
	"os"
	"sync"
	"time"
)

// DefaultPort is kept for backward compatibility
const (
	DefaultPort = "8080"
)

var (
	// Commands that should skip peer discovery
	skipDiscoveryCommands = map[string]bool{
		"discover":       true,
		"recv-chunked":   true,  // Skip global discovery - we start it manually in the function
		"test-quic-send": true,
		"test-quic-recv": true,
	}
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	command := os.Args[1]

	// Initialize TLS configuration
	if err := p2p.InitializeTLS(); err != nil {
		fmt.Printf("Warning: Failed to initialize TLS configuration: %v\n", err)
	}

	// Start peer discovery listener for applicable commands
	if !shouldSkipDiscovery(command) {
		go p2p.ListenForDiscovery(p2p.DefaultPort)
	}

	// Route command to appropriate handler
	if err := handleCommand(command); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}

// shouldSkipDiscovery determines if peer discovery should be skipped for a command
func shouldSkipDiscovery(command string) bool {
	return skipDiscoveryCommands[command]
}

// handleCommand routes the command to the appropriate handler
func handleCommand(command string) error {
	switch command {
	case "discover":
		return handleDiscover()
	case "send":
		return handleSend()
	case "recv":
		return handleRecv()
	case "test-quic-send":
		return handleQUICSend()
	case "test-quic-recv":
		return handleQUICRecv()
	case "send-chunked":
		return handleChunkedSend()
	case "recv-chunked":
		return handleChunkedRecv()
	case "device-info":
		return handleDeviceInfo()
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleDiscover discovers and displays available peers on the network
func handleDiscover() error {
	peers := p2p.DiscoverPeers()
	if len(peers) == 0 {
		fmt.Println("No other peers found on the network.")
		return nil
	}

	fmt.Println("Available peers:")
	for _, peer := range peers {
		fmt.Printf("  - %s (%s)\n", peer.Hostname, peer.IP)
	}
	return nil
}

// handleSend handles file sending to peers
func handleSend() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: landrop send <filename> <peer-hostname|all>")
	}

	filename := os.Args[2]
	target := os.Args[3]

	fmt.Println("Finding peers...")
	peers := p2p.DiscoverPeers()
	if len(peers) == 0 {
		return fmt.Errorf("no peers found to send to")
	}

	if target == "all" {
		return sendToAllPeers(filename, peers)
	}

	return sendToSinglePeer(filename, target, peers)
}

// sendToAllPeers broadcasts a file to all discovered peers
func sendToAllPeers(filename string, peers map[string]p2p.Peer) error {
	fmt.Printf("Preparing to broadcast '%s' to %d peers.\n", filename, len(peers))

	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(peer p2p.Peer) {
			defer wg.Done()
			fmt.Printf("\n--- Starting transfer to %s ---\n", peer.Hostname)
			p2p.SendFile(filename, peer.IP)
		}(peer)
	}

	wg.Wait()
	fmt.Println("\n--- All broadcast transfers complete. ---")
	return nil
}

// sendToSinglePeer sends a file to a specific peer
func sendToSinglePeer(filename, target string, peers map[string]p2p.Peer) error {
	peer, exists := peers[target]
	if !exists {
		return fmt.Errorf("peer '%s' not found. Run 'landrop discover' to see available peers", target)
	}

	p2p.SendFile(filename, peer.IP)
	return nil
}

// handleRecv handles file receiving
func handleRecv() error {
	port := getPortFromArgs(2)
	fmt.Printf("Starting receiver on TCP port %s\n", port)
	fmt.Println("This machine is now discoverable by other peers.")
	p2p.ReceiveFile(port)
	return nil
}

// handleQUICSend handles QUIC message sending for testing
func handleQUICSend() error {
	if len(os.Args) != 3 {
		return fmt.Errorf("usage: landrop test-quic-send <peer-address>")
	}

	peerAddr := os.Args[2]
	if err := p2p.SendQUICMessage(peerAddr, "Hello, QUIC!"); err != nil {
		return fmt.Errorf("QUIC send failed: %w", err)
	}

	return nil
}

// handleQUICRecv handles QUIC message receiving for testing
func handleQUICRecv() error {
	port := getPortFromArgs(2)
	if err := p2p.ReceiveQUICMessage(port); err != nil {
		return fmt.Errorf("QUIC receive failed: %w", err)
	}
	return nil
}

// handleChunkedSend handles chunked file sending
func handleChunkedSend() error {
	if len(os.Args) != 4 {
		return fmt.Errorf("usage: landrop send-chunked <filename> <peer-hostname|peer-address|all>")
	}

	filename := os.Args[2]
	target := os.Args[3]

	fmt.Println("Finding peers...")
	peers := p2p.DiscoverPeers()
	if len(peers) == 0 {
		return fmt.Errorf("no peers found to send to")
	}

	if target == "all" {
		return sendToAllPeersChunked(filename, peers)
	}

	return sendToSinglePeerChunked(filename, target, peers)
}

// handleChunkedRecv handles chunked file receiving
func handleChunkedRecv() error {
	port := getPortFromArgs(2)
	if err := p2p.ReceiveFileChunked(port); err != nil {
		return fmt.Errorf("chunked receive failed: %w", err)
	}
	return nil
}

// getPortFromArgs extracts port from command line arguments, returns default if not provided
func getPortFromArgs(argIndex int) string {
	if len(os.Args) > argIndex {
		return os.Args[argIndex]
	}
	return p2p.DefaultPort
}

// handleDeviceInfo displays device information and security details
func handleDeviceInfo() error {
	deviceInfo := p2p.GetDeviceInfo()
	if deviceInfo == nil {
		return fmt.Errorf("TLS manager not initialized - run with a command that initializes TLS first")
	}

	fmt.Println("=== LanDrop Device Information ===")
	fmt.Printf("Device ID:     %s\n", deviceInfo.DeviceID)
	fmt.Printf("Hostname:      %s\n", deviceInfo.Hostname)
	fmt.Printf("Fingerprint:   %s\n", deviceInfo.Fingerprint)
	fmt.Printf("Created:       %s\n", time.Unix(deviceInfo.CreatedAt, 0).Format("2006-01-02 15:04:05"))
	fmt.Println("\n=== Security Status ===")
	fmt.Println("✅ Embedded CA certificate: Active")
	fmt.Println("✅ Device certificate: Active") 
	fmt.Println("✅ Certificate pinning: Enabled")
	fmt.Println("✅ Peer authentication: Required")
	fmt.Println("\n=== Cross-Device Communication ===")
	fmt.Println("This device automatically trusts other LanDrop devices")
	fmt.Println("after user approval on first connection.")
	fmt.Println("No manual certificate sharing required!")
	fmt.Println("\nFirst-time connection will show:")
	fmt.Println("  - Device name and fingerprint")
	fmt.Println("  - User approval prompt (y/n)")
	fmt.Println("  - Automatic trust after approval")
	return nil
}

// sendToAllPeersChunked broadcasts a file to all discovered peers using chunked protocol
func sendToAllPeersChunked(filename string, peers map[string]p2p.Peer) error {
	fmt.Printf("Preparing to broadcast '%s' to %d peers using chunked protocol.\n", filename, len(peers))

	var wg sync.WaitGroup
	for _, peer := range peers {
		wg.Add(1)
		go func(peer p2p.Peer) {
			defer wg.Done()
			fmt.Printf("\n--- Starting chunked transfer to %s ---\n", peer.Hostname)
			if err := p2p.SendFileChunked(filename, peer.IP); err != nil {
				fmt.Printf("Error sending to %s: %v\n", peer.Hostname, err)
			}
		}(peer)
	}

	wg.Wait()
	fmt.Println("\n--- All chunked broadcast transfers complete. ---")
	return nil
}

// sendToSinglePeerChunked sends a file to a specific peer using chunked protocol
func sendToSinglePeerChunked(filename, target string, peers map[string]p2p.Peer) error {
	peer, exists := peers[target]
	if !exists {
		return fmt.Errorf("peer '%s' not found. Run 'landrop discover' to see available peers", target)
	}

	if err := p2p.SendFileChunked(filename, peer.IP); err != nil {
		return fmt.Errorf("chunked send failed: %w", err)
	}

	return nil
}

// printUsage displays the application usage information
func printUsage() {
	fmt.Println("LanDrop - Peer-to-peer file transfer over LAN")
	fmt.Println("\nUsage: landrop <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  discover                  Find other peers on the LAN")
	fmt.Println("  send <file> <hostname|all> Send a file to a specific peer or to all peers")
	fmt.Println("  recv [port]               Listen for incoming files (default port: 8080)")
	fmt.Println("  test-quic-recv [port]     Test QUIC receiver (default port: 8080)")
	fmt.Println("  test-quic-send <address>  Test QUIC sender to <address>")
	fmt.Println("  send-chunked <file> <hostname|all> Send file using new chunked protocol")
	fmt.Println("  recv-chunked [port]       Receive file using new chunked protocol")
	fmt.Println("  device-info               Display device security information")
	fmt.Println("\n🔐 Security Features:")
	fmt.Println("  ✅ Automatic peer authentication")
	fmt.Println("  ✅ Trust-on-first-use (TOFU)")
	fmt.Println("  ✅ No manual certificate sharing required")
	fmt.Println("\nFirst connection between devices will show approval prompt.")
}
