package acme

import (
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	if config.ServerURL != LetsEncryptStaging() {
		t.Errorf("ServerURL = %v, want %v", config.ServerURL, LetsEncryptStaging())
	}

	if config.KeyType != "RSA2048" {
		t.Errorf("KeyType = %v, want RSA2048", config.KeyType)
	}

	if !config.AutoRenew {
		t.Error("AutoRenew = false, want true")
	}

	if config.CheckInterval != 24*time.Hour {
		t.Errorf("CheckInterval = %v, want 24h", config.CheckInterval)
	}

	if config.RenewBefore != 30*24*time.Hour {
		t.Errorf("RenewBefore = %v, want 720h", config.RenewBefore)
	}

	if config.PropagationWait != 60*time.Second {
		t.Errorf("PropagationWait = %v, want 60s", config.PropagationWait)
	}

	if config.Storage.Type != "kubernetes-secret" {
		t.Errorf("Storage.Type = %v, want kubernetes-secret", config.Storage.Type)
	}

	if config.Storage.Namespace != "default" {
		t.Errorf("Storage.Namespace = %v, want default", config.Storage.Namespace)
	}
}

func TestLetsEncryptProduction(t *testing.T) {
	url := LetsEncryptProduction()
	expected := "https://acme-v02.api.letsencrypt.org/directory"

	if url != expected {
		t.Errorf("LetsEncryptProduction() = %v, want %v", url, expected)
	}
}

func TestLetsEncryptStaging(t *testing.T) {
	url := LetsEncryptStaging()
	expected := "https://acme-staging-v02.api.letsencrypt.org/directory"

	if url != expected {
		t.Errorf("LetsEncryptStaging() = %v, want %v", url, expected)
	}
}

func TestConfig_CustomValues(t *testing.T) {
	config := &Config{
		ServerURL:       LetsEncryptProduction(),
		Email:           "admin@example.com",
		KeyType:         "EC256",
		AutoRenew:       false,
		CheckInterval:   12 * time.Hour,
		RenewBefore:     15 * 24 * time.Hour,
		PropagationWait: 120 * time.Second,
		Storage: StorageConfig{
			Type:      "file",
			Namespace: "custom-ns",
			Path:      "/var/certs",
		},
	}

	if config.ServerURL != LetsEncryptProduction() {
		t.Errorf("ServerURL = %v", config.ServerURL)
	}

	if config.Email != "admin@example.com" {
		t.Errorf("Email = %v", config.Email)
	}

	if config.KeyType != "EC256" {
		t.Errorf("KeyType = %v", config.KeyType)
	}

	if config.AutoRenew {
		t.Error("AutoRenew should be false")
	}

	if config.Storage.Type != "file" {
		t.Errorf("Storage.Type = %v", config.Storage.Type)
	}

	if config.Storage.Path != "/var/certs" {
		t.Errorf("Storage.Path = %v", config.Storage.Path)
	}
}

func TestZeroSSLProduction(t *testing.T) {
	url := ZeroSSLProduction()
	expected := "https://acme.zerossl.com/v2/DV90"

	if url != expected {
		t.Errorf("ZeroSSLProduction() = %v, want %v", url, expected)
	}
}

func TestConfig_WithEAB(t *testing.T) {
	config := &Config{
		ServerURL: ZeroSSLProduction(),
		Email:     "admin@example.com",
		KeyType:   "RSA2048",
		AutoRenew: true,
		EAB: EABConfig{
			KID:     "test-kid-123",
			HMACKey: "test-hmac-key-456",
		},
		Storage: StorageConfig{
			Type:      "kubernetes-secret",
			Namespace: "jw238dns",
		},
	}

	if config.EAB.KID != "test-kid-123" {
		t.Errorf("EAB.KID = %v, want test-kid-123", config.EAB.KID)
	}

	if config.EAB.HMACKey != "test-hmac-key-456" {
		t.Errorf("EAB.HMACKey = %v, want test-hmac-key-456", config.EAB.HMACKey)
	}

	if config.ServerURL != ZeroSSLProduction() {
		t.Errorf("ServerURL = %v, want %v", config.ServerURL, ZeroSSLProduction())
	}
}

func TestConfig_WithoutEAB(t *testing.T) {
	config := &Config{
		ServerURL: LetsEncryptProduction(),
		Email:     "admin@example.com",
		KeyType:   "RSA2048",
	}

	if config.EAB.KID != "" {
		t.Errorf("EAB.KID should be empty, got %v", config.EAB.KID)
	}

	if config.EAB.HMACKey != "" {
		t.Errorf("EAB.HMACKey should be empty, got %v", config.EAB.HMACKey)
	}
}

func TestDefaultConfig_EABEmpty(t *testing.T) {
	config := DefaultConfig()

	if config.EAB.KID != "" {
		t.Errorf("DefaultConfig EAB.KID should be empty, got %v", config.EAB.KID)
	}

	if config.EAB.HMACKey != "" {
		t.Errorf("DefaultConfig EAB.HMACKey should be empty, got %v", config.EAB.HMACKey)
	}
}
