package dns

import (
	"context"
	"fmt"
	"net"
	"testing"

	"jabberwocky238/jw238dns/geoip"
	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

	"github.com/miekg/dns"
)

func setupBackend(t *testing.T) (*Backend, *storage.MemoryStorage) {
	t.Helper()
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "example.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"192.168.1.1"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "example.com.", Type: types.RecordTypeAAAA, TTL: 300, Value: []string{"2001:db8::1"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "example.com.", Type: types.RecordTypeTXT, TTL: 300, Value: []string{"v=spf1 ~all"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "example.com.", Type: types.RecordTypeMX, TTL: 300, Value: []string{"mail.example.com."},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "example.com.", Type: types.RecordTypeNS, TTL: 300, Value: []string{"ns1.example.com."},
	})
	// CNAME chain: www -> example.com
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "www.example.com.", Type: types.RecordTypeCNAME, TTL: 300, Value: []string{"example.com."},
	})
	// Deeper CNAME chain: alias -> www -> example.com
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "alias.example.com.", Type: types.RecordTypeCNAME, TTL: 300, Value: []string{"www.example.com."},
	})

	backend := NewBackend(store, DefaultBackendConfig())
	return backend, store
}

func TestBackend_Resolve(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	tests := []struct {
		name       string
		query      *types.QueryInfo
		wantErr    bool
		wantCount  int
		wantValue  string
	}{
		{
			name:      "A record",
			query:     &types.QueryInfo{Domain: "example.com.", Type: dns.TypeA, Class: dns.ClassINET},
			wantErr:   false,
			wantCount: 1,
			wantValue: "192.168.1.1",
		},
		{
			name:      "AAAA record",
			query:     &types.QueryInfo{Domain: "example.com.", Type: dns.TypeAAAA, Class: dns.ClassINET},
			wantErr:   false,
			wantCount: 1,
			wantValue: "2001:db8::1",
		},
		{
			name:      "TXT record",
			query:     &types.QueryInfo{Domain: "example.com.", Type: dns.TypeTXT, Class: dns.ClassINET},
			wantErr:   false,
			wantCount: 1,
			wantValue: "v=spf1 ~all",
		},
		{
			name:      "MX record",
			query:     &types.QueryInfo{Domain: "example.com.", Type: dns.TypeMX, Class: dns.ClassINET},
			wantErr:   false,
			wantCount: 1,
			wantValue: "mail.example.com.",
		},
		{
			name:      "NS record",
			query:     &types.QueryInfo{Domain: "example.com.", Type: dns.TypeNS, Class: dns.ClassINET},
			wantErr:   false,
			wantCount: 1,
			wantValue: "ns1.example.com.",
		},
		{
			name:    "non-existent domain",
			query:   &types.QueryInfo{Domain: "notfound.com.", Type: dns.TypeA, Class: dns.ClassINET},
			wantErr: true,
		},
		{
			name:    "non-existent type for existing domain",
			query:   &types.QueryInfo{Domain: "example.com.", Type: dns.TypeSRV, Class: dns.ClassINET},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs, err := backend.Resolve(ctx, tt.query)
			if (err != nil) != tt.wantErr {
				t.Errorf("Resolve() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if len(recs) != tt.wantCount {
					t.Errorf("Resolve() returned %d records, want %d", len(recs), tt.wantCount)
					return
				}
				if recs[0].Value[0] != tt.wantValue {
					t.Errorf("Resolve() value = %q, want %q", recs[0].Value[0], tt.wantValue)
				}
			}
		})
	}
}

func TestBackend_Resolve_CNAMEChain(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	tests := []struct {
		name      string
		domain    string
		qtype     uint16
		wantCount int
		wantLast  string
	}{
		{
			name:      "single CNAME hop www -> example.com A",
			domain:    "www.example.com.",
			qtype:     dns.TypeA,
			wantCount: 2, // CNAME + A
			wantLast:  "192.168.1.1",
		},
		{
			name:      "double CNAME hop alias -> www -> example.com A",
			domain:    "alias.example.com.",
			qtype:     dns.TypeA,
			wantCount: 3, // CNAME + CNAME + A
			wantLast:  "192.168.1.1",
		},
		{
			name:      "CNAME to AAAA",
			domain:    "www.example.com.",
			qtype:     dns.TypeAAAA,
			wantCount: 2, // CNAME + AAAA
			wantLast:  "2001:db8::1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recs, err := backend.Resolve(ctx, &types.QueryInfo{
				Domain: tt.domain, Type: tt.qtype, Class: dns.ClassINET,
			})
			if err != nil {
				t.Fatalf("Resolve() error = %v", err)
			}
			if len(recs) != tt.wantCount {
				t.Errorf("Resolve() returned %d records, want %d", len(recs), tt.wantCount)
				return
			}
			last := recs[len(recs)-1]
			if last.Value[0] != tt.wantLast {
				t.Errorf("last record value = %q, want %q", last.Value[0], tt.wantLast)
			}
		})
	}
}

func TestBackend_Resolve_CNAMEDepthLimit(t *testing.T) {
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	// Build a CNAME chain longer than MaxCNAMEDepth (default 10).
	for i := 0; i < 12; i++ {
		var target string
		if i < 11 {
			target = chainName(i + 1)
		} else {
			target = "final.com."
		}
		_ = store.Create(ctx, &types.DNSRecord{
			Name: chainName(i), Type: types.RecordTypeCNAME, TTL: 300, Value: []string{target},
		})
	}
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "final.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
	})

	backend := NewBackend(store, DefaultBackendConfig())

	_, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain: chainName(0), Type: dns.TypeA, Class: dns.ClassINET,
	})
	if err != types.ErrRecordNotFound {
		t.Errorf("Resolve() error = %v, want ErrRecordNotFound (depth exceeded)", err)
	}
}

func chainName(i int) string {
	return "chain" + string(rune('a'+i)) + ".com."
}

func TestBackend_Resolve_ANY(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain: "example.com.", Type: dns.TypeANY, Class: dns.ClassINET,
	})
	if err != nil {
		t.Fatalf("Resolve(ANY) error = %v", err)
	}
	// example.com. has A, AAAA, TXT, MX, NS = 5 records
	if len(recs) < 5 {
		t.Errorf("Resolve(ANY) returned %d records, want >= 5", len(recs))
	}
}

func TestBackend_Resolve_ANY_NotFound(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	_, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain: "notfound.com.", Type: dns.TypeANY, Class: dns.ClassINET,
	})
	if err != types.ErrRecordNotFound {
		t.Errorf("Resolve(ANY) error = %v, want ErrRecordNotFound", err)
	}
}

func TestBackend_ApplyRules_DefaultTTL(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	records := []*types.DNSRecord{
		{Name: "a.com.", Type: types.RecordTypeA, TTL: 0, Value: []string{"1.2.3.4"}},
		{Name: "b.com.", Type: types.RecordTypeA, TTL: 600, Value: []string{"5.6.7.8"}},
	}

	result, err := backend.ApplyRules(ctx, records)
	if err != nil {
		t.Fatalf("ApplyRules() error = %v", err)
	}

	if result[0].TTL != 300 {
		t.Errorf("record with TTL=0 should get default TTL 300, got %d", result[0].TTL)
	}
	if result[1].TTL != 600 {
		t.Errorf("record with TTL=600 should keep TTL 600, got %d", result[1].TTL)
	}
}

func TestBackend_Resolve_CNAMEDisabled(t *testing.T) {
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "target.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"1.2.3.4"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "alias.com.", Type: types.RecordTypeCNAME, TTL: 300, Value: []string{"target.com."},
	})

	cfg := DefaultBackendConfig()
	cfg.ResolveCNAMEChain = false
	backend := NewBackend(store, cfg)

	_, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain: "alias.com.", Type: dns.TypeA, Class: dns.ClassINET,
	})
	if err != types.ErrRecordNotFound {
		t.Errorf("Resolve() with CNAME disabled error = %v, want ErrRecordNotFound", err)
	}
}

func TestBackend_Resolve_DirectCNAME(t *testing.T) {
	backend, _ := setupBackend(t)
	ctx := context.Background()

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain: "www.example.com.", Type: dns.TypeCNAME, Class: dns.ClassINET,
	})
	if err != nil {
		t.Fatalf("Resolve(CNAME) error = %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("Resolve(CNAME) returned %d records, want 1", len(recs))
	}
	if recs[0].Value[0] != "example.com." {
		t.Errorf("CNAME target = %q, want %q", recs[0].Value[0], "example.com.")
	}
}

// mockGeoLookup implements geoip.IPLookup for backend GeoIP tests.
type mockGeoLookup struct {
	coords map[string]*geoip.Coordinates
}

func (m *mockGeoLookup) Lookup(ip net.IP) (*geoip.Coordinates, error) {
	c, ok := m.coords[ip.String()]
	if !ok {
		return nil, fmt.Errorf("IP not found: %s", ip)
	}
	return c, nil
}

func newMockGeoLookup() *mockGeoLookup {
	return &mockGeoLookup{
		coords: map[string]*geoip.Coordinates{
			// Client in New York
			"203.0.113.1": {Latitude: 40.7128, Longitude: -74.0060},
			// Server in Toronto (close to NY)
			"10.0.0.1": {Latitude: 43.6532, Longitude: -79.3832},
			// Server in London
			"10.0.0.2": {Latitude: 51.5074, Longitude: -0.1278},
			// Server in Tokyo (far from NY)
			"10.0.0.3": {Latitude: 35.6762, Longitude: 139.6503},
		},
	}
}

func setupGeoBackend(t *testing.T) (*Backend, *storage.MemoryStorage) {
	t.Helper()
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	// A record with multiple IPs (Tokyo, London, Toronto order)
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "geo.example.com.", Type: types.RecordTypeA, TTL: 300,
		Value: []string{"10.0.0.3", "10.0.0.2", "10.0.0.1"},
	})
	_ = store.Create(ctx, &types.DNSRecord{
		Name: "geo.example.com.", Type: types.RecordTypeTXT, TTL: 300,
		Value: []string{"v=spf1 ~all"},
	})

	cfg := DefaultBackendConfig()
	cfg.EnableGeoIP = true
	backend := NewBackend(store, cfg)
	backend.geoReader = newMockGeoLookup()

	return backend, store
}

func TestBackend_GeoIP_SortsARecordsByDistance(t *testing.T) {
	backend, _ := setupGeoBackend(t)
	ctx := context.Background()

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain:   "geo.example.com.",
		Type:     dns.TypeA,
		Class:    dns.ClassINET,
		ClientIP: net.ParseIP("203.0.113.1"), // New York
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if len(recs) != 1 {
		t.Fatalf("Resolve() returned %d records, want 1", len(recs))
	}

	// Expected order: Toronto (closest), London, Tokyo (farthest)
	if recs[0].Value[0] != "10.0.0.1" {
		t.Errorf("closest IP should be Toronto (10.0.0.1), got %s", recs[0].Value[0])
	}
	if recs[0].Value[1] != "10.0.0.2" {
		t.Errorf("middle IP should be London (10.0.0.2), got %s", recs[0].Value[1])
	}
	if recs[0].Value[2] != "10.0.0.3" {
		t.Errorf("farthest IP should be Tokyo (10.0.0.3), got %s", recs[0].Value[2])
	}
}

func TestBackend_GeoIP_SkipsWhenNoClientIP(t *testing.T) {
	backend, _ := setupGeoBackend(t)
	ctx := context.Background()

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain: "geo.example.com.",
		Type:   dns.TypeA,
		Class:  dns.ClassINET,
		// No ClientIP set
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Original order preserved: Tokyo, London, Toronto
	if recs[0].Value[0] != "10.0.0.3" {
		t.Errorf("without client IP, original order should be preserved, got %v", recs[0].Value)
	}
}

func TestBackend_GeoIP_SkipsNonARecords(t *testing.T) {
	backend, _ := setupGeoBackend(t)
	ctx := context.Background()

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain:   "geo.example.com.",
		Type:     dns.TypeTXT,
		Class:    dns.ClassINET,
		ClientIP: net.ParseIP("203.0.113.1"),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	if recs[0].Value[0] != "v=spf1 ~all" {
		t.Errorf("TXT record should be unchanged, got %v", recs[0].Value)
	}
}

func TestBackend_GeoIP_SkipsWhenDisabled(t *testing.T) {
	store := storage.NewMemoryStorage()
	ctx := context.Background()

	_ = store.Create(ctx, &types.DNSRecord{
		Name: "geo.example.com.", Type: types.RecordTypeA, TTL: 300,
		Value: []string{"10.0.0.3", "10.0.0.2", "10.0.0.1"},
	})

	cfg := DefaultBackendConfig()
	cfg.EnableGeoIP = false
	backend := NewBackend(store, cfg)

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain:   "geo.example.com.",
		Type:     dns.TypeA,
		Class:    dns.ClassINET,
		ClientIP: net.ParseIP("203.0.113.1"),
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Original order preserved when GeoIP disabled
	if recs[0].Value[0] != "10.0.0.3" {
		t.Errorf("with GeoIP disabled, original order should be preserved, got %v", recs[0].Value)
	}
}

func TestBackend_GeoIP_SkipsUnknownClientIP(t *testing.T) {
	backend, _ := setupGeoBackend(t)
	ctx := context.Background()

	recs, err := backend.Resolve(ctx, &types.QueryInfo{
		Domain:   "geo.example.com.",
		Type:     dns.TypeA,
		Class:    dns.ClassINET,
		ClientIP: net.ParseIP("192.168.99.99"), // Not in mock lookup
	})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	// Original order preserved when client IP not in DB
	if recs[0].Value[0] != "10.0.0.3" {
		t.Errorf("with unknown client IP, original order should be preserved, got %v", recs[0].Value)
	}
}

func TestBackend_NewBackend_GeoIPInvalidPath(t *testing.T) {
	store := storage.NewMemoryStorage()

	cfg := DefaultBackendConfig()
	cfg.EnableGeoIP = true
	cfg.MMDBPath = "/nonexistent/path.mmdb"

	backend := NewBackend(store, cfg)

	// GeoIP should be disabled after failing to open MMDB
	if backend.config.EnableGeoIP {
		t.Error("EnableGeoIP should be false after MMDB open failure")
	}
	if backend.geoReader != nil {
		t.Error("geoReader should be nil after MMDB open failure")
	}
}

func TestBackend_Close_NoGeoIP(t *testing.T) {
	backend, _ := setupBackend(t)
	err := backend.Close()
	if err != nil {
		t.Errorf("Close() without GeoIP should return nil, got %v", err)
	}
}
