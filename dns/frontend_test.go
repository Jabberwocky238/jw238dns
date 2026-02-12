package dns

import (
	"context"
	"testing"

	"jabberwocky238/jw238dns/storage"
	"jabberwocky238/jw238dns/types"

	"github.com/miekg/dns"
)

func setupFrontend(t *testing.T) (*Frontend, *storage.MemoryStorage) {
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

	backend := NewBackend(store, DefaultBackendConfig())
	frontend := NewFrontend(backend)
	return frontend, store
}

func TestFrontend_ParseQuery(t *testing.T) {
	fe, _ := setupFrontend(t)

	tests := []struct {
		name       string
		buildQuery func() *dns.Msg
		wantDomain string
		wantType   uint16
		wantErr    bool
	}{
		{
			name: "valid A query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeA)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeA,
			wantErr:    false,
		},
		{
			name: "valid AAAA query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeAAAA)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeAAAA,
			wantErr:    false,
		},
		{
			name: "valid MX query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeMX)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeMX,
			wantErr:    false,
		},
		{
			name: "valid TXT query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeTXT)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeTXT,
			wantErr:    false,
		},
		{
			name: "valid CNAME query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("www.example.com.", dns.TypeCNAME)
				return m
			},
			wantDomain: "www.example.com.",
			wantType:   dns.TypeCNAME,
			wantErr:    false,
		},
		{
			name: "valid NS query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeNS)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeNS,
			wantErr:    false,
		},
		{
			name: "valid SOA query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeSOA)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeSOA,
			wantErr:    false,
		},
		{
			name: "valid SRV query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("_sip._tcp.example.com.", dns.TypeSRV)
				return m
			},
			wantDomain: "_sip._tcp.example.com.",
			wantType:   dns.TypeSRV,
			wantErr:    false,
		},
		{
			name: "valid PTR query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("1.1.168.192.in-addr.arpa.", dns.TypePTR)
				return m
			},
			wantDomain: "1.1.168.192.in-addr.arpa.",
			wantType:   dns.TypePTR,
			wantErr:    false,
		},
		{
			name: "valid CAA query",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.SetQuestion("example.com.", dns.TypeCAA)
				return m
			},
			wantDomain: "example.com.",
			wantType:   dns.TypeCAA,
			wantErr:    false,
		},
		{
			name:       "nil query",
			buildQuery: func() *dns.Msg { return nil },
			wantErr:    true,
		},
		{
			name: "empty question section",
			buildQuery: func() *dns.Msg {
				m := new(dns.Msg)
				m.Id = dns.Id()
				return m
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			info, err := fe.ParseQuery(tt.buildQuery())
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseQuery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if info.Domain != tt.wantDomain {
					t.Errorf("Domain = %q, want %q", info.Domain, tt.wantDomain)
				}
				if info.Type != tt.wantType {
					t.Errorf("Type = %d, want %d", info.Type, tt.wantType)
				}
			}
		})
	}
}

func TestFrontend_ReceiveQuery_Success(t *testing.T) {
	fe, _ := setupFrontend(t)
	ctx := context.Background()

	tests := []struct {
		name      string
		qname     string
		qtype     uint16
		wantRcode int
		wantCount int
	}{
		{
			name:      "A record found",
			qname:     "example.com.",
			qtype:     dns.TypeA,
			wantRcode: dns.RcodeSuccess,
			wantCount: 1,
		},
		{
			name:      "AAAA record found",
			qname:     "example.com.",
			qtype:     dns.TypeAAAA,
			wantRcode: dns.RcodeSuccess,
			wantCount: 1,
		},
		{
			name:      "TXT record found",
			qname:     "example.com.",
			qtype:     dns.TypeTXT,
			wantRcode: dns.RcodeSuccess,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			query := new(dns.Msg)
			query.SetQuestion(tt.qname, tt.qtype)

			resp, err := fe.ReceiveQuery(ctx, query)
			if err != nil {
				t.Fatalf("ReceiveQuery() error = %v", err)
			}
			if resp.Rcode != tt.wantRcode {
				t.Errorf("Rcode = %d, want %d", resp.Rcode, tt.wantRcode)
			}
			if len(resp.Answer) != tt.wantCount {
				t.Errorf("Answer count = %d, want %d", len(resp.Answer), tt.wantCount)
			}
		})
	}
}

func TestFrontend_ReceiveQuery_NXDOMAIN(t *testing.T) {
	fe, _ := setupFrontend(t)
	ctx := context.Background()

	query := new(dns.Msg)
	query.SetQuestion("notfound.com.", dns.TypeA)

	resp, err := fe.ReceiveQuery(ctx, query)
	if err != nil {
		t.Fatalf("ReceiveQuery() error = %v", err)
	}
	if resp.Rcode != dns.RcodeNameError {
		t.Errorf("Rcode = %d, want %d (NXDOMAIN)", resp.Rcode, dns.RcodeNameError)
	}
	if len(resp.Answer) != 0 {
		t.Errorf("Answer count = %d, want 0", len(resp.Answer))
	}
}

func TestFrontend_ReceiveQuery_FormatError(t *testing.T) {
	fe, _ := setupFrontend(t)
	ctx := context.Background()

	// Empty question section.
	query := new(dns.Msg)
	query.Id = dns.Id()

	resp, err := fe.ReceiveQuery(ctx, query)
	if err == nil {
		t.Error("ReceiveQuery() expected error for malformed query")
	}
	if resp.Rcode != dns.RcodeFormatError {
		t.Errorf("Rcode = %d, want %d (FORMERR)", resp.Rcode, dns.RcodeFormatError)
	}
}
