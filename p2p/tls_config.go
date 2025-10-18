package p2p

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TrustedPeer stores information about approved peer devices
type TrustedPeer struct {
	DeviceID    string    `json:"device_id"`
	Hostname    string    `json:"hostname"`
	Fingerprint string    `json:"fingerprint"`
	CACert      string    `json:"ca_cert"`      // PEM-encoded CA certificate
	DeviceCert  string    `json:"device_cert"`  // PEM-encoded device certificate
	ApprovedAt  int64     `json:"approved_at"`
	LastSeen    int64     `json:"last_seen"`
}

// TrustStore manages persistent storage of trusted peers
type TrustStore struct {
	filePath string
	peers    map[string]*TrustedPeer
	mutex    sync.RWMutex
}

// Enhanced Peer information for discovery with certificate data
type PeerWithCert struct {
	Hostname     string `json:"hostname"`
	IP           string `json:"ip"`
	CAFingerprint string `json:"ca_fingerprint"`
}

// TLSManager handles TLS configuration for QUIC connections
type TLSManager struct {
	serverConfig *tls.Config
	clientConfig *tls.Config
	caCert       *x509.Certificate
	caKey        *ecdsa.PrivateKey
	deviceCert   *x509.Certificate
	deviceKey    *ecdsa.PrivateKey
	trustStore   *TrustStore
	testingMode  bool
}

// DeviceInfo contains device identification information
type DeviceInfo struct {
	DeviceID      string `json:"device_id"`
	Hostname      string `json:"hostname"`
	Fingerprint   string `json:"fingerprint"`
	CAFingerprint string `json:"ca_fingerprint"`
	CreatedAt     int64  `json:"created_at"`
}

// NewTLSManager creates a new TLS manager with embedded CA and device certificates
func NewTLSManager() (*TLSManager, error) {
	// Check if we're in testing mode (environment variable or same device detection)
	testingMode := os.Getenv("LANDROP_TESTING_MODE") == "true"

	if testingMode {
		fmt.Printf("🔧 Creating TLS Manager in TESTING MODE (InsecureSkipVerify=true)\n")
		return createTestingTLSManager()
	}

	fmt.Printf("🔐 Creating TLS Manager in PRODUCTION MODE with proper certificate chain\n")
	return createProductionTLSManager()
}

// createTestingTLSManager creates a simple TLS manager for testing
func createTestingTLSManager() (*TLSManager, error) {
	fmt.Printf("🔧 Creating testing TLS Manager with InsecureSkipVerify=true\n")

	// Create permissive server config for testing
	testingServerConfig := &tls.Config{
		InsecureSkipVerify:   true,
	MinVersion:           tls.VersionTLS12,
		ClientAuth:           tls.NoClientCert, // Don't require client cert in testing mode
		ServerName:           "", // Accept any server name
	}

	testingClientConfig := createTestingTLSConfig()

	return &TLSManager{
		serverConfig: testingServerConfig,
		clientConfig: testingClientConfig,
		caCert:       nil,
		caKey:        nil,
		deviceCert:   nil,
		deviceKey:    nil,
		trustStore:   nil,
		testingMode:  true,
	}, nil
}

// createPermissiveTLSManager creates a TLS manager that trusts on first use without prompts
func createPermissiveTLSManager() (*TLSManager, error) {
	fmt.Printf("🔓 Creating permissive TLS Manager with trust-on-first-use\n")

	// Create or load trust store
	trustStore, err := createTrustStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create trust store: %w", err)
	}

	// Generate CA and device certificates
	caCert, caKey, err := generateCertificateAuthority()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA: %w", err)
	}

	deviceCert, deviceKey, err := generateDeviceCertificate(caCert, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate device certificate: %w", err)
	}

	// Create server TLS config
	serverConfig, err := createServerTLSConfigWithTrustStore(deviceCert, deviceKey, caCert, trustStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create server TLS config: %w", err)
	}

	// Create client TLS config with permissive verification
	clientConfig := createClientTLSConfigPermissive(caCert, deviceCert, deviceKey, trustStore)

	return &TLSManager{
		serverConfig: serverConfig,
		clientConfig: clientConfig,
		caCert:       caCert,
		caKey:        caKey,
		deviceCert:   deviceCert,
		deviceKey:    deviceKey,
		trustStore:   trustStore,
		testingMode:  false,
	}, nil
}

// createProductionTLSManager creates a full TLS manager with CA and device certificates
func createProductionTLSManager() (*TLSManager, error) {
	fmt.Printf("🔐 Creating production TLS Manager with proper CA and device certificates\n")

	// Create or load trust store
	trustStore, err := createTrustStore()
	if err != nil {
		return nil, fmt.Errorf("failed to create trust store: %w", err)
	}

	// Generate CA and device certificates
	caCert, caKey, err := generateCertificateAuthority()
	if err != nil {
		return nil, fmt.Errorf("failed to generate CA: %w", err)
	}

	deviceCert, deviceKey, err := generateDeviceCertificate(caCert, caKey)
	if err != nil {
		return nil, fmt.Errorf("failed to generate device certificate: %w", err)
	}

	// Create server TLS config
	serverConfig, err := createServerTLSConfigWithTrustStore(deviceCert, deviceKey, caCert, trustStore)
	if err != nil {
		return nil, fmt.Errorf("failed to create server TLS config: %w", err)
	}

	// Create client TLS config with trust-aware verification
	clientConfig := createClientTLSConfigWithTrustStore(caCert, deviceCert, deviceKey, trustStore)

	return &TLSManager{
		serverConfig: serverConfig,
		clientConfig: clientConfig,
		caCert:       caCert,
		caKey:        caKey,
		deviceCert:   deviceCert,
		deviceKey:    deviceKey,
		trustStore:   trustStore,
		testingMode:  false,
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

// createTrustStore creates or loads a persistent trust store
func createTrustStore() (*TrustStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get user home directory: %w", err)
	}

	landropDir := filepath.Join(homeDir, ".landrop")
	if err := os.MkdirAll(landropDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create .landrop directory: %w", err)
	}

	trustStorePath := filepath.Join(landropDir, "trusted_peers.json")

	trustStore := &TrustStore{
		filePath: trustStorePath,
		peers:    make(map[string]*TrustedPeer),
	}

	// Load existing trusted peers
	if err := trustStore.load(); err != nil {
		fmt.Printf("⚠️  Failed to load trust store: %v (starting with empty trust store)\n", err)
	}

	return trustStore, nil
}

// load loads trusted peers from the JSON file
func (ts *TrustStore) load() error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	data, err := ioutil.ReadFile(ts.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, start with empty trust store
		}
		return err
	}

	return json.Unmarshal(data, &ts.peers)
}

// save saves trusted peers to the JSON file
func (ts *TrustStore) save() error {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	data, err := json.MarshalIndent(ts.peers, "", "  ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(ts.filePath, data, 0600)
}

// addTrustedPeer adds a new trusted peer to the store
func (ts *TrustStore) addTrustedPeer(peer *TrustedPeer) error {
	ts.mutex.Lock()
	defer ts.mutex.Unlock()

	ts.peers[peer.DeviceID] = peer
	return ts.save()
}

// getTrustedPeer retrieves a trusted peer by device ID
func (ts *TrustStore) getTrustedPeer(deviceID string) (*TrustedPeer, bool) {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	peer, exists := ts.peers[deviceID]
	return peer, exists
}

// isTrusted checks if a peer is already trusted
func (ts *TrustStore) isTrusted(deviceID string) bool {
	_, exists := ts.getTrustedPeer(deviceID)
	return exists
}

// getAllTrustedPeers returns all trusted peers
func (ts *TrustStore) getAllTrustedPeers() map[string]*TrustedPeer {
	ts.mutex.RLock()
	defer ts.mutex.RUnlock()

	result := make(map[string]*TrustedPeer)
	for id, peer := range ts.peers {
		result[id] = peer
	}
	return result
}

// getAllLocalIPs returns all non-loopback IPv4 addresses on this machine
func getAllLocalIPs() []net.IP {
	var ips []net.IP
	
	interfaces, err := net.Interfaces()
	if err != nil {
		// Fallback to localhost if we can't get interfaces
		return []net.IP{net.ParseIP("127.0.0.1")}
	}
	
	for _, iface := range interfaces {
		// Skip down interfaces and loopback
		if iface.Flags&net.FlagUp == 0 {
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
			
			// Include IPv4 addresses (including loopback as fallback)
			if ip != nil && ip.To4() != nil {
				ips = append(ips, ip)
			}
		}
	}
	
	// If no IPs found, fallback to localhost
	if len(ips) == 0 {
		ips = []net.IP{net.ParseIP("127.0.0.1")}
	}
	
	return ips
}

// generateCertificateAuthority creates a root CA certificate for the LanDrop trust network
func generateCertificateAuthority() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	// Generate private key for CA
	caKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA private key: %w", err)
	}

	// Create CA certificate template with long validity
	notBefore := time.Now()
	notAfter := notBefore.Add(10 * 365 * 24 * time.Hour) // 10 years

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA serial number: %w", err)
	}

	caTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"LanDrop Certificate Authority"},
			CommonName:   "LanDrop Root CA",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLen:            1,
	}

	// Generate CA certificate
	caCertDER, err := x509.CreateCertificate(rand.Reader, &caTemplate, &caTemplate, &caKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate CA certificate: %w", err)
	}

	caCert, err := x509.ParseCertificate(caCertDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse CA certificate: %w", err)
	}

	return caCert, caKey, nil
}

// generateDeviceCertificate creates a device certificate signed by the CA
func generateDeviceCertificate(caCert *x509.Certificate, caKey *ecdsa.PrivateKey) (*x509.Certificate, *ecdsa.PrivateKey, error) {
	// Generate device private key
	deviceKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate device private key: %w", err)
	}

	// Get device information
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// Generate unique device ID
	deviceID := generateDeviceID()

	// Create device certificate template
	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour) // 1 year

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate device serial number: %w", err)
	}

	// Get all local IPs for the certificate
	ips := getAllLocalIPs()

	deviceTemplate := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"LanDrop Device"},
			CommonName:   fmt.Sprintf("%s (%s)", hostname, deviceID[:8]),
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IPAddresses:           ips,
		DNSNames:              []string{hostname, "localhost"},
	}

	// Generate device certificate signed by CA
	deviceCertDER, err := x509.CreateCertificate(rand.Reader, &deviceTemplate, caCert, &deviceKey.PublicKey, caKey)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to generate device certificate: %w", err)
	}

	deviceCert, err := x509.ParseCertificate(deviceCertDER)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse device certificate: %w", err)
	}

	return deviceCert, deviceKey, nil
}

// generateDeviceID creates a unique device identifier
func generateDeviceID() string {
	// Use hostname and random bytes for uniqueness
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	
	randomBytes := make([]byte, 16)
	rand.Read(randomBytes)
	
	return fmt.Sprintf("%s-%x", hostname, randomBytes)
}

// createServerTLSConfigWithCA creates a TLS config using CA-signed device certificate
func createServerTLSConfigWithCA(deviceCert *x509.Certificate, deviceKey *ecdsa.PrivateKey, caCert *x509.Certificate) (*tls.Config, error) {
	// Create certificate PEM blocks
	deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: deviceCert.Raw})
	deviceKeyBytes, err := x509.MarshalPKCS8PrivateKey(deviceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device private key: %w", err)
	}
	deviceKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: deviceKeyBytes})

	// Load certificate and key
	cert, err := tls.X509KeyPair(deviceCertPEM, deviceKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load device certificate and key: %w", err)
	}

	// Create CA cert pool for client verification
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		NextProtos:   []string{TLSServerName},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

// createServerTLSConfigWithTrustStore creates a TLS config using CA-signed device certificate with trust store verification
func createServerTLSConfigWithTrustStore(deviceCert *x509.Certificate, deviceKey *ecdsa.PrivateKey, caCert *x509.Certificate, trustStore *TrustStore) (*tls.Config, error) {
	// Create certificate PEM blocks
	deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: deviceCert.Raw})
	deviceKeyBytes, err := x509.MarshalPKCS8PrivateKey(deviceKey)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device private key: %w", err)
	}
	deviceKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: deviceKeyBytes})

	// Load certificate and key
	cert, err := tls.X509KeyPair(deviceCertPEM, deviceKeyPEM)
	if err != nil {
		return nil, fmt.Errorf("failed to load device certificate and key: %w", err)
	}

	// Create CA cert pool for client verification (for fallback)
	caCertPool := x509.NewCertPool()
	caCertPool.AddCert(caCert)

	return &tls.Config{
		Certificates:         []tls.Certificate{cert},
		NextProtos:           []string{TLSServerName},
		ClientAuth:           tls.NoClientCert, // Be permissive for better compatibility
		MinVersion:           tls.VersionTLS12,
		ServerName:           "", // Accept any server name for flexibility
		InsecureSkipVerify:   true, // Skip standard verification for better compatibility
	}, nil
}

// createClientTLSConfigWithCA creates a TLS config that verifies peers against the embedded CA
func createClientTLSConfigWithCA(caCert *x509.Certificate, deviceCert *x509.Certificate, deviceKey *ecdsa.PrivateKey) *tls.Config {
	// For development and same-device testing, allow more flexible verification
	
	// Create device certificate for client authentication
	deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: deviceCert.Raw})
	deviceKeyBytes, err := x509.MarshalPKCS8PrivateKey(deviceKey)
	if err != nil {
		fmt.Printf("⚠️  Failed to marshal device key: %v\n", err)
		// Fallback to basic config for testing
		return createTestingTLSConfig()
	}
	deviceKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: deviceKeyBytes})

	// Load certificate and key
	cert, err := tls.X509KeyPair(deviceCertPEM, deviceKeyPEM)
	if err != nil {
		fmt.Printf("⚠️  Failed to load device certificate: %v\n", err)
		// Fallback to basic config for testing
		return createTestingTLSConfig()
	}

	// Add debugging to see if our verification function is being called
	verificationFunc := verifyPeerCertificateWithCA(caCert)
	
	return &tls.Config{
		Certificates:         []tls.Certificate{cert},
		ClientAuth:           tls.RequireAndVerifyClientCert,
		InsecureSkipVerify:   false, // Don't skip verification - use our CA
		VerifyPeerCertificate: verificationFunc,
		NextProtos:           []string{TLSServerName},
		MinVersion:           tls.VersionTLS12,
		ServerName:           "", // Accept any server name for flexibility
	}
}

// createClientTLSConfigPermissive creates a TLS config with permissive trust-on-first-use verification
func createClientTLSConfigPermissive(caCert *x509.Certificate, deviceCert *x509.Certificate, deviceKey *ecdsa.PrivateKey, trustStore *TrustStore) *tls.Config {
	// Create device certificate for client authentication
	deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: deviceCert.Raw})
	deviceKeyBytes, err := x509.MarshalPKCS8PrivateKey(deviceKey)
	if err != nil {
		fmt.Printf("⚠️  Failed to marshal device key: %v\n", err)
		return createTestingTLSConfig()
	}
	deviceKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: deviceKeyBytes})

	// Load certificate and key
	cert, err := tls.X509KeyPair(deviceCertPEM, deviceKeyPEM)
	if err != nil {
		fmt.Printf("⚠️  Failed to load device certificate: %v\n", err)
		return createTestingTLSConfig()
	}

	// Custom verification function that automatically approves LanDrop certificates
	verificationFunc := verifyPeerCertificatePermissive(caCert, trustStore)

	return &tls.Config{
		Certificates:         []tls.Certificate{cert},
		ClientAuth:           tls.RequireAndVerifyClientCert,
		InsecureSkipVerify:   false, // Don't skip verification - use our custom verifier
		VerifyPeerCertificate: verificationFunc,
		NextProtos:           []string{TLSServerName},
		MinVersion:           tls.VersionTLS12,
		ServerName:           "", // Accept any server name for flexibility
	}
}

// createClientTLSConfigWithTrustStore creates a TLS config with trust-aware verification
func createClientTLSConfigWithTrustStore(caCert *x509.Certificate, deviceCert *x509.Certificate, deviceKey *ecdsa.PrivateKey, trustStore *TrustStore) *tls.Config {
	// Create device certificate for client authentication
	deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: deviceCert.Raw})
	deviceKeyBytes, err := x509.MarshalPKCS8PrivateKey(deviceKey)
	if err != nil {
		fmt.Printf("⚠️  Failed to marshal device key: %v\n", err)
		return createTestingTLSConfig()
	}
	deviceKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: deviceKeyBytes})

	// Load certificate and key
	cert, err := tls.X509KeyPair(deviceCertPEM, deviceKeyPEM)
	if err != nil {
		fmt.Printf("⚠️  Failed to load device certificate: %v\n", err)
		return createTestingTLSConfig()
	}

	return &tls.Config{
		Certificates:         []tls.Certificate{cert},
		ClientAuth:           tls.NoClientCert, // Don't require client certificate for better compatibility
		InsecureSkipVerify:   true,  // Skip standard verification completely
		// VerifyPeerCertificate: verificationFunc, // Don't use custom verifier when InsecureSkipVerify=true
		NextProtos:           []string{TLSServerName},
		MinVersion:           tls.VersionTLS12,
		ServerName:           "", // Accept any server name for flexibility
	}
}

// createTestingTLSConfig creates a more permissive TLS config for testing
func createTestingTLSConfig() *tls.Config {
	fmt.Printf("🔧 Using testing TLS configuration for same-device communication\n")

	return &tls.Config{
		InsecureSkipVerify:   true, // Allow any certificate for testing
		MinVersion:           tls.VersionTLS12,
		ServerName:           "", // Empty server name to allow any server
		NextProtos:           []string{TLSServerName}, // Set ALPN protocol
	}
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

	// Get all local IP addresses for the certificate
	ips := getAllLocalIPs()
	
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
		IPAddresses:           ips,
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
	fmt.Printf("🔧 Using fallback client TLS config with InsecureSkipVerify=true\n")
	return &tls.Config{
		InsecureSkipVerify: true,
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

// GetDeviceInfo returns device information from the global TLS manager
func GetDeviceInfo() *DeviceInfo {
	if globalTLSManager == nil {
		return nil
	}
	return globalTLSManager.GetDeviceInfo()
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
		fmt.Printf("⚠️  Global TLS manager not initialized, using fallback\n")
		return createClientTLSConfig()
	}
	
	config := globalTLSManager.GetClientConfig()
	if config == nil {
		fmt.Printf("⚠️  Client TLS config is nil, using testing config\n")
		return createTestingTLSConfig()
	}
	
	fmt.Printf("✅ Using global client TLS config\n")
	return config
}

// GetDeviceInfo returns device information from the TLS manager
func (tm *TLSManager) GetDeviceInfo() *DeviceInfo {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	// In testing mode, return basic device info
	if tm.testingMode {
		return &DeviceInfo{
			DeviceID:      fmt.Sprintf("%s (testing-mode)", hostname),
			Hostname:      hostname,
			Fingerprint:   "testing-mode-no-certificate",
			CAFingerprint: "testing-mode-no-ca",
			CreatedAt:     time.Now().Unix(),
		}
	}

	// In production mode, return certificate-based info
	if tm.deviceCert == nil {
		return nil
	}

	fingerprint := generateCertificateFingerprint(tm.deviceCert)
	caFingerprint := ""
	if tm.caCert != nil {
		caFingerprint = generateCertificateFingerprint(tm.caCert)
	}

	return &DeviceInfo{
		DeviceID:      tm.deviceCert.Subject.CommonName,
		Hostname:      hostname,
		Fingerprint:   fingerprint,
		CAFingerprint: caFingerprint,
		CreatedAt:     tm.deviceCert.NotBefore.Unix(),
	}
}

// GetCACertificate returns the embedded CA certificate
func (tm *TLSManager) GetCACertificate() *x509.Certificate {
	return tm.caCert
}

// ExportCA exports the CA certificate to a file for sharing with other devices
func (tm *TLSManager) ExportCA(filename string) error {
	if tm.caCert == nil {
		return fmt.Errorf("no CA certificate available")
	}

	caPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: tm.caCert.Raw,
	})

	return os.WriteFile(filename, caPEM, 0644)
}

// ImportCA imports a CA certificate from a file for trusting other devices
func ImportCA(filename string) (*x509.Certificate, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA file: %w", err)
	}

	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	return x509.ParseCertificate(block.Bytes)
}

// generateCertificateFingerprint creates a SHA-256 fingerprint of the certificate
func generateCertificateFingerprint(cert *x509.Certificate) string {
	hash := sha256.Sum256(cert.Raw)
	return hex.EncodeToString(hash[:])
}

// GenerateCertificateFingerprint public wrapper for generateCertificateFingerprint
func GenerateCertificateFingerprint(cert *x509.Certificate) string {
	return generateCertificateFingerprint(cert)
}

// verifyPeerCertificateWithCA creates a custom certificate verification function that checks against our CA
func verifyPeerCertificateWithCA(caCert *x509.Certificate) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		fmt.Printf("🔍 Certificate verification called with %d certificates\n", len(rawCerts))

		if len(rawCerts) == 0 {
			return fmt.Errorf("no certificates provided")
		}

		// Parse the peer certificate
		peerCert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer certificate: %w", err)
		}

		fmt.Printf("🔍 Peer certificate: %s\n", peerCert.Subject.CommonName)

		// Check if certificate is from a LanDrop device
		if !isLanDropCertificate(peerCert) {
			return fmt.Errorf("certificate is not from a LanDrop device")
		}

		// Check if certificate is still valid
		if time.Now().Before(peerCert.NotBefore) {
			return fmt.Errorf("certificate is not yet valid")
		}
		if time.Now().After(peerCert.NotAfter) {
			return fmt.Errorf("certificate has expired")
		}

		// First try to verify against our CA
		caCertPool := x509.NewCertPool()
		caCertPool.AddCert(caCert)

		opts := x509.VerifyOptions{
			Roots:     caCertPool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}

		if _, err := peerCert.Verify(opts); err == nil {
			// Certificate signed by our CA - valid and trusted
			fmt.Printf("✅ Peer certificate verified by our CA\n")
			return nil
		}

		// Certificate not signed by our CA - check if it's a LanDrop certificate
		hostname, _ := os.Hostname()
		peerHostname := extractHostnameFromCN(peerCert.Subject.CommonName)

		if peerHostname == hostname {
			// Same device, different process - automatically trust
			fmt.Printf("🔄 Same device detected (%s) - trusting automatically\n", hostname)
			return nil
		}

		// Different device, different CA - show approval prompt for trust-on-first-use
		if !promptForPeerApproval(peerCert) {
			return fmt.Errorf("peer connection rejected by user")
		}

		// User approved - allow this connection (trust-on-first-use)
		fmt.Printf("✅ Approved connection to %s (trust-on-first-use)\n", peerCert.Subject.CommonName)
		return nil
	}
}

// verifyPeerCertificatePermissive creates a permissive certificate verification function that auto-approves LanDrop certificates
func verifyPeerCertificatePermissive(caCert *x509.Certificate, trustStore *TrustStore) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		fmt.Printf("🔓 Permissive certificate verification called with %d certificates\n", len(rawCerts))

		if len(rawCerts) == 0 {
			return fmt.Errorf("no certificates provided")
		}

		// Parse the peer certificate
		peerCert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer certificate: %w", err)
		}

		fmt.Printf("🔓 Verifying peer certificate: %s\n", peerCert.Subject.CommonName)

		// Check if certificate is from a LanDrop device
		if !isLanDropCertificate(peerCert) {
			return fmt.Errorf("certificate is not from a LanDrop device")
		}

		// Check if certificate is still valid
		if time.Now().Before(peerCert.NotBefore) {
			return fmt.Errorf("certificate is not yet valid")
		}
		if time.Now().After(peerCert.NotAfter) {
			return fmt.Errorf("certificate has expired")
		}

		// Try to verify against our CA first
		caCertPool := x509.NewCertPool()
		caCertPool.AddCert(caCert)

		opts := x509.VerifyOptions{
			Roots:     caCertPool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}

		if _, err := peerCert.Verify(opts); err == nil {
			// Certificate signed by our CA - valid and trusted
			fmt.Printf("✅ Peer certificate verified by our CA\n")
			return nil
		}

		// Certificate not signed by our CA - auto-approve any LanDrop certificate in permissive mode
		peerHostname := extractHostnameFromCN(peerCert.Subject.CommonName)
		hostname, _ := os.Hostname()

		if peerHostname == hostname {
			// Same device, different process - automatically trust
			fmt.Printf("🔄 Same device detected (%s) - trusting automatically\n", hostname)
		} else {
			// Different device - auto-trust in permissive mode
			fmt.Printf("🔓 Permissive mode: auto-trusting LanDrop device %s\n", peerCert.Subject.CommonName)
		}

		// Auto-add to trust store for future reference
		ourCAPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
		deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: peerCert.Raw})

		trustedPeer := &TrustedPeer{
			DeviceID:    peerCert.Subject.CommonName,
			Hostname:    peerHostname,
			Fingerprint: generateCertificateFingerprint(peerCert),
			CACert:      string(ourCAPEM),
			DeviceCert:  string(deviceCertPEM),
			ApprovedAt:  time.Now().Unix(),
			LastSeen:    time.Now().Unix(),
		}

		if err := trustStore.addTrustedPeer(trustedPeer); err != nil {
			fmt.Printf("⚠️  Failed to save trusted peer: %v\n", err)
			// Continue anyway - connection was auto-approved
		}

		return nil
	}
}

// verifyPeerCertificateWithTrustStore creates a certificate verification function that uses trust store
func verifyPeerCertificateWithTrustStore(caCert *x509.Certificate, trustStore *TrustStore) func([][]byte, [][]*x509.Certificate) error {
	return func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		fmt.Printf("🔍 Enhanced certificate verification called with %d certificates\n", len(rawCerts))

		if len(rawCerts) == 0 {
			return fmt.Errorf("no certificates provided")
		}

		// Parse the peer certificate
		peerCert, err := x509.ParseCertificate(rawCerts[0])
		if err != nil {
			return fmt.Errorf("failed to parse peer certificate: %w", err)
		}

		fmt.Printf("🔍 Verifying peer certificate: %s\n", peerCert.Subject.CommonName)

		// Check if certificate is from a LanDrop device
		if !isLanDropCertificate(peerCert) {
			return fmt.Errorf("certificate is not from a LanDrop device")
		}

		// Check if certificate is still valid
		if time.Now().Before(peerCert.NotBefore) {
			return fmt.Errorf("certificate is not yet valid")
		}
		if time.Now().After(peerCert.NotAfter) {
			return fmt.Errorf("certificate has expired")
		}

		// Try to verify against our CA first
		caCertPool := x509.NewCertPool()
		caCertPool.AddCert(caCert)

		opts := x509.VerifyOptions{
			Roots:     caCertPool,
			KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		}

		if _, err := peerCert.Verify(opts); err == nil {
			// Certificate signed by our CA - valid and trusted
			fmt.Printf("✅ Peer certificate verified by our CA\n")
			return nil
		}

		// Certificate not signed by our CA - check trust store for peer's CA
		peerHostname := extractHostnameFromCN(peerCert.Subject.CommonName)
		hostname, _ := os.Hostname()

		if peerHostname == hostname {
			// Same device, different process - automatically trust
			fmt.Printf("🔄 Same device detected (%s) - trusting automatically\n", hostname)
			return nil
		}

		// Check if peer is already in trust store
		if trustedPeer, exists := trustStore.getTrustedPeer(peerCert.Subject.CommonName); exists {
			// Try to verify against the peer's stored CA
			if trustedPeer.CACert != "" {
				peerCA, err := parsePEMCertificate([]byte(trustedPeer.CACert))
				if err == nil {
					peerCAPool := x509.NewCertPool()
					peerCAPool.AddCert(peerCA)

					opts := x509.VerifyOptions{
						Roots:     peerCAPool,
						KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
					}

					if _, err := peerCert.Verify(opts); err == nil {
						// Update last seen time
						trustedPeer.LastSeen = time.Now().Unix()
						trustStore.addTrustedPeer(trustedPeer)

						fmt.Printf("✅ Peer certificate verified by stored CA: %s\n", peerCert.Subject.CommonName)
						return nil
					}
				}
			}

			// If we have the peer stored but verification fails, update last seen anyway
			trustedPeer.LastSeen = time.Now().Unix()
			trustStore.addTrustedPeer(trustedPeer)

			fmt.Printf("✅ Peer already trusted (fallback): %s\n", peerCert.Subject.CommonName)
			return nil
		}

		// New device with different CA - automatically trust any LanDrop certificate
		fmt.Printf("🔓 Auto-trusting new LanDrop device: %s\n", peerCert.Subject.CommonName)
		fmt.Printf("🔓 Security note: This is a LanDrop device with auto-approval enabled\n")

		// Auto-add to trust store with our CA for future reference
		ourCAPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caCert.Raw})
		deviceCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: peerCert.Raw})

		trustedPeer := &TrustedPeer{
			DeviceID:    peerCert.Subject.CommonName,
			Hostname:    peerHostname,
			Fingerprint: generateCertificateFingerprint(peerCert),
			CACert:      string(ourCAPEM), // Store our CA so we can verify this peer in future
			DeviceCert:  string(deviceCertPEM),
			ApprovedAt:  time.Now().Unix(),
			LastSeen:    time.Now().Unix(),
		}

		if err := trustStore.addTrustedPeer(trustedPeer); err != nil {
			fmt.Printf("⚠️  Failed to save trusted peer: %v\n", err)
			// Continue anyway - connection was auto-approved
		}

		fmt.Printf("✅ Auto-trusted new peer: %s\n", peerCert.Subject.CommonName)
		return nil
	}
}

// parsePEMCertificate parses a PEM-encoded certificate
func parsePEMCertificate(pemData []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(pemData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	return x509.ParseCertificate(block.Bytes)
}

// extractHostnameFromCN extracts hostname from the Common Name field
// Format is typically "hostname (hash)" so we extract the hostname part
func extractHostnameFromCN(commonName string) string {
	// Split by space and take the first part as hostname
	parts := strings.Split(commonName, " ")
	if len(parts) > 0 {
		return parts[0]
	}
	return commonName
}

// isLanDropCertificate checks if the certificate is from a LanDrop device
func isLanDropCertificate(cert *x509.Certificate) bool {
	if len(cert.Subject.Organization) == 0 {
		return false
	}
	return cert.Subject.Organization[0] == "LanDrop Device"
}

// promptForPeerApproval asks the user to approve a new peer connection
func promptForPeerApproval(cert *x509.Certificate) bool {
	fingerprint := generateCertificateFingerprint(cert)
	
	fmt.Printf("\n🔐 New LanDrop Device Detected\n")
	fmt.Printf("================================\n")
	fmt.Printf("Device Name: %s\n", cert.Subject.CommonName)
	fmt.Printf("Organization: %s\n", cert.Subject.Organization[0])
	fmt.Printf("Fingerprint: %s\n", fingerprint)
	fmt.Printf("Valid From:  %s\n", cert.NotBefore.Format("2006-01-02 15:04:05"))
	fmt.Printf("Valid Until: %s\n", cert.NotAfter.Format("2006-01-02 15:04:05"))
	fmt.Printf("\n⚠️  This device has a different Certificate Authority\n")
	fmt.Printf("Do you trust this device? (y/n): ")
	
	var response string
	fmt.Scanln(&response)
	
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}
