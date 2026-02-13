package acme

import (
	"context"
	"testing"

	"github.com/go-acme/lego/v4/certificate"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestStorageIntegration_NormalDomain tests the complete flow for normal domain
func TestStorageIntegration_NormalDomain(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "jw238dns")
	ctx := context.Background()

	// Test domain: mesh-worker.cloud
	domain := "mesh-worker.cloud"

	// Parse domain to get mapping
	mapping, err := ParseDomain(domain)
	if err != nil {
		t.Fatalf("ParseDomain() error = %v", err)
	}

	// Verify mapping is correct
	expectedSecretName := "tls-normal--mesh-worker_cloud"
	if mapping.SecretName != expectedSecretName {
		t.Errorf("SecretName = %q, want %q", mapping.SecretName, expectedSecretName)
	}

	if mapping.DomainType != DomainTypeNormal {
		t.Errorf("DomainType = %v, want %v", mapping.DomainType, DomainTypeNormal)
	}

	if mapping.NormalizedDomain != "mesh-worker_cloud" {
		t.Errorf("NormalizedDomain = %q, want %q", mapping.NormalizedDomain, "mesh-worker_cloud")
	}

	// Create certificate
	cert := &certificate.Resource{
		Domain:            domain,
		Certificate:       []byte("cert-data-for-apex"),
		PrivateKey:        []byte("key-data-for-apex"),
		IssuerCertificate: []byte("issuer-data"),
	}

	// Store certificate
	if err := storage.Store(ctx, mapping, cert); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Verify secret was created with correct name
	secret, err := client.CoreV1().Secrets("jw238dns").Get(ctx, expectedSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret error = %v", err)
	}

	if secret.Name != expectedSecretName {
		t.Errorf("Secret name = %q, want %q", secret.Name, expectedSecretName)
	}

	if string(secret.Data["tls.crt"]) != "cert-data-for-apex" {
		t.Errorf("tls.crt = %v, want cert-data-for-apex", string(secret.Data["tls.crt"]))
	}

	// Verify labels
	if secret.Labels["domain"] != "mesh-worker_cloud" {
		t.Errorf("domain label = %q, want %q", secret.Labels["domain"], "mesh-worker_cloud")
	}

	t.Logf("✓ Normal domain %q → Secret %q", domain, expectedSecretName)
}

// TestStorageIntegration_WildcardDomain tests the complete flow for wildcard domain
func TestStorageIntegration_WildcardDomain(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "jw238dns")
	ctx := context.Background()

	// Test domain: *.mesh-worker.cloud
	domain := "*.mesh-worker.cloud"

	// Parse domain to get mapping
	mapping, err := ParseDomain(domain)
	if err != nil {
		t.Fatalf("ParseDomain() error = %v", err)
	}

	// Verify mapping is correct
	expectedSecretName := "tls-wildcard--__mesh-worker_cloud"
	if mapping.SecretName != expectedSecretName {
		t.Errorf("SecretName = %q, want %q", mapping.SecretName, expectedSecretName)
	}

	if mapping.DomainType != DomainTypeWildcard {
		t.Errorf("DomainType = %v, want %v", mapping.DomainType, DomainTypeWildcard)
	}

	if mapping.NormalizedDomain != "__mesh-worker_cloud" {
		t.Errorf("NormalizedDomain = %q, want %q", mapping.NormalizedDomain, "__mesh-worker_cloud")
	}

	// Create certificate
	cert := &certificate.Resource{
		Domain:            domain,
		Certificate:       []byte("cert-data-for-wildcard"),
		PrivateKey:        []byte("key-data-for-wildcard"),
		IssuerCertificate: []byte("issuer-data"),
	}

	// Store certificate
	if err := storage.Store(ctx, mapping, cert); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	// Verify secret was created with correct name
	secret, err := client.CoreV1().Secrets("jw238dns").Get(ctx, expectedSecretName, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get secret error = %v", err)
	}

	if secret.Name != expectedSecretName {
		t.Errorf("Secret name = %q, want %q", secret.Name, expectedSecretName)
	}

	if string(secret.Data["tls.crt"]) != "cert-data-for-wildcard" {
		t.Errorf("tls.crt = %v, want cert-data-for-wildcard", string(secret.Data["tls.crt"]))
	}

	// Verify labels
	if secret.Labels["domain"] != "__mesh-worker_cloud" {
		t.Errorf("domain label = %q, want %q", secret.Labels["domain"], "__mesh-worker_cloud")
	}

	t.Logf("✓ Wildcard domain %q → Secret %q", domain, expectedSecretName)
}

// TestStorageIntegration_BothDomains tests that both domains create separate secrets
func TestStorageIntegration_BothDomains(t *testing.T) {
	client := fake.NewSimpleClientset()
	storage := NewKubernetesSecretStorage(client, "jw238dns")
	ctx := context.Background()

	// Store normal domain certificate
	normalDomain := "mesh-worker.cloud"
	normalMapping, _ := ParseDomain(normalDomain)
	normalCert := &certificate.Resource{
		Domain:      normalDomain,
		Certificate: []byte("cert-for-apex"),
		PrivateKey:  []byte("key-for-apex"),
	}
	if err := storage.Store(ctx, normalMapping, normalCert); err != nil {
		t.Fatalf("Store normal domain error = %v", err)
	}

	// Store wildcard domain certificate
	wildcardDomain := "*.mesh-worker.cloud"
	wildcardMapping, _ := ParseDomain(wildcardDomain)
	wildcardCert := &certificate.Resource{
		Domain:      wildcardDomain,
		Certificate: []byte("cert-for-wildcard"),
		PrivateKey:  []byte("key-for-wildcard"),
	}
	if err := storage.Store(ctx, wildcardMapping, wildcardCert); err != nil {
		t.Fatalf("Store wildcard domain error = %v", err)
	}

	// Verify both secrets exist with different names
	normalSecret, err := client.CoreV1().Secrets("jw238dns").Get(ctx, "tls-normal--mesh-worker_cloud", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get normal secret error = %v", err)
	}

	wildcardSecret, err := client.CoreV1().Secrets("jw238dns").Get(ctx, "tls-wildcard--__mesh-worker_cloud", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Get wildcard secret error = %v", err)
	}

	// Verify they have different content
	if string(normalSecret.Data["tls.crt"]) == string(wildcardSecret.Data["tls.crt"]) {
		t.Error("Normal and wildcard secrets have same certificate data - they should be different!")
	}

	// Verify correct content
	if string(normalSecret.Data["tls.crt"]) != "cert-for-apex" {
		t.Errorf("Normal secret has wrong cert: %v", string(normalSecret.Data["tls.crt"]))
	}

	if string(wildcardSecret.Data["tls.crt"]) != "cert-for-wildcard" {
		t.Errorf("Wildcard secret has wrong cert: %v", string(wildcardSecret.Data["tls.crt"]))
	}

	t.Logf("✓ Both domains stored in separate secrets:")
	t.Logf("  - %q → %q", normalDomain, normalSecret.Name)
	t.Logf("  - %q → %q", wildcardDomain, wildcardSecret.Name)
}

// TestStorageIntegration_ManagerFlow tests the complete Manager flow
func TestStorageIntegration_ManagerFlow(t *testing.T) {
	// This test simulates what happens in Manager.ObtainCertificate()

	domains := []string{"mesh-worker.cloud"}

	// Step 1: Parse domain (this is what Manager does)
	mapping, err := ParseDomain(domains[0])
	if err != nil {
		t.Fatalf("ParseDomain() error = %v", err)
	}

	// Step 2: Verify mapping
	expectedSecretName := "tls-normal--mesh-worker_cloud"
	if mapping.SecretName != expectedSecretName {
		t.Errorf("Manager would store to wrong secret: %q, want %q", mapping.SecretName, expectedSecretName)
	}

	t.Logf("✓ Manager flow: domains=%v → Secret=%q", domains, mapping.SecretName)

	// Test wildcard domain
	wildcardDomains := []string{"*.mesh-worker.cloud"}
	wildcardMapping, err := ParseDomain(wildcardDomains[0])
	if err != nil {
		t.Fatalf("ParseDomain() error = %v", err)
	}

	expectedWildcardSecret := "tls-wildcard--__mesh-worker_cloud"
	if wildcardMapping.SecretName != expectedWildcardSecret {
		t.Errorf("Manager would store to wrong secret: %q, want %q", wildcardMapping.SecretName, expectedWildcardSecret)
	}

	t.Logf("✓ Manager flow: domains=%v → Secret=%q", wildcardDomains, wildcardMapping.SecretName)
}
