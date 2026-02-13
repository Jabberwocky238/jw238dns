package acme

import "time"

const (
	// DefaultCheckInterval is how often to check for certificates needing renewal
	DefaultCheckInterval = 24 * time.Hour

	// DefaultRenewBefore is how long before expiry to renew certificates
	DefaultRenewBefore = 30 * 24 * time.Hour // 30 days

	// DefaultPropagationWait is the default wait time for DNS propagation.
	// Short wait since records are immediately available in memory.
	DefaultPropagationWait = 2 * time.Second
)

// EABConfig holds External Account Binding credentials for ACME providers
// that require it (e.g., ZeroSSL).
type EABConfig struct {
	// KID is the EAB Key Identifier
	KID string

	// HMACKey is the EAB HMAC key
	HMACKey string
}

// Config holds the ACME client configuration.
type Config struct {
	// ServerURL is the ACME directory URL (e.g., Let's Encrypt production/staging)
	ServerURL string

	// Email is the account email for ACME registration
	Email string

	// KeyType specifies the certificate key type (RSA2048, RSA4096, EC256, EC384)
	KeyType string

	// AutoRenew enables automatic certificate renewal
	AutoRenew bool

	// CheckInterval is how often to check for certificates needing renewal
	CheckInterval time.Duration

	// RenewBefore is how long before expiry to renew certificates
	RenewBefore time.Duration

	// PropagationWait is how long to wait for DNS propagation
	PropagationWait time.Duration

	// Storage configuration for certificates
	Storage StorageConfig

	// EAB holds External Account Binding credentials (optional, required for ZeroSSL)
	EAB EABConfig
}

// StorageConfig defines where and how to store certificates.
type StorageConfig struct {
	// Type is the storage backend type (kubernetes-secret, file)
	Type string

	// Namespace is the Kubernetes namespace for secrets (if Type is kubernetes-secret)
	Namespace string

	// Path is the file system path (if Type is file)
	Path string
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		ServerURL:       "https://acme-staging-v02.api.letsencrypt.org/directory",
		KeyType:         "RSA2048",
		AutoRenew:       true,
		CheckInterval:   DefaultCheckInterval,
		RenewBefore:     DefaultRenewBefore,
		PropagationWait: DefaultPropagationWait,
		Storage: StorageConfig{
			Type:      "kubernetes-secret",
			Namespace: "default",
		},
	}
}

// LetsEncryptProduction returns the Let's Encrypt production server URL.
func LetsEncryptProduction() string {
	return "https://acme-v02.api.letsencrypt.org/directory"
}

// LetsEncryptStaging returns the Let's Encrypt staging server URL.
func LetsEncryptStaging() string {
	return "https://acme-staging-v02.api.letsencrypt.org/directory"
}

// ZeroSSLProduction returns the ZeroSSL production ACME server URL.
func ZeroSSLProduction() string {
	return "https://acme.zerossl.com/v2/DV90"
}
