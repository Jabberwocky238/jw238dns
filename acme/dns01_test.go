package acme

import (
	"context"
	"testing"
	"time"

	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

	"github.com/go-acme/lego/v4/challenge/dns01"
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

// TestDNS01Provider_MultiDomain_SameFQDN tests the scenario where multiple domains
// (e.g., *.example.com and example.com) share the same ACME challenge FQDN.
// This is a common case when obtaining a wildcard certificate with the apex domain.
func TestDNS01Provider_MultiDomain_SameFQDN(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)
	provider.SetPropagationWait(10 * time.Millisecond)

	// Simulate two domains that share the same challenge FQDN
	domain1 := "*.example.com"
	domain2 := "example.com"
	token := "test-token"
	keyAuth1 := "key-auth-for-wildcard"
	keyAuth2 := "key-auth-for-apex"

	// Get expected values from dns01 library
	_, expectedValue1 := dns01.GetRecord(domain1, keyAuth1)
	_, expectedValue2 := dns01.GetRecord(domain2, keyAuth2)

	// Present first domain (wildcard)
	if err := provider.Present(domain1, token, keyAuth1); err != nil {
		t.Fatalf("Present() for wildcard domain error = %v", err)
	}

	// Present second domain (apex) - should append, not replace
	if err := provider.Present(domain2, token, keyAuth2); err != nil {
		t.Fatalf("Present() for apex domain error = %v", err)
	}

	// Verify TXT record contains both values
	ctx := context.Background()
	records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if len(records) != 1 {
		t.Fatalf("expected 1 TXT record, got %d", len(records))
	}

	// The record should have 2 values (one for each domain)
	if len(records[0].Value) != 2 {
		t.Errorf("expected 2 TXT values, got %d: %v", len(records[0].Value), records[0].Value)
	}

	// Verify both values are present (order doesn't matter)
	values := records[0].Value
	hasValue1 := false
	hasValue2 := false
	for _, v := range values {
		if v == expectedValue1 {
			hasValue1 = true
		}
		if v == expectedValue2 {
			hasValue2 = true
		}
	}

	if !hasValue1 {
		t.Errorf("TXT record missing value for wildcard domain (expected: %s, got: %v)", expectedValue1, values)
	}
	if !hasValue2 {
		t.Errorf("TXT record missing value for apex domain (expected: %s, got: %v)", expectedValue2, values)
	}
}

// TestDNS01Provider_MultiDomain_Sequential tests presenting multiple domains sequentially
// and verifies that values are appended correctly.
func TestDNS01Provider_MultiDomain_Sequential(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)
	provider.SetPropagationWait(10 * time.Millisecond)

	domain := "example.com"
	token := "test-token"

	// Present three different challenges for the same domain
	keyAuths := []string{"auth1", "auth2", "auth3"}
	expectedValues := make([]string, len(keyAuths))

	// Get expected values from dns01 library
	for i, keyAuth := range keyAuths {
		_, expectedValues[i] = dns01.GetRecord(domain, keyAuth)
	}

	for i, keyAuth := range keyAuths {
		if err := provider.Present(domain, token, keyAuth); err != nil {
			t.Fatalf("Present() call %d error = %v", i+1, err)
		}

		// Verify the number of values increases
		ctx := context.Background()
		records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
		if err != nil {
			t.Fatalf("Get() after call %d error = %v", i+1, err)
		}

		expectedCount := i + 1
		if len(records[0].Value) != expectedCount {
			t.Errorf("after call %d: expected %d values, got %d", i+1, expectedCount, len(records[0].Value))
		}
	}

	// Verify all values are present
	ctx := context.Background()
	records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
	if err != nil {
		t.Fatalf("Get() final check error = %v", err)
	}

	if len(records[0].Value) != 3 {
		t.Errorf("expected 3 final values, got %d", len(records[0].Value))
	}

	for i, expectedValue := range expectedValues {
		found := false
		for _, actualValue := range records[0].Value {
			if actualValue == expectedValue {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("value %d (%s) not found in TXT record, got: %v", i+1, expectedValue, records[0].Value)
		}
	}
}

// TestDNS01Provider_CleanUp_MultiValue tests cleanup when TXT record has multiple values.
// After cleanup, the record should be completely removed.
func TestDNS01Provider_CleanUp_MultiValue(t *testing.T) {
	store := storage.NewMemoryStorage()
	provider := NewDNS01Provider(store)
	provider.SetPropagationWait(10 * time.Millisecond)

	domain := "example.com"
	token := "test-token"
	keyAuth1 := "auth1"
	keyAuth2 := "auth2"

	// Create record with two values
	if err := provider.Present(domain, token, keyAuth1); err != nil {
		t.Fatalf("Present() first call error = %v", err)
	}
	if err := provider.Present(domain, token, keyAuth2); err != nil {
		t.Fatalf("Present() second call error = %v", err)
	}

	// Cleanup should remove the entire record
	if err := provider.CleanUp(domain, token, keyAuth1); err != nil {
		t.Fatalf("CleanUp() error = %v", err)
	}

	// Verify record was deleted
	ctx := context.Background()
	records, err := store.Get(ctx, "_acme-challenge.example.com.", types.RecordTypeTXT)
	if err != types.ErrRecordNotFound {
		t.Errorf("expected ErrRecordNotFound after cleanup, got %v with %d records", err, len(records))
	}
}
