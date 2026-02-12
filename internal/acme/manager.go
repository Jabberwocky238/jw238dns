package acme

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"jabberwocky238/jw238dns/internal/storage"

	"github.com/go-acme/lego/v4/certificate"
)

// Manager manages certificate lifecycle including automatic renewal.
type Manager struct {
	client        *Client
	certStorage   CertificateStorage
	config        *Config
	certificates  map[string]*ManagedCertificate
	mu            sync.RWMutex
	stopCh        chan struct{}
	renewalTicker *time.Ticker
}

// ManagedCertificate represents a certificate being managed.
type ManagedCertificate struct {
	Domains      []string
	Certificate  []byte
	PrivateKey   []byte
	IssuerCert   []byte
	CSR          []byte
	CertURL      string
	NotBefore    time.Time
	NotAfter     time.Time
	NeedsRenewal bool
}

// NewManager creates a new certificate manager.
func NewManager(config *Config, store storage.CoreStorage, certStorage CertificateStorage) (*Manager, error) {
	client, err := NewClient(config, store)
	if err != nil {
		return nil, fmt.Errorf("failed to create ACME client: %w", err)
	}

	// Register ACME account if needed
	if err := client.Register(); err != nil {
		slog.Warn("ACME registration failed (may already be registered)", "error", err)
	}

	return &Manager{
		client:       client,
		certStorage:  certStorage,
		config:       config,
		certificates: make(map[string]*ManagedCertificate),
		stopCh:       make(chan struct{}),
	}, nil
}

// ObtainCertificate requests a new certificate for the given domains.
func (m *Manager) ObtainCertificate(ctx context.Context, domains []string) error {
	if len(domains) == 0 {
		return fmt.Errorf("no domains specified")
	}

	slog.Info("obtaining certificate", "domains", domains)

	cert, err := m.client.ObtainCertificate(domains)
	if err != nil {
		return fmt.Errorf("failed to obtain certificate: %w", err)
	}

	// Parse certificate to get expiry info
	managed, err := m.parseCertificate(cert)
	if err != nil {
		return fmt.Errorf("failed to parse certificate: %w", err)
	}

	// Store certificate
	if err := m.certStorage.Store(ctx, domains[0], cert); err != nil {
		return fmt.Errorf("failed to store certificate: %w", err)
	}

	// Track certificate
	m.mu.Lock()
	m.certificates[domains[0]] = managed
	m.mu.Unlock()

	slog.Info("certificate obtained and stored", "domains", domains, "expires", managed.NotAfter)
	return nil
}

// RenewCertificate renews a certificate for the given domain.
func (m *Manager) RenewCertificate(ctx context.Context, domain string) error {
	m.mu.RLock()
	managed, exists := m.certificates[domain]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("certificate not found for domain: %s", domain)
	}

	slog.Info("renewing certificate", "domain", domain)

	// Load existing certificate resource
	certResource := &certificate.Resource{
		Domain:      domain,
		Certificate: managed.Certificate,
		PrivateKey:  managed.PrivateKey,
		CSR:         managed.CSR,
		CertURL:     managed.CertURL,
	}

	renewed, err := m.client.RenewCertificate(certResource)
	if err != nil {
		return fmt.Errorf("failed to renew certificate: %w", err)
	}

	// Parse renewed certificate
	renewedManaged, err := m.parseCertificate(renewed)
	if err != nil {
		return fmt.Errorf("failed to parse renewed certificate: %w", err)
	}

	// Store renewed certificate
	if err := m.certStorage.Store(ctx, domain, renewed); err != nil {
		return fmt.Errorf("failed to store renewed certificate: %w", err)
	}

	// Update tracked certificate
	m.mu.Lock()
	m.certificates[domain] = renewedManaged
	m.mu.Unlock()

	slog.Info("certificate renewed and stored", "domain", domain, "expires", renewedManaged.NotAfter)
	return nil
}

// StartAutoRenewal starts the automatic renewal process.
func (m *Manager) StartAutoRenewal(ctx context.Context) {
	if !m.config.AutoRenew {
		slog.Info("auto-renewal disabled")
		return
	}

	slog.Info("starting auto-renewal", "check_interval", m.config.CheckInterval, "renew_before", m.config.RenewBefore)

	m.renewalTicker = time.NewTicker(m.config.CheckInterval)

	go func() {
		for {
			select {
			case <-ctx.Done():
				m.renewalTicker.Stop()
				return
			case <-m.stopCh:
				m.renewalTicker.Stop()
				return
			case <-m.renewalTicker.C:
				m.checkAndRenew(ctx)
			}
		}
	}()
}

// StopAutoRenewal stops the automatic renewal process.
func (m *Manager) StopAutoRenewal() {
	close(m.stopCh)
}

// checkAndRenew checks all certificates and renews those that need renewal.
func (m *Manager) checkAndRenew(ctx context.Context) {
	m.mu.RLock()
	domains := make([]string, 0, len(m.certificates))
	for domain := range m.certificates {
		domains = append(domains, domain)
	}
	m.mu.RUnlock()

	for _, domain := range domains {
		m.mu.RLock()
		cert := m.certificates[domain]
		m.mu.RUnlock()

		// Check if renewal is needed
		timeUntilExpiry := time.Until(cert.NotAfter)
		if timeUntilExpiry <= m.config.RenewBefore {
			slog.Info("certificate needs renewal", "domain", domain, "expires_in", timeUntilExpiry)
			if err := m.RenewCertificate(ctx, domain); err != nil {
				slog.Error("failed to renew certificate", "domain", domain, "error", err)
			}
		}
	}
}

// parseCertificate parses a certificate resource into a managed certificate.
func (m *Manager) parseCertificate(cert *certificate.Resource) (*ManagedCertificate, error) {
	block, _ := pem.Decode(cert.Certificate)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM certificate")
	}

	x509Cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse x509 certificate: %w", err)
	}

	// Extract issuer certificate from the full chain
	issuerCert := extractIssuerCert(cert.Certificate)

	return &ManagedCertificate{
		Domains:     []string{cert.Domain},
		Certificate: cert.Certificate,
		PrivateKey:  cert.PrivateKey,
		IssuerCert:  issuerCert,
		CSR:         cert.CSR,
		CertURL:     cert.CertURL,
		NotBefore:   x509Cert.NotBefore,
		NotAfter:    x509Cert.NotAfter,
	}, nil
}

// extractIssuerCert extracts the issuer certificate from a certificate chain.
func extractIssuerCert(certChain []byte) []byte {
	// The certificate chain typically contains the leaf cert followed by intermediates
	// We'll extract everything after the first certificate as the issuer chain
	var issuerCerts []byte
	rest := certChain
	first := true

	for {
		block, remaining := pem.Decode(rest)
		if block == nil {
			break
		}
		if !first {
			issuerCerts = append(issuerCerts, pem.EncodeToMemory(block)...)
		}
		first = false
		rest = remaining
	}

	return issuerCerts
}

// LoadCertificates loads existing certificates from storage.
func (m *Manager) LoadCertificates(ctx context.Context, domains []string) error {
	for _, domain := range domains {
		cert, err := m.certStorage.Load(ctx, domain)
		if err != nil {
			slog.Warn("failed to load certificate", "domain", domain, "error", err)
			continue
		}

		managed, err := m.parseCertificate(cert)
		if err != nil {
			slog.Warn("failed to parse loaded certificate", "domain", domain, "error", err)
			continue
		}

		m.mu.Lock()
		m.certificates[domain] = managed
		m.mu.Unlock()

		slog.Info("loaded certificate", "domain", domain, "expires", managed.NotAfter)
	}

	return nil
}

// GetCertificate returns the managed certificate for a domain.
func (m *Manager) GetCertificate(domain string) (*ManagedCertificate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cert, exists := m.certificates[domain]
	return cert, exists
}

// ListCertificates returns all managed certificates.
func (m *Manager) ListCertificates() map[string]*ManagedCertificate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*ManagedCertificate, len(m.certificates))
	for k, v := range m.certificates {
		result[k] = v
	}
	return result
}
