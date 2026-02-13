package dns

import (
	"context"
	"log/slog"

	"jabberwocky238/jw238dns/geoip"
	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

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
	DefaultTTL          uint32 // Default TTL when record has none
	ResolveCNAMEChain   bool   // Follow CNAME chains
	MaxCNAMEDepth       int    // Maximum CNAME chain depth
	ReturnSOAOnNXDOMAIN bool   // Attach SOA to NXDOMAIN responses
	EnableGeoIP         bool   // Enable GeoIP distance-based sorting
	MMDBPath            string // Path to MaxMind MMDB file

	// Upstream forwarding configuration.
	Forwarder ForwarderConfig // Upstream DNS forwarder configuration
}

// DefaultBackendConfig returns a BackendConfig with sensible defaults.
func DefaultBackendConfig() BackendConfig {
	return BackendConfig{
		DefaultTTL:          300,
		ResolveCNAMEChain:   true,
		MaxCNAMEDepth:       10,
		ReturnSOAOnNXDOMAIN: true,
		Forwarder:           DefaultForwarderConfig(),
	}
}

// Backend implements DNSBackend using a CoreStorage for lookups.
type Backend struct {
	storage   storage.CoreStorage
	config    BackendConfig
	geoReader geoip.IPLookup
	geoCloser func() error
	forwarder *Forwarder
}

// NewBackend creates a Backend backed by the given storage and config.
// If GeoIP is enabled but the MMDB file cannot be opened, GeoIP is
// disabled and a warning is logged.
func NewBackend(store storage.CoreStorage, cfg BackendConfig) *Backend {
	b := &Backend{
		storage:   store,
		config:    cfg,
		forwarder: NewForwarder(cfg.Forwarder),
	}

	if cfg.EnableGeoIP && cfg.MMDBPath != "" {
		reader, err := geoip.NewReader(cfg.MMDBPath)
		if err != nil {
			slog.Warn("GeoIP disabled: failed to open MMDB",
				"path", cfg.MMDBPath,
				"error", err,
			)
			b.config.EnableGeoIP = false
		} else {
			b.geoReader = reader
			b.geoCloser = reader.Close
		}
	}

	return b
}

// Close releases resources held by the Backend, including the GeoIP reader.
func (b *Backend) Close() error {
	if b.geoCloser != nil {
		return b.geoCloser()
	}
	return nil
}

// Resolve looks up records from storage. For non-SOA queries it will follow
// CNAME chains when configured. If no records are found it returns
// ErrRecordNotFound.
func (b *Backend) Resolve(ctx context.Context, query *types.QueryInfo) ([]*types.DNSRecord, error) {
	rt := uint16ToRecordType(query.Type)

	// ANY query: return all record types for the domain.
	if query.Type == dns.TypeANY {
		recs, err := b.resolveAny(ctx, query.Domain)
		if err != nil {
			return nil, err
		}
		b.applyGeoSort(recs, query)
		return recs, nil
	}

	// Direct lookup.
	recs, err := b.storage.Get(ctx, query.Domain, rt)
	if err == nil {
		recs, _ = b.ApplyRules(ctx, recs)
		b.applyGeoSort(recs, query)
		return recs, nil
	}

	// If the requested type is not CNAME, check whether a CNAME exists and
	// follow the chain.
	if b.config.ResolveCNAMEChain && rt != types.RecordTypeCNAME {
		chainRecs, chainErr := b.resolveCNAMEChain(ctx, query.Domain, rt, 0)
		if chainErr == nil && len(chainRecs) > 0 {
			chainRecs, _ = b.ApplyRules(ctx, chainRecs)
			b.applyGeoSort(chainRecs, query)
			return chainRecs, nil
		}
	}

	// Forward to upstream DNS if enabled and local lookup missed.
	if b.forwarder != nil {
		upstreamRecs, upstreamErr := b.forwarder.Forward(ctx, query.Domain, query.Type)
		if upstreamErr == nil && len(upstreamRecs) > 0 {
			return upstreamRecs, nil
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

// applyGeoSort sorts A and AAAA record values by distance from the client
// IP when GeoIP is enabled. If the client IP is missing or cannot be looked
// up, sorting is silently skipped.
func (b *Backend) applyGeoSort(records []*types.DNSRecord, query *types.QueryInfo) {
	if !b.config.EnableGeoIP || b.geoReader == nil {
		return
	}
	if query.ClientIP == nil {
		return
	}

	clientCoords, err := b.geoReader.Lookup(query.ClientIP)
	if err != nil || clientCoords == nil {
		return
	}

	geoip.SortRecordsByDistance(records, *clientCoords, b.geoReader)
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

