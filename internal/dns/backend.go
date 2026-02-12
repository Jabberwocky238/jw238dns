package dns

import (
	"context"
	"log/slog"

	"jabberwocky238/jw238dns/internal/storage"
	"jabberwocky238/jw238dns/internal/types"

	"github.com/miekg/dns"
)

// DNSBackend resolves DNS queries from storage and applies rules.
type DNSBackend interface {
	// Resolve returns DNS records matching the query.
	Resolve(ctx context.Context, query *types.QueryInfo) ([]*types.DNSRecord, error)

	// ApplyRules applies transformation rules to a set of records.
	ApplyRules(ctx context.Context, records []*types.DNSRecord) ([]*types.DNSRecord, error)
}

// BackendConfig holds configurable behaviour for the Backend.
type BackendConfig struct {
	DefaultTTL         uint32 // Default TTL when record has none
	ResolveCNAMEChain  bool   // Follow CNAME chains
	MaxCNAMEDepth      int    // Maximum CNAME chain depth
	ReturnSOAOnNXDOMAIN bool  // Attach SOA to NXDOMAIN responses
}

// DefaultBackendConfig returns a BackendConfig with sensible defaults.
func DefaultBackendConfig() BackendConfig {
	return BackendConfig{
		DefaultTTL:         300,
		ResolveCNAMEChain:  true,
		MaxCNAMEDepth:      10,
		ReturnSOAOnNXDOMAIN: true,
	}
}

// Backend implements DNSBackend using a CoreStorage for lookups.
type Backend struct {
	storage storage.CoreStorage
	config  BackendConfig
}

// NewBackend creates a Backend backed by the given storage and config.
func NewBackend(store storage.CoreStorage, cfg BackendConfig) *Backend {
	return &Backend{
		storage: store,
		config:  cfg,
	}
}

// Resolve looks up records from storage. For non-SOA queries it will follow
// CNAME chains when configured. If no records are found it returns
// ErrRecordNotFound.
func (b *Backend) Resolve(ctx context.Context, query *types.QueryInfo) ([]*types.DNSRecord, error) {
	rt := uint16ToRecordType(query.Type)

	// ANY query: return all record types for the domain.
	if query.Type == dns.TypeANY {
		return b.resolveAny(ctx, query.Domain)
	}

	// Direct lookup.
	recs, err := b.storage.Get(ctx, query.Domain, rt)
	if err == nil {
		recs, _ = b.ApplyRules(ctx, recs)
		return recs, nil
	}

	// If the requested type is not CNAME, check whether a CNAME exists and
	// follow the chain.
	if b.config.ResolveCNAMEChain && rt != types.RecordTypeCNAME {
		chainRecs, chainErr := b.resolveCNAMEChain(ctx, query.Domain, rt, 0)
		if chainErr == nil && len(chainRecs) > 0 {
			chainRecs, _ = b.ApplyRules(ctx, chainRecs)
			return chainRecs, nil
		}
	}

	return nil, types.ErrRecordNotFound
}

// ApplyRules applies the configured default TTL to any record that has a
// zero TTL. Additional rule logic can be added here in the future.
func (b *Backend) ApplyRules(_ context.Context, records []*types.DNSRecord) ([]*types.DNSRecord, error) {
	for _, r := range records {
		if r.TTL == 0 {
			r.TTL = b.config.DefaultTTL
		}
	}
	return records, nil
}

// resolveCNAMEChain follows CNAME records up to MaxCNAMEDepth, collecting
// the CNAME records and the final target records of the requested type.
func (b *Backend) resolveCNAMEChain(ctx context.Context, domain string, targetType types.RecordType, depth int) ([]*types.DNSRecord, error) {
	if depth >= b.config.MaxCNAMEDepth {
		slog.Warn("CNAME chain depth exceeded", "domain", domain, "depth", depth)
		return nil, types.ErrRecordNotFound
	}

	cnameRecs, err := b.storage.Get(ctx, domain, types.RecordTypeCNAME)
	if err != nil {
		return nil, types.ErrRecordNotFound
	}

	if len(cnameRecs) == 0 || len(cnameRecs[0].Value) == 0 {
		return nil, types.ErrRecordNotFound
	}

	var result []*types.DNSRecord
	result = append(result, cnameRecs...)

	target := cnameRecs[0].Value[0]

	// Try to resolve the target as the requested type.
	targetRecs, err := b.storage.Get(ctx, target, targetType)
	if err == nil {
		result = append(result, targetRecs...)
		return result, nil
	}

	// Target might itself be a CNAME; recurse.
	deeper, err := b.resolveCNAMEChain(ctx, target, targetType, depth+1)
	if err != nil {
		return nil, err
	}
	result = append(result, deeper...)
	return result, nil
}

// resolveAny returns all record types stored for the given domain.
func (b *Backend) resolveAny(ctx context.Context, domain string) ([]*types.DNSRecord, error) {
	all, err := b.storage.List(ctx)
	if err != nil {
		return nil, err
	}

	var matched []*types.DNSRecord
	for _, r := range all {
		if r.Name == domain {
			matched = append(matched, r)
		}
	}

	if len(matched) == 0 {
		return nil, types.ErrRecordNotFound
	}

	matched, _ = b.ApplyRules(ctx, matched)
	return matched, nil
}
