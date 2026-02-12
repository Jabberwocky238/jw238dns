package dns

import (
	"testing"

	"jabberwocky238/jw238dns/types"

	"github.com/miekg/dns"
)

func TestBuildRR_AllTypes(t *testing.T) {
	tests := []struct {
		name       string
		record     *types.DNSRecord
		wantNil    bool
		wantRRType uint16
	}{
		{
			name:       "A record",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"192.168.1.1"}},
			wantNil:    false,
			wantRRType: dns.TypeA,
		},
		{
			name:       "AAAA record",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeAAAA, TTL: 300, Value: []string{"2001:db8::1"}},
			wantNil:    false,
			wantRRType: dns.TypeAAAA,
		},
		{
			name:       "CNAME record",
			record:     &types.DNSRecord{Name: "www.a.com.", Type: types.RecordTypeCNAME, TTL: 300, Value: []string{"a.com."}},
			wantNil:    false,
			wantRRType: dns.TypeCNAME,
		},
		{
			name:       "MX record",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeMX, TTL: 300, Value: []string{"mail.a.com."}},
			wantNil:    false,
			wantRRType: dns.TypeMX,
		},
		{
			name:       "TXT record",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeTXT, TTL: 300, Value: []string{"v=spf1 ~all"}},
			wantNil:    false,
			wantRRType: dns.TypeTXT,
		},
		{
			name:       "NS record",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeNS, TTL: 300, Value: []string{"ns1.a.com."}},
			wantNil:    false,
			wantRRType: dns.TypeNS,
		},
		{
			name:       "PTR record",
			record:     &types.DNSRecord{Name: "1.1.168.192.in-addr.arpa.", Type: types.RecordTypePTR, TTL: 300, Value: []string{"host.a.com."}},
			wantNil:    false,
			wantRRType: dns.TypePTR,
		},
		{
			name:       "SOA record with full fields",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeSOA, TTL: 300, Value: []string{"ns1.a.com. admin.a.com. 2024 3600 900 604800 86400"}},
			wantNil:    false,
			wantRRType: dns.TypeSOA,
		},
		{
			name:       "CAA record",
			record:     &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeCAA, TTL: 300, Value: []string{"letsencrypt.org"}},
			wantNil:    false,
			wantRRType: dns.TypeCAA,
		},
		{
			name:       "SRV record",
			record:     &types.DNSRecord{Name: "_sip._tcp.a.com.", Type: types.RecordTypeSRV, TTL: 300, Value: []string{"10 60 5060 sip.a.com."}},
			wantNil:    false,
			wantRRType: dns.TypeSRV,
		},
		{
			name:    "empty value returns nil",
			record:  &types.DNSRecord{Name: "a.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{}},
			wantNil: true,
		},
		{
			name:    "unknown type returns nil",
			record:  &types.DNSRecord{Name: "a.com.", Type: types.RecordType("UNKNOWN"), TTL: 300, Value: []string{"data"}},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rr := buildRR(tt.record)
			if tt.wantNil {
				if rr != nil {
					t.Errorf("buildRR() = %v, want nil", rr)
				}
				return
			}
			if rr == nil {
				t.Fatal("buildRR() = nil, want non-nil")
			}
			if rr.Header().Rrtype != tt.wantRRType {
				t.Errorf("RR type = %d, want %d", rr.Header().Rrtype, tt.wantRRType)
			}
			if rr.Header().Ttl != tt.record.TTL {
				t.Errorf("RR TTL = %d, want %d", rr.Header().Ttl, tt.record.TTL)
			}
		})
	}
}

func TestBuildSOA(t *testing.T) {
	hdr := dns.RR_Header{Name: "a.com.", Rrtype: dns.TypeSOA, Class: dns.ClassINET, Ttl: 300}

	tests := []struct {
		name       string
		values     []string
		wantNs     string
		wantMbox   string
		wantSerial uint32
	}{
		{
			name:       "empty values uses defaults",
			values:     []string{},
			wantNs:     "ns1.example.com.",
			wantMbox:   "admin.example.com.",
			wantSerial: 1,
		},
		{
			name:       "full 7-field SOA",
			values:     []string{"ns1.a.com. admin.a.com. 100 7200 1800 1209600 172800"},
			wantNs:     "ns1.a.com.",
			wantMbox:   "admin.a.com.",
			wantSerial: 100,
		},
		{
			name:       "partial 2-field SOA",
			values:     []string{"ns2.b.com. hostmaster.b.com."},
			wantNs:     "ns2.b.com.",
			wantMbox:   "hostmaster.b.com.",
			wantSerial: 1, // default
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			soa := buildSOA(hdr, tt.values)
			if soa.Ns != tt.wantNs {
				t.Errorf("Ns = %q, want %q", soa.Ns, tt.wantNs)
			}
			if soa.Mbox != tt.wantMbox {
				t.Errorf("Mbox = %q, want %q", soa.Mbox, tt.wantMbox)
			}
			if soa.Serial != tt.wantSerial {
				t.Errorf("Serial = %d, want %d", soa.Serial, tt.wantSerial)
			}
		})
	}
}

func TestBuildSRV(t *testing.T) {
	hdr := dns.RR_Header{Name: "_sip._tcp.a.com.", Rrtype: dns.TypeSRV, Class: dns.ClassINET, Ttl: 300}

	tests := []struct {
		name         string
		values       []string
		wantPriority uint16
		wantWeight   uint16
		wantPort     uint16
		wantTarget   string
	}{
		{
			name:         "full SRV fields",
			values:       []string{"10 60 5060 sip.a.com."},
			wantPriority: 10,
			wantWeight:   60,
			wantPort:     5060,
			wantTarget:   "sip.a.com.",
		},
		{
			name:       "empty values",
			values:     []string{},
			wantTarget: "",
		},
		{
			name:       "target only",
			values:     []string{"sip.a.com."},
			wantTarget: "sip.a.com.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := buildSRV(hdr, tt.values)
			if srv.Priority != tt.wantPriority {
				t.Errorf("Priority = %d, want %d", srv.Priority, tt.wantPriority)
			}
			if srv.Weight != tt.wantWeight {
				t.Errorf("Weight = %d, want %d", srv.Weight, tt.wantWeight)
			}
			if srv.Port != tt.wantPort {
				t.Errorf("Port = %d, want %d", srv.Port, tt.wantPort)
			}
			if srv.Target != tt.wantTarget {
				t.Errorf("Target = %q, want %q", srv.Target, tt.wantTarget)
			}
		})
	}
}

func TestRecordTypeToUint16(t *testing.T) {
	tests := []struct {
		name string
		rt   types.RecordType
		want uint16
	}{
		{"A", types.RecordTypeA, dns.TypeA},
		{"AAAA", types.RecordTypeAAAA, dns.TypeAAAA},
		{"CNAME", types.RecordTypeCNAME, dns.TypeCNAME},
		{"MX", types.RecordTypeMX, dns.TypeMX},
		{"TXT", types.RecordTypeTXT, dns.TypeTXT},
		{"NS", types.RecordTypeNS, dns.TypeNS},
		{"SRV", types.RecordTypeSRV, dns.TypeSRV},
		{"PTR", types.RecordTypePTR, dns.TypePTR},
		{"SOA", types.RecordTypeSOA, dns.TypeSOA},
		{"CAA", types.RecordTypeCAA, dns.TypeCAA},
		{"unknown", types.RecordType("BOGUS"), 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := recordTypeToUint16(tt.rt)
			if got != tt.want {
				t.Errorf("recordTypeToUint16(%q) = %d, want %d", tt.rt, got, tt.want)
			}
		})
	}
}

func TestUint16ToRecordType(t *testing.T) {
	tests := []struct {
		name string
		qt   uint16
		want types.RecordType
	}{
		{"A", dns.TypeA, types.RecordTypeA},
		{"AAAA", dns.TypeAAAA, types.RecordTypeAAAA},
		{"CNAME", dns.TypeCNAME, types.RecordTypeCNAME},
		{"MX", dns.TypeMX, types.RecordTypeMX},
		{"TXT", dns.TypeTXT, types.RecordTypeTXT},
		{"NS", dns.TypeNS, types.RecordTypeNS},
		{"SRV", dns.TypeSRV, types.RecordTypeSRV},
		{"PTR", dns.TypePTR, types.RecordTypePTR},
		{"SOA", dns.TypeSOA, types.RecordTypeSOA},
		{"CAA", dns.TypeCAA, types.RecordTypeCAA},
		{"unknown", 9999, types.RecordType("")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := uint16ToRecordType(tt.qt)
			if got != tt.want {
				t.Errorf("uint16ToRecordType(%d) = %q, want %q", tt.qt, got, tt.want)
			}
		})
	}
}

func TestExtractZone(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   string
	}{
		{"simple domain", "example.com.", "example.com."},
		{"subdomain", "sub.example.com.", "example.com."},
		{"deep subdomain", "a.b.c.example.com.", "example.com."},
		{"single label", "localhost.", "localhost."},
		{"two labels no dot", "example.com", "example.com."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractZone(tt.domain)
			if got != tt.want {
				t.Errorf("extractZone(%q) = %q, want %q", tt.domain, got, tt.want)
			}
		})
	}
}

func TestSoaForZone(t *testing.T) {
	rec := soaForZone("sub.example.com.")
	if rec.Name != "example.com." {
		t.Errorf("Name = %q, want %q", rec.Name, "example.com.")
	}
	if rec.Type != types.RecordTypeSOA {
		t.Errorf("Type = %q, want %q", rec.Type, types.RecordTypeSOA)
	}
	if len(rec.Value) == 0 {
		t.Fatal("Value should not be empty")
	}
}

func TestParseIP(t *testing.T) {
	tests := []struct {
		name  string
		input string
		isNil bool
	}{
		{"valid IPv4", "192.168.1.1", false},
		{"valid IPv6", "2001:db8::1", false},
		{"invalid", "not-an-ip", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := parseIP(tt.input)
			if (ip == nil) != tt.isNil {
				t.Errorf("parseIP(%q) nil = %v, want nil = %v", tt.input, ip == nil, tt.isNil)
			}
		})
	}
}
