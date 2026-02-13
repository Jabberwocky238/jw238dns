package acme

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"fmt"
	"log/slog"

	"jabberwocky238/jw238dns/storage"

	"github.com/go-acme/lego/v4/certcrypto"
	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/lego"
	"github.com/go-acme/lego/v4/registration"
)

const (
	ClusterDomain = "jw238dns.jw238dns.svc.cluster.local:53"
)

// Client wraps the lego ACME client with our configuration.
type Client struct {
	config      *Config
	legoClient  *lego.Client
	user        *User
	dnsProvider *DNS01Provider
}

// NewClient creates a new ACME client.
func NewClient(config *Config, store storage.CoreStorage) (*Client, error) {
	if config == nil {
		config = DefaultConfig()
	}

	// Generate user private key
	privateKey, err := generatePrivateKey(config.KeyType)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	user := &User{
		Email: config.Email,
		key:   privateKey,
	}

	// Create lego config
	legoConfig := lego.NewConfig(user)
	legoConfig.CADirURL = config.ServerURL
	legoConfig.Certificate.KeyType = parseKeyType(config.KeyType)

	// Create lego client
	legoClient, err := lego.NewClient(legoConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create lego client: %w", err)
	}

	// Create DNS-01 provider
	dnsProvider := NewDNS01Provider(store)
	dnsProvider.SetPropagationWait(config.PropagationWait)

	// Set DNS-01 challenge provider with custom DNS servers
	// Use jw238dns service for DNS-01 validation
	// This allows ACME to query our own DNS server for _acme-challenge records
	if err := legoClient.Challenge.SetDNS01Provider(dnsProvider,
		dns01.AddRecursiveNameservers([]string{ClusterDomain}),
	); err != nil {
		return nil, fmt.Errorf("failed to set DNS-01 provider: %w", err)
	}

	client := &Client{
		config:      config,
		legoClient:  legoClient,
		user:        user,
		dnsProvider: dnsProvider,
	}

	return client, nil
}

// Register registers a new ACME account.
// If EAB credentials are configured, it uses External Account Binding registration
// (required by providers like ZeroSSL). Otherwise, it uses plain registration
// (for Let's Encrypt and other providers that don't require EAB).
func (c *Client) Register() error {
	var reg *registration.Resource
	var err error

	if c.config.EAB.KID != "" {
		reg, err = c.legoClient.Registration.RegisterWithExternalAccountBinding(registration.RegisterEABOptions{
			TermsOfServiceAgreed: true,
			Kid:                  c.config.EAB.KID,
			HmacEncoded:          c.config.EAB.HMACKey,
		})
		if err != nil {
			return fmt.Errorf("failed to register ACME account with EAB: %w", err)
		}
		slog.Info("ACME account registered with EAB", "email", c.user.Email, "uri", reg.URI)
	} else {
		reg, err = c.legoClient.Registration.Register(registration.RegisterOptions{
			TermsOfServiceAgreed: true,
		})
		if err != nil {
			return fmt.Errorf("failed to register ACME account: %w", err)
		}
		slog.Info("ACME account registered", "email", c.user.Email, "uri", reg.URI)
	}

	c.user.Registration = reg
	return nil
}

// ObtainCertificate requests a new certificate for the given domains.
func (c *Client) ObtainCertificate(domains []string) (*certificate.Resource, error) {
	if len(domains) == 0 {
		return nil, fmt.Errorf("no domains specified")
	}

	request := certificate.ObtainRequest{
		Domains: domains,
		Bundle:  true,
	}

	cert, err := c.legoClient.Certificate.Obtain(request)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain certificate: %w", err)
	}

	slog.Info("certificate obtained", "domains", domains, "url", cert.CertURL)
	return cert, nil
}

// RenewCertificate renews an existing certificate.
func (c *Client) RenewCertificate(cert *certificate.Resource) (*certificate.Resource, error) {
	renewed, err := c.legoClient.Certificate.Renew(*cert, true, false, "")
	if err != nil {
		return nil, fmt.Errorf("failed to renew certificate: %w", err)
	}

	slog.Info("certificate renewed", "domain", cert.Domain, "url", renewed.CertURL)
	return renewed, nil
}

// RevokeCertificate revokes a certificate.
func (c *Client) RevokeCertificate(cert []byte) error {
	if err := c.legoClient.Certificate.Revoke(cert); err != nil {
		return fmt.Errorf("failed to revoke certificate: %w", err)
	}

	slog.Info("certificate revoked")
	return nil
}

// generatePrivateKey generates a private key based on the key type.
func generatePrivateKey(keyType string) (crypto.PrivateKey, error) {
	switch keyType {
	case "RSA2048":
		return rsa.GenerateKey(rand.Reader, 2048)
	case "RSA4096":
		return rsa.GenerateKey(rand.Reader, 4096)
	case "EC256":
		return ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	case "EC384":
		return ecdsa.GenerateKey(elliptic.P384(), rand.Reader)
	default:
		return rsa.GenerateKey(rand.Reader, 2048)
	}
}

// parseKeyType converts string key type to certcrypto.KeyType.
func parseKeyType(keyType string) certcrypto.KeyType {
	switch keyType {
	case "RSA2048":
		return certcrypto.RSA2048
	case "RSA4096":
		return certcrypto.RSA4096
	case "EC256":
		return certcrypto.EC256
	case "EC384":
		return certcrypto.EC384
	default:
		return certcrypto.RSA2048
	}
}
