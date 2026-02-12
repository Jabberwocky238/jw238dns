package acme

import "time"

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
		CheckInterval:   24 * time.Hour,
		RenewBefore:     30 * 24 * time.Hour, // 30 days
		PropagationWait: 60 * time.Second,
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
