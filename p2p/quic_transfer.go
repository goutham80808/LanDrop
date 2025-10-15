package p2p

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)



// SendQUICMessage sends a simple message over QUIC for testing the protocol foundation
func SendQUICMessage(peerAddr, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get client TLS config
	tlsConfig := GetClientTLSConfig()

	// Dial QUIC connection
	conn, err := quic.DialAddr(ctx, peerAddr, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to dial QUIC: %w", err)
	}
	defer conn.CloseWithError(0, "")

	// Open a stream
	stream, err := conn.OpenStreamSync(ctx)
	if err != nil {
		return fmt.Errorf("failed to open stream: %w", err)
	}

	// Send message
	_, err = stream.Write([]byte(message))
	if err != nil {
		stream.CancelRead(0)
		return fmt.Errorf("failed to write message: %w", err)
	}

	fmt.Printf("Sent QUIC message: %s\n", message)
	return nil
}

// ReceiveQUICMessage listens for a QUIC connection and receives a message
func ReceiveQUICMessage(port string) error {
	// Get server TLS config
	tlsConfig := GetServerTLSConfig()
	if tlsConfig == nil {
		return fmt.Errorf("failed to get server TLS config")
	}

	// Create UDP listener
	addr, err := net.ResolveUDPAddr("udp", ":"+port)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP address: %w", err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return fmt.Errorf("failed to listen on UDP: %w", err)
	}
	defer conn.Close()

	fmt.Printf("Listening for QUIC connections on port %s...\n", port)

	// Create QUIC listener
	listener, err := quic.Listen(conn, tlsConfig, nil)
	if err != nil {
		return fmt.Errorf("failed to create QUIC listener: %w", err)
	}
	defer listener.Close()

	// Accept connection
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	quicConn, err := listener.Accept(ctx)
	if err != nil {
		return fmt.Errorf("failed to accept QUIC connection: %w", err)
	}
	defer quicConn.CloseWithError(0, "")

	fmt.Println("Accepted QUIC connection")

	// Accept stream
	stream, err := quicConn.AcceptStream(ctx)
	if err != nil {
		return fmt.Errorf("failed to accept stream: %w", err)
	}
	defer stream.Close()

	// Read message
	buffer := make([]byte, 1024)
	n, err := stream.Read(buffer)
	if err != nil && err != io.EOF {
		return fmt.Errorf("failed to read message: %w", err)
	}

	if n > 0 {
		receivedMessage := string(buffer[:n])
		fmt.Printf("Received QUIC message: %s\n", receivedMessage)
	} else {
		fmt.Printf("Received empty QUIC message\n")
	}

	return nil
}
