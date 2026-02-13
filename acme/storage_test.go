package acme

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-acme/lego/v4/certificate"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestKubernetesSecretStorage_Store(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "default")

	ctx := context.Background()
	cert := &certificate.Resource{
		Domain:            "example.com",
		Certificate:       []byte("cert-data"),
		PrivateKey:        []byte("key-data"),
		IssuerCertificate: []byte("issuer-data"),
	}

	err := storage.Store(ctx, "example.com", cert)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Verify secret was created
	secret, err := client.CoreV1().Secrets("default").Get(ctx, "tls-example-com", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret error = %v", err)
	}

	if secret.Type != corev1.SecretTypeTLS {
		t.Errorf("secret type = %v, want %v", secret.Type, corev1.SecretTypeTLS)
	}

	if string(secret.Data["tls.crt"]) != "cert-data" {
		t.Errorf("tls.crt = %v, want cert-data", string(secret.Data["tls.crt"]))
	}

	if string(secret.Data["tls.key"]) != "key-data" {
		t.Errorf("tls.key = %v, want key-data", string(secret.Data["tls.key"]))
	}

	if string(secret.Data["ca.crt"]) != "issuer-data" {
		t.Errorf("ca.crt = %v, want issuer-data", string(secret.Data["ca.crt"]))
	}
}

func TestKubernetesSecretStorage_Load(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "default")

	ctx := context.Background()

	// Create a secret first
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-example-com",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
		Data: map[string][]byte{
			"tls.crt": []byte("cert-data"),
			"tls.key": []byte("key-data"),
			"ca.crt":  []byte("issuer-data"),
		},
	}
	_, err := client.CoreV1().Secrets("default").Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create secret error = %v", err)
	}

	// Load the certificate
	cert, err := storage.Load(ctx, "example.com")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cert.Domain != "example.com" {
		t.Errorf("Domain = %v, want example.com", cert.Domain)
	}

	if string(cert.Certificate) != "cert-data" {
		t.Errorf("Certificate = %v, want cert-data", string(cert.Certificate))
	}

	if string(cert.PrivateKey) != "key-data" {
		t.Errorf("PrivateKey = %v, want key-data", string(cert.PrivateKey))
	}

	if string(cert.IssuerCertificate) != "issuer-data" {
		t.Errorf("IssuerCertificate = %v, want issuer-data", string(cert.IssuerCertificate))
	}
}

func TestKubernetesSecretStorage_Delete(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "default")

	ctx := context.Background()

	// Create a secret first
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tls-example-com",
			Namespace: "default",
		},
		Type: corev1.SecretTypeTLS,
	}
	_, err := client.CoreV1().Secrets("default").Create(ctx, secret, metav1.CreateOptions{})
	if err != nil {
		t.Fatalf("Create secret error = %v", err)
	}

	// Delete the certificate
	err = storage.Delete(ctx, "example.com")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify secret was deleted
	_, err = client.CoreV1().Secrets("default").Get(ctx, "tls-example-com", metav1.GetOptions{})
	if err == nil {
		t.Error("expected error when getting deleted secret")
	}
}

func TestKubernetesSecretStorage_Update(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "default")

	ctx := context.Background()

	// Store initial certificate
	cert1 := &certificate.Resource{
		Domain:      "example.com",
		Certificate: []byte("cert-data-1"),
		PrivateKey:  []byte("key-data-1"),
	}
	if err := storage.Store(ctx, "example.com", cert1); err != nil {
		t.Fatalf("Store() first call error = %v", err)
	}

	// Update with new certificate
	cert2 := &certificate.Resource{
		Domain:      "example.com",
		Certificate: []byte("cert-data-2"),
		PrivateKey:  []byte("key-data-2"),
	}
	if err := storage.Store(ctx, "example.com", cert2); err != nil {
		t.Fatalf("Store() second call error = %v", err)
	}

	// Verify secret was updated
	secret, err := client.CoreV1().Secrets("default").Get(ctx, "tls-example-com", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret error = %v", err)
	}

	if string(secret.Data["tls.crt"]) != "cert-data-2" {
		t.Errorf("tls.crt = %v, want cert-data-2", string(secret.Data["tls.crt"]))
	}
}

func TestFileStorage_Store(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	ctx := context.Background()
	cert := &certificate.Resource{
		Domain:            "example.com",
		Certificate:       []byte("cert-data"),
		PrivateKey:        []byte("key-data"),
		IssuerCertificate: []byte("issuer-data"),
	}

	err := storage.Store(ctx, "example.com", cert)
	if err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Verify files were created
	domainDir := filepath.Join(tmpDir, "example-com")

	certData, err := os.ReadFile(filepath.Join(domainDir, "certificate.crt"))
	if err != nil {
		t.Fatalf("ReadFile certificate error = %v", err)
	}
	if string(certData) != "cert-data" {
		t.Errorf("certificate = %v, want cert-data", string(certData))
	}

	keyData, err := os.ReadFile(filepath.Join(domainDir, "private.key"))
	if err != nil {
		t.Fatalf("ReadFile key error = %v", err)
	}
	if string(keyData) != "key-data" {
		t.Errorf("key = %v, want key-data", string(keyData))
	}

	issuerData, err := os.ReadFile(filepath.Join(domainDir, "issuer.crt"))
	if err != nil {
		t.Fatalf("ReadFile issuer error = %v", err)
	}
	if string(issuerData) != "issuer-data" {
		t.Errorf("issuer = %v, want issuer-data", string(issuerData))
	}
}

func TestFileStorage_Load(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	ctx := context.Background()

	// Create files first
	domainDir := filepath.Join(tmpDir, "example-com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	os.WriteFile(filepath.Join(domainDir, "certificate.crt"), []byte("cert-data"), 0o644)
	os.WriteFile(filepath.Join(domainDir, "private.key"), []byte("key-data"), 0o600)
	os.WriteFile(filepath.Join(domainDir, "issuer.crt"), []byte("issuer-data"), 0o644)

	// Load the certificate
	cert, err := storage.Load(ctx, "example.com")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cert.Domain != "example.com" {
		t.Errorf("Domain = %v, want example.com", cert.Domain)
	}

	if string(cert.Certificate) != "cert-data" {
		t.Errorf("Certificate = %v, want cert-data", string(cert.Certificate))
	}

	if string(cert.PrivateKey) != "key-data" {
		t.Errorf("PrivateKey = %v, want key-data", string(cert.PrivateKey))
	}

	if string(cert.IssuerCertificate) != "issuer-data" {
		t.Errorf("IssuerCertificate = %v, want issuer-data", string(cert.IssuerCertificate))
	}
}

func TestFileStorage_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	ctx := context.Background()

	// Create files first
	cert := &certificate.Resource{
		Domain:      "example.com",
		Certificate: []byte("cert-data"),
		PrivateKey:  []byte("key-data"),
	}
	if err := storage.Store(ctx, "example.com", cert); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Delete the certificate
	err := storage.Delete(ctx, "example.com")
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	// Verify directory was deleted
	domainDir := filepath.Join(tmpDir, "example-com")
	if _, err := os.Stat(domainDir); !os.IsNotExist(err) {
		t.Error("expected directory to be deleted")
	}
}

func TestSanitizeDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   string
	}{
		{
			name:   "simple domain",
			domain: "example.com",
			want:   "example-com",
		},
		{
			name:   "subdomain",
			domain: "www.example.com",
			want:   "example-com",
		},
		{
			name:   "wildcard domain",
			domain: "*.example.com",
			want:   "example-com",
		},
		{
			name:   "wildcard mesh-worker.cloud",
			domain: "*.mesh-worker.cloud",
			want:   "mesh-worker-cloud",
		},
		{
			name:   "apex mesh-worker.cloud",
			domain: "mesh-worker.cloud",
			want:   "mesh-worker-cloud",
		},
		{
			name:   "api subdomain",
			domain: "api.mesh-worker.cloud",
			want:   "mesh-worker-cloud",
		},
		{
			name:   "multi-level subdomain",
			domain: "v1.api.mesh-worker.cloud",
			want:   "mesh-worker-cloud",
		},
		{
			name:   "deep subdomain",
			domain: "service.v1.api.example.com",
			want:   "example-com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeDomain(tt.domain)
			if got != tt.want {
				t.Errorf("sanitizeDomain(%q) = %q, want %q", tt.domain, got, tt.want)
			}

			// Verify secret name format
			secretName := fmt.Sprintf("tls--%s", got)
			t.Logf("Domain: %s → Secret: %s", tt.domain, secretName)
		})
	}
}

func TestFileStorage_LoadWithoutIssuer(t *testing.T) {
	tmpDir := t.TempDir()
	storage := NewFileStorage(tmpDir)

	ctx := context.Background()

	// Create files without issuer
	domainDir := filepath.Join(tmpDir, "example-com")
	if err := os.MkdirAll(domainDir, 0o755); err != nil {
		t.Fatalf("MkdirAll error = %v", err)
	}

	os.WriteFile(filepath.Join(domainDir, "certificate.crt"), []byte("cert-data"), 0o644)
	os.WriteFile(filepath.Join(domainDir, "private.key"), []byte("key-data"), 0o600)

	// Load should succeed even without issuer
	cert, err := storage.Load(ctx, "example.com")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cert.IssuerCertificate) != 0 {
		t.Errorf("IssuerCertificate should be empty, got %v", cert.IssuerCertificate)
	}
}
