package p2p

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"time"
)

// TLSManager handles TLS configuration for QUIC connections
type TLSManager struct {
	serverConfig *tls.Config
	clientConfig *tls.Config
}

// NewTLSManager creates a new TLS manager with self-signed certificates
func NewTLSManager() (*TLSManager, error) {
	serverConfig, err := generateServerTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to generate server TLS config: %w", err)
	}

	clientConfig := createClientTLSConfig()

	return &TLSManager{
		serverConfig: serverConfig,
		clientConfig: clientConfig,
	}, nil
}

// GetServerConfig returns the TLS configuration for server connections
func (tm *TLSManager) GetServerConfig() *tls.Config {
	return tm.serverConfig
}

// GetClientConfig returns the TLS configuration for client connections
func (tm *TLSManager) GetClientConfig() *tls.Config {
	return tm.clientConfig
}

// generateServerTLSConfig creates a self-signed certificate for QUIC server connections
func generateServerTLSConfig() (*tls.Config, error) {
	// Generate private key
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Create certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(time.Duration(CertificateValidityDays) * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, err
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{CertificateOrganization},
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
		NextProtos:   []string{TLSServerName},
	}, nil
}

// createClientTLSConfig creates a TLS config that trusts self-signed certificates for testing
func createClientTLSConfig() *tls.Config {
	return &tls.Config{
		InsecureSkipVerify: true,
		NextProtos:         []string{TLSServerName},
	}
}

// Global TLS manager instance
var globalTLSManager *TLSManager

// InitializeTLS initializes the global TLS manager
func InitializeTLS() error {
	var err error
	globalTLSManager, err = NewTLSManager()
	return err
}

// GetServerTLSConfig returns the global server TLS configuration
func GetServerTLSConfig() *tls.Config {
	if globalTLSManager == nil {
		// Fallback to direct generation if not initialized
		config, err := generateServerTLSConfig()
		if err != nil {
			return nil
		}
		return config
	}
	return globalTLSManager.GetServerConfig()
}

// GetClientTLSConfig returns the global client TLS configuration
func GetClientTLSConfig() *tls.Config {
	if globalTLSManager == nil {
		// Fallback to direct generation if not initialized
		return createClientTLSConfig()
	}
	return globalTLSManager.GetClientConfig()
}
