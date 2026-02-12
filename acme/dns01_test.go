package acme

import (
	"context"
	"testing"
	"time"

	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"
)

func TestDNS01Provider_Present(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)
	provider.SetPropagationWait(10 * time.Millisecond) // Short wait for testing

	domain := "example.com"
	token := "test-token"
	keyAuth := "test-key-auth"

	err := provider.Present(domain, token, keyAuth)
	if err != nil {
		t.Fatalf("Present() error = %v", err)
	}

	// Verify TXT record was created
	ctx := context.Background()
	records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(records) == 0 {
		t.Fatal("expected TXT record to be created")
	}

	if records[0].Type != types.RecordTypeTXT {
		t.Errorf("record type = %v, want TXT", records[0].Type)
	}

	if records[0].TTL != 60 {
		t.Errorf("TTL = %d, want 60", records[0].TTL)
	}
}

func TestDNS01Provider_CleanUp(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)
	provider.SetPropagationWait(10 * time.Millisecond)

	domain := "example.com"
	token := "test-token"
	keyAuth := "test-key-auth"

	// First create the record
	if err := provider.Present(domain, token, keyAuth); err != nil {
		t.Fatalf("Present() error = %v", err)
	}

	// Then clean it up
	if err := provider.CleanUp(domain, token, keyAuth); err != nil {
		t.Fatalf("CleanUp() error = %v", err)
	}

	// Verify record was deleted
	ctx := context.Background()
	records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
	if err != types.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound, got %v with %d records", err, len(records))
	}
}

func TestDNS01Provider_Timeout(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)

	timeout, interval := provider.Timeout()

	if timeout != 2*time.Minute {
		t.Errorf("timeout = %v, want 2m", timeout)
	}

	if interval != 2*time.Second {
		t.Errorf("interval = %v, want 2s", interval)
	}
}

func TestDNS01Provider_Sequential(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)

	if provider.Sequential() {
		t.Error("Sequential() = true, want false")
	}
}

func TestDNS01Provider_SetPropagationWait(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)

	customWait := 30 * time.Second
	provider.SetPropagationWait(customWait)

	if provider.propagationWait != customWait {
		t.Errorf("propagationWait = %v, want %v", provider.propagationWait, customWait)
	}
}

func TestUser_GetEmail(t *testing.T) {
	user := &User{Email: "test@example.com"}

	if user.GetEmail() != "test@example.com" {
		t.Errorf("GetEmail() = %v, want test@example.com", user.GetEmail())
	}
}

func TestUser_GetPrivateKey(t *testing.T) {
	user := &User{}

	if user.GetPrivateKey() != nil {
		t.Error("GetPrivateKey() should return nil for uninitialized key")
	}
}

func TestDNS01Provider_Present_UpdateExisting(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)
	provider.SetPropagationWait(10 * time.Millisecond)

	domain := "example.com"
	token := "test-token"
	keyAuth1 := "test-key-auth-1"
	keyAuth2 := "test-key-auth-2"

	// Create first record
	if err := provider.Present(domain, token, keyAuth1); err != nil {
		t.Fatalf("Present() first call error = %v", err)
	}

	// Update with second record (should update, not fail)
	if err := provider.Present(domain, token, keyAuth2); err != nil {
		t.Fatalf("Present() second call error = %v", err)
	}

	// Verify record was updated
	ctx := context.Background()
	records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(records) != 1 {
		t.Errorf("expected 1 record, got %d", len(records))
	}
}

func TestDNS01Provider_CleanUp_NonExistent(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)

	domain := "nonexistent.com"
	token := "test-token"
	keyAuth := "test-key-auth"

	// Cleanup should not error even if record doesn't exist
	if err := provider.CleanUp(domain, token, keyAuth); err != nil {
		t.Errorf("CleanUp() on non-existent record error = %v, want nil", err)
	}
}
