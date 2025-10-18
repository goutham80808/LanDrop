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

	// Try multiple broadcast addresses for different network scenarios
	broadcastAddresses := []string{
		fmt.Sprintf("255.255.255.255:%d", DiscoveryPort), // Global broadcast
	}
	
	// Add network-specific broadcast addresses (IPv4 only)
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			
			for _, addr := range addrs {
				if ipNet, ok := addr.(*net.IPNet); ok && ipNet.IP.To4() != nil {
					// Calculate broadcast address for this IPv4 subnet
					broadcast := make(net.IP, len(ipNet.IP))
					copy(broadcast, ipNet.IP)
					
					for i := 0; i < len(ipNet.Mask); i++ {
						broadcast[i] |= ^ipNet.Mask[i]
					}
					
					// Only add if it's a valid IPv4 broadcast address
					if broadcast.To4() != nil {
						broadcastAddr := fmt.Sprintf("%s:%d", broadcast.To4().String(), DiscoveryPort)
						broadcastAddresses = append(broadcastAddresses, broadcastAddr)
					}
				}
			}
		}
	}
	
	fmt.Printf("Trying %d broadcast addresses for discovery...\n", len(broadcastAddresses))

	// Send broadcast messages to all addresses
	for i, broadcastAddrStr := range broadcastAddresses {
		broadcastAddr, err := net.ResolveUDPAddr("udp", broadcastAddrStr)
		if err != nil {
			fmt.Printf("Error resolving broadcast address %s: %s\n", broadcastAddrStr, err)
			continue
		}
		
		_, err = conn.WriteToUDP([]byte(DiscoveryMsg), broadcastAddr)
		if err != nil {
			fmt.Printf("Error sending discovery broadcast to %s: %s\n", broadcastAddrStr, err)
		} else {
			fmt.Printf("Sent discovery broadcast to %s\n", broadcastAddrStr)
		}
		
		// Small delay between broadcasts to avoid network congestion
		if i < len(broadcastAddresses)-1 {
			time.Sleep(10 * time.Millisecond)
		}
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
			fmt.Printf("Discovery: Found peer %s at %s\n", peer.Hostname, peer.IP)
			peers[peer.Hostname] = peer
		} else {
			fmt.Printf("Discovery: Failed to parse peer response: %v\n", err)
		}
	}

	return peers
}

// ListenForDiscovery runs in the background to reply to discovery broadcasts.
func ListenForDiscovery(tcpPort string) {
	fmt.Printf("Discovery: ListenForDiscovery called with port: '%s'\n", tcpPort)
	
	addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf(":%d", DiscoveryPort))
	if err != nil {
		fmt.Printf("Error resolving discovery UDP address: %s\n", err)
		return
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		// Silently ignore port conflicts - discovery is optional
		fmt.Printf("Discovery: UDP port %d already in use (another discovery listener may be running)\n", DiscoveryPort)
		return
	}
	defer conn.Close()

	hostname, _ := os.Hostname()
	buffer := DiscoveryBufferPool.Get()
	defer DiscoveryBufferPool.Put(buffer)

	fmt.Printf("Discovery: Started listener for TCP port %s\n", tcpPort)

	for {
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			continue
		}

		if string(buffer[:n]) == DiscoveryMsg {
			// Got a discovery message, prepare and send a reply
			localIP := getLocalIP()
			fmt.Printf("Discovery: Replying with IP %s:%s from interface\n", localIP, tcpPort)
			reply := Peer{
				Hostname: hostname,
				IP:       localIP + ":" + tcpPort,
			}
			replyBytes, _ := json.Marshal(reply)
			conn.WriteToUDP(replyBytes, remoteAddr)
		}
	}
}

// getLocalIP finds the preferred outbound IP address of this machine.
func getLocalIP() string {
	// Try multiple methods to get a suitable local IP
	
	// Method 1: Get all non-loopback interfaces and pick the first suitable one
	interfaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range interfaces {
			// Skip down interfaces and loopback
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				
				// Skip IPv6 and loopback addresses
				if ip == nil || ip.IsLoopback() || ip.To4() == nil {
					continue
				}
				
				fmt.Printf("Found local IP: %s (interface: %s)\n", ip.String(), iface.Name)
				return ip.String()
			}
		}
	}
	
	// Method 2: Fallback to original method with Google DNS
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		if !localAddr.IP.IsLoopback() && localAddr.IP.To4() != nil {
			fmt.Printf("Found local IP via Google DNS: %s\n", localAddr.IP.String())
			return localAddr.IP.String()
		}
	}
	
	// Method 3: Try connecting to a local router
	conn, err = net.Dial("udp", "192.168.1.1:80")
	if err == nil {
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		if !localAddr.IP.IsLoopback() && localAddr.IP.To4() != nil {
			fmt.Printf("Found local IP via router: %s\n", localAddr.IP.String())
			return localAddr.IP.String()
		}
	}
	
	// Last resort: use localhost but warn the user
	fmt.Println("Warning: Could not find a suitable non-loopback IP address.")
	fmt.Println("Falling back to localhost (127.0.0.1) - this will only work for same-device transfers.")
	fmt.Println("For cross-device transfers, please check:")
	fmt.Println("  - Network connection is active")
	fmt.Println("  - Firewall allows UDP port 8888 and TCP port 8080")
	fmt.Println("  - Devices are on the same network subnet")
	return "127.0.0.1"
}
