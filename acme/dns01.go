package acme

import (
	"context"
	"crypto"
	"fmt"
	"log/slog"
	"time"

	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

	"github.com/go-acme/lego/v4/challenge/dns01"
	"github.com/go-acme/lego/v4/registration"
)

// DNS01Provider implements the lego DNS provider interface using our storage layer.
type DNS01Provider struct {
	storage         storage.CoreStorage
	propagationWait time.Duration
}

// NewDNS01Provider creates a new DNS-01 challenge provider.
func NewDNS01Provider(store storage.CoreStorage) *DNS01Provider {
	return &DNS01Provider{
		storage:         store,
		propagationWait: 60 * time.Second, // Default wait time for DNS propagation
	}
}

// Present creates the TXT record for ACME DNS-01 challenge.
func (p *DNS01Provider) Present(domain, token, keyAuth string) error {
	fqdn, value := dns01.GetRecord(domain, keyAuth)

	// Normalize FQDN: remove wildcard prefix if present
	// *.example.com should use _acme-challenge.example.com., not _acme-challenge.*.example.com.
	fqdn = normalizeFQDN(fqdn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Check if record already exists
	existing, err := p.storage.Get(ctx, fqdn, types.RecordTypeTXT)
	if err != nil && err != types.ErrRecordNotFound {
		return fmt.Errorf("failed to check existing DNS-01 challenge record: %w", err)
	}

	var values []string
	if len(existing) > 0 {
		// Record exists, append new value to existing values
		values = append(existing[0].Value, value)
		slog.Info("appending DNS-01 challenge value", "fqdn", fqdn, "existing_count", len(existing[0].Value), "new_count", len(values))
	} else {
		// New record
		values = []string{value}
		slog.Info("creating DNS-01 challenge record", "fqdn", fqdn)
	}

	record := &types.DNSRecord{
		Name:  fqdn,
		Type:  types.RecordTypeTXT,
		TTL:   60,
		Value: values,
	}

	// Try to create, if exists then update
	err = p.storage.Create(ctx, record)
	if err != nil && err == types.ErrRecordExists {
		err = p.storage.Update(ctx, record)
	}

	if err != nil {
		return fmt.Errorf("failed to create DNS-01 challenge record: %w", err)
	}

	// Wait for DNS propagation with exponential backoff logging
	p.waitForPropagation(domain)
	return nil
}

// waitForPropagation waits for DNS propagation with exponential backoff logging.
// Logs at intervals: 1s, 2s, 4s, 8s, 8s, 8s... until total wait time is reached.
func (p *DNS01Provider) waitForPropagation(domain string) {
	totalWait := p.propagationWait
	elapsed := time.Duration(0)
	interval := 1 * time.Second
	maxInterval := 8 * time.Second

	slog.Info("waiting for DNS record propagation", "domain", domain, "total_wait", totalWait)

	for elapsed < totalWait {
		time.Sleep(interval)
		elapsed += interval

		// Log progress with exponential backoff
		if elapsed < totalWait {
			slog.Info("DNS propagation in progress", "domain", domain, "elapsed", elapsed, "remaining", totalWait-elapsed)
		}

		// Exponential backoff: 1s -> 2s -> 4s -> 8s (max)
		interval *= 2
		if interval > maxInterval {
			interval = maxInterval
		}
	}

	slog.Info("DNS propagation wait complete", "domain", domain, "total_wait", totalWait)
}

// CleanUp removes the TXT record after ACME DNS-01 challenge completion.
func (p *DNS01Provider) CleanUp(domain, token, keyAuth string) error {
	fqdn, _ := dns01.GetRecord(domain, keyAuth)

	// Normalize FQDN: remove wildcard prefix if present
	fqdn = normalizeFQDN(fqdn)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err := p.storage.Delete(ctx, fqdn, types.RecordTypeTXT)
	if err != nil && err != types.ErrRecordNotFound {
		return fmt.Errorf("failed to cleanup DNS-01 challenge record: %w", err)
	}

	return nil
}

// Timeout returns the timeout and interval to use when checking for DNS propagation.
func (p *DNS01Provider) Timeout() (timeout, interval time.Duration) {
	return 2 * time.Minute, 2 * time.Second
}

// Sequential returns whether challenges should be run sequentially.
func (p *DNS01Provider) Sequential() bool {
	return false
}

// SetPropagationWait sets the wait time for DNS propagation.
func (p *DNS01Provider) SetPropagationWait(wait time.Duration) {
	p.propagationWait = wait
}

// User implements the lego User interface for ACME registration.
type User struct {
	Email        string
	Registration *registration.Resource
	key          crypto.PrivateKey
}

// GetEmail returns the user's email.
func (u *User) GetEmail() string {
	return u.Email
}

// GetRegistration returns the user's registration resource.
func (u *User) GetRegistration() *registration.Resource {
	return u.Registration
}

// GetPrivateKey returns the user's private key.
func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

// normalizeFQDN removes wildcard prefix from FQDN.
// _acme-challenge.*.example.com. → _acme-challenge.example.com.
// This is necessary because ACME DNS-01 validation for *.example.com
// should use _acme-challenge.example.com., not _acme-challenge.*.example.com.
func normalizeFQDN(fqdn string) string {
	// Replace "_acme-challenge.*." with "_acme-challenge."
	if len(fqdn) > 18 && fqdn[:18] == "_acme-challenge.*." {
		return "_acme-challenge." + fqdn[18:]
	}
	return fqdn
}
