// Package storage provides the CoreStorage interface and implementations
// for DNS record persistence with hot reload support.
package storage

import (
	"context"

	"jabberwocky238/jw238dns/types"
)

// CoreStorage defines the interface for DNS record storage with CRUD
// operations, hot reload support, and change notifications.
type CoreStorage interface {
	// Get returns all records matching the given name and type.
	Get(ctx context.Context, name string, recordType types.RecordType) ([]*types.DNSRecord, error)

	// List returns all stored DNS records.
	List(ctx context.Context) ([]*types.DNSRecord, error)

	// Create adds a new DNS record to storage.
	Create(ctx context.Context, record *types.DNSRecord) error

	// Update replaces an existing DNS record in storage.
	Update(ctx context.Context, record *types.DNSRecord) error

	// Delete removes a DNS record identified by name and type.
	Delete(ctx context.Context, name string, recordType types.RecordType) error

	// HotReload replaces all records atomically with the provided set.
	HotReload(ctx context.Context, records []*types.DNSRecord) error

	// PartialReload applies only the changed records atomically.
	PartialReload(ctx context.Context, changes *types.RecordChanges) error

	// Watch returns a channel that receives storage change events.
	Watch(ctx context.Context) (<-chan types.StorageEvent, error)
}
