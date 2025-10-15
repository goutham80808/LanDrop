package p2p

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"time"

	"github.com/quic-go/quic-go"
)

// generateTLSConfig creates a self-signed certificate for QUIC connections
func generateTLSConfig() (*tls.Config, error) {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"LanDrop"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}

	// Generate certificate
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, err
	}

	// Create PEM blocks
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
	privBytes, err := x509.MarshalPKCS8PrivateKey(priv)
	if err != nil {
		return nil, err
	}
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})

	// Load certificate and key
	cert, err := tls.X509KeyPair(certPEM, privPEM)
	if err != nil {
		return nil, err
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{"landrop"},
	}, nil
}

// createInsecureTLSConfig creates a TLS config that trusts self-signed certificates for testing
func createInsecureTLSConfig() (*tls.Config, error) {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{"landrop"},
	}, nil
}

// SendQUICMessage sends a simple message over QUIC for testing the protocol foundation
func SendQUICMessage(peerAddr, message string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Create insecure TLS config for client (trusts self-signed certs)
	tlsConfig, err := createInsecureTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to create TLS config: %w", err)
	}

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
	// Generate TLS config for server
	tlsConfig, err := generateTLSConfig()
	if err != nil {
		return fmt.Errorf("failed to generate TLS config: %w", err)
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
