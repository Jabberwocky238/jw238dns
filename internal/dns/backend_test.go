package dns

import (
	"context"
	"testing"

	"jabberwocky238/jw238dns/internal/storage"
	"jabberwocky238/jw238dns/internal/types"

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
