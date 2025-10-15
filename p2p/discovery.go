package p2p

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"
)

// Peer represents a discovered peer on the network.
type Peer struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

// DiscoverPeers broadcasts a discovery message and collects responses.
func DiscoverPeers() map[string]Peer {
	fmt.Println("Discovering peers on the network...")

	// Listen for replies on a random UDP port
	localAddr, err := net.ResolveUDPAddr("udp", ":0")
	if err != nil {
		fmt.Printf("Error resolving local UDP address: %s\n", err)
		return nil
	}
	conn, err := net.ListenUDP("udp", localAddr)
	if err != nil {
		fmt.Printf("Error listening for UDP replies: %s\n", err)
		return nil
	}
	defer conn.Close()

	// The broadcast address for the local network
	broadcastAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("255.255.255.255:%d", DiscoveryPort))
	if err != nil {
		fmt.Printf("Error resolving broadcast address: %s\n", err)
		return nil
	}

	// Send the broadcast message
	_, err = conn.WriteToUDP([]byte(DiscoveryMsg), broadcastAddr)
	if err != nil {
		fmt.Printf("Error sending discovery broadcast: %s\n", err)
		return nil
	}

	peers := make(map[string]Peer)
	buffer := DiscoveryBufferPool.Get()
	defer DiscoveryBufferPool.Put(buffer)

	// Set a deadline to stop listening for replies
	conn.SetReadDeadline(time.Now().Add(ReplyTimeout))

	for {
		n, _, err := conn.ReadFromUDP(buffer)
		if err != nil {
			// If it's a timeout error, that's expected. We're done listening.
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			fmt.Printf("Error reading UDP reply: %s\n", err)
			break
		}

		var peer Peer
		if err := json.Unmarshal(buffer[:n], &peer); err == nil {
			// Use hostname as the key to avoid duplicates
			peers[peer.Hostname] = peer
		}
	}

	return peers
}

// ListenForDiscovery runs in the background to reply to discovery broadcasts.
func ListenForDiscovery(tcpPort string) {
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", DiscoveryPort))
	if err != nil {
		fmt.Printf("Error resolving discovery UDP address: %s\n", err)
		return
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		// Silently ignore port conflicts - discovery is optional
		return
	}
	defer conn.Close()

	hostname, _ := os.Hostname()
	buffer := DiscoveryBufferPool.Get()
	defer DiscoveryBufferPool.Put(buffer)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		if string(buffer[:n]) == DiscoveryMsg {
			// Got a discovery message, prepare and send a reply
			reply := Peer{
				Hostname: hostname,
				IP:       getLocalIP() + ":" + tcpPort,
			}
			replyBytes, _ := json.Marshal(reply)
			conn.WriteToUDP(replyBytes, remoteAddr)
		}
	}
}

// getLocalIP finds the preferred outbound IP address of this machine.
func getLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "127.0.0.1" // Fallback to localhost
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String()
}
