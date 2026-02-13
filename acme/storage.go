package acme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/go-acme/lego/v4/certificate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// CertificateStorage defines the interface for storing certificates.
type CertificateStorage interface {
	Store(ctx context.Context, mapping *DomainSecretMapping, cert *certificate.Resource) error
	Load(ctx context.Context, mapping *DomainSecretMapping) (*certificate.Resource, error)
	Delete(ctx context.Context, mapping *DomainSecretMapping) error
}

// KubernetesSecretStorage stores certificates in Kubernetes Secrets.
type KubernetesSecretStorage struct {
	client    kubernetes.Interface
	namespace string
}

// NewKubernetesSecretStorage creates a new Kubernetes Secret storage.
func NewKubernetesSecretStorage(client kubernetes.Interface, namespace string) *KubernetesSecretStorage {
	return &KubernetesSecretStorage{
		client:    client,
		namespace: namespace,
	}
}

// Store saves a certificate to a Kubernetes Secret.
func (s *KubernetesSecretStorage) Store(ctx context.Context, mapping *DomainSecretMapping, cert *certificate.Resource) error {

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      mapping.SecretName,
			Namespace: s.namespace,
			Labels: map[string]string{
				"app":    "jw238dns",
				"domain": mapping.NormalizedDomain,
			},
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": cert.Certificate,
			"tls.key": cert.PrivateKey,
			"ca.crt":  cert.IssuerCertificate,
		},
	}

	// Try to get existing secret first
	existing, err := s.client.CoreV1().Secrets(s.namespace).Get(ctx, secret.Name, metav1.GetOptions{})
	if err != nil {
		// Secret doesn't exist, create it
		_, err = s.client.CoreV1().Secrets(s.namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return fmt.Errorf("failed to create secret: %w", err)
		}
	} else {
		// Secret exists, update it
		// Preserve ResourceVersion for optimistic concurrency control
		secret.ResourceVersion = existing.ResourceVersion
		_, err = s.client.CoreV1().Secrets(s.namespace).Update(ctx, secret, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to update secret: %w", err)
		}
	}

	return nil
}

// Load retrieves a certificate from a Kubernetes Secret.
func (s *KubernetesSecretStorage) Load(ctx context.Context, mapping *DomainSecretMapping) (*certificate.Resource, error) {
	secret, err := s.client.CoreV1().Secrets(s.namespace).Get(ctx, mapping.SecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret: %w", err)
	}

	return &certificate.Resource{
		Domain:            mapping.OriginalDomain,
		Certificate:       secret.Data["tls.crt"],
		PrivateKey:        secret.Data["tls.key"],
		IssuerCertificate: secret.Data["ca.crt"],
	}, nil
}

// Delete removes a certificate from Kubernetes Secret.
func (s *KubernetesSecretStorage) Delete(ctx context.Context, mapping *DomainSecretMapping) error {
	err := s.client.CoreV1().Secrets(s.namespace).Delete(ctx, mapping.SecretName, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete secret: %w", err)
	}

	return nil
}

// FileStorage stores certificates in the file system.
type FileStorage struct {
	basePath string
}

// NewFileStorage creates a new file-based certificate storage.
func NewFileStorage(basePath string) *FileStorage {
	return &FileStorage{
		basePath: basePath,
	}
}

// Store saves a certificate to the file system.
func (s *FileStorage) Store(ctx context.Context, mapping *DomainSecretMapping, cert *certificate.Resource) error {
	domainDir := filepath.Join(s.basePath, mapping.NormalizedDomain)

	// Create directory if it doesn't exist
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Write certificate
	certPath := filepath.Join(domainDir, "certificate.crt")
	if err := os.WriteFile(certPath, cert.Certificate, 0o644); err != nil {
		return fmt.Errorf("failed to write certificate: %w", err)
	}

	// Write private key
	keyPath := filepath.Join(domainDir, "private.key")
	if err := os.WriteFile(keyPath, cert.PrivateKey, 0o600); err != nil {
		return fmt.Errorf("failed to write private key: %w", err)
	}

	// Write issuer certificate
	if len(cert.IssuerCertificate) > 0 {
		issuerPath := filepath.Join(domainDir, "issuer.crt")
		if err := os.WriteFile(issuerPath, cert.IssuerCertificate, 0o644); err != nil {
			return fmt.Errorf("failed to write issuer certificate: %w", err)
		}
	}

	return nil
}

// Load retrieves a certificate from the file system.
func (s *FileStorage) Load(ctx context.Context, mapping *DomainSecretMapping) (*certificate.Resource, error) {
	domainDir := filepath.Join(s.basePath, mapping.NormalizedDomain)

	certPath := filepath.Join(domainDir, "certificate.crt")
	certData, err := os.ReadFile(certPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read certificate: %w", err)
	}

	keyPath := filepath.Join(domainDir, "private.key")
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	issuerPath := filepath.Join(domainDir, "issuer.crt")
	issuerData, _ := os.ReadFile(issuerPath) // Optional

	return &certificate.Resource{
		Domain:            mapping.OriginalDomain,
		Certificate:       certData,
		PrivateKey:        keyData,
		IssuerCertificate: issuerData,
	}, nil
}

// Delete removes a certificate from the file system.
func (s *FileStorage) Delete(ctx context.Context, mapping *DomainSecretMapping) error {
	domainDir := filepath.Join(s.basePath, mapping.NormalizedDomain)

	if err := os.RemoveAll(domainDir); err != nil {
		return fmt.Errorf("failed to delete certificate directory: %w", err)
	}

	return nil
}
