// main.go
package main

import (
	"fmt"
	"landrop/p2p"
	"os"
	"sync"
)

const DefaultPort = "8080"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	// Every peer should be discoverable, so we start the listener
	// for all commands except 'discover' and test commands.
	// We run it in the background so it doesn't block other commands.
	// Skip discovery for chunked commands to avoid port conflicts
	shouldSkipDiscovery := len(os.Args) > 1 && (
		os.Args[1] == "discover" || 
		os.Args[1] == "send-chunked" || 
		os.Args[1] == "recv-chunked" ||
		os.Args[1] == "test-quic-send" ||
		os.Args[1] == "test-quic-recv")
	
	if !shouldSkipDiscovery {
		go p2p.ListenForDiscovery(DefaultPort)
	}

	command := os.Args[1]

	switch command {
	case "discover":
		peers := p2p.DiscoverPeers()
		if len(peers) == 0 {
			fmt.Println("No other peers found on the network.")
			return
		}
		fmt.Println("Available peers:")
		for _, peer := range peers {
			fmt.Printf("  - %s (%s)\n", peer.Hostname, peer.IP)
		}

	case "send":
		if len(os.Args) != 4 {
			fmt.Println("Usage: landrop send <filename> <peer-hostname|all>")
			return
		}
		filename := os.Args[2]
		target := os.Args[3]

		fmt.Println("Finding peers...")
		peers := p2p.DiscoverPeers()
		if len(peers) == 0 {
			fmt.Println("No peers found to send to.")
			return
		}

		if target == "all" {
			fmt.Printf("Preparing to broadcast '%s' to %d peers.\n", filename, len(peers))
			// Use a WaitGroup to wait for all transfers to complete.
			var wg sync.WaitGroup
			for _, peer := range peers {
				wg.Add(1) // Increment the WaitGroup counter.
				// Launch a new goroutine for each transfer.
				go func(p p2p.Peer) {
					defer wg.Done() // Decrement the counter when the goroutine completes.
					fmt.Printf("\n--- Starting transfer to %s ---\n", p.Hostname)
					p2p.SendFile(filename, p.IP)
				}(peer)
			}
			wg.Wait() // Block until all goroutines have called Done().
			fmt.Println("\n--- All broadcast transfers complete. ---")
		} else {
			// Sending to a single, named peer.
			peer, exists := peers[target]
			if !exists {
				fmt.Printf("Error: Peer '%s' not found. Run 'landrop discover' to see available peers.\n", target)
				return
			}
			p2p.SendFile(filename, peer.IP)
		}

	case "recv":
		var port string
		if len(os.Args) > 2 {
			port = os.Args[2]
		} else {
			port = DefaultPort
		}
		// The discovery listener is already running from the top of main().
		fmt.Printf("Starting receiver on TCP port %s\n", port)
		fmt.Println("This machine is now discoverable by other peers.")
		p2p.ReceiveFile(port)

	case "test-quic-send":
		if len(os.Args) != 3 {
			fmt.Println("Usage: landrop test-quic-send <peer-address>")
			return
		}
		peerAddr := os.Args[2]
		err := p2p.SendQUICMessage(peerAddr, "Hello, QUIC!")
		if err != nil {
			fmt.Printf("QUIC send failed: %s\n", err)
		}

	case "test-quic-recv":
		var port string
		if len(os.Args) > 2 {
			port = os.Args[2]
		} else {
			port = DefaultPort
		}
		err := p2p.ReceiveQUICMessage(port)
		if err != nil {
			fmt.Printf("QUIC receive failed: %s\n", err)
		}

	case "send-chunked":
		if len(os.Args) != 4 {
			fmt.Println("Usage: landrop send-chunked <filename> <peer-address>")
			return
		}
		filename := os.Args[2]
		peerAddr := os.Args[3]
		err := p2p.SendFileChunked(filename, peerAddr)
		if err != nil {
			fmt.Printf("Chunked send failed: %s\n", err)
		}

	case "recv-chunked":
		var port string
		if len(os.Args) > 2 {
			port = os.Args[2]
		} else {
			port = DefaultPort
		}
		err := p2p.ReceiveFileChunked(port)
		if err != nil {
			fmt.Printf("Chunked receive failed: %s\n", err)
		}

	default:
		fmt.Printf("Unknown command: %s\n", command)
		printUsage()
	}
}

func printUsage() {
	fmt.Println("Usage: landrop <command> [options]")
	fmt.Println("\nCommands:")
	fmt.Println("  discover                  - Find other peers on the LAN")
	fmt.Println("  send <file> <hostname|all> - Send a file to a specific peer or to all peers")
	fmt.Println("  recv [port]               - Listen for incoming files (default port: 8080)")
	fmt.Println("  test-quic-recv [port]     - Test QUIC receiver (default port: 8080)")
	fmt.Println("  test-quic-send <address>  - Test QUIC sender to <address>")
	fmt.Println("  send-chunked <file> <addr> - Send file using new chunked protocol")
	fmt.Println("  recv-chunked [port]       - Receive file using new chunked protocol")
}
