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
