package dns

import (
	"context"
	"net"
	"testing"
	"time"

	"jabberwocky238/jw238dns/types"

	"github.com/miekg/dns"
)

func TestNewForwarder(t *testing.T) {
	cfg := ForwarderConfig{
		Enabled: true,
		Servers: []string{"1.1.1.1:53", "8.8.8.8:53"},
		Timeout: 3 * time.Second,
	}

	f := NewForwarder(cfg)
	if f == nil {
		t.Fatal("NewForwarder returned nil")
	}
	if f.config.Enabled != true {
		t.Error("Expected Enabled to be true")
	}
	if len(f.config.Servers) != 2 {
		t.Errorf("Expected 2 servers, got %d", len(f.config.Servers))
	}
	if f.config.Timeout != 3*time.Second {
		t.Errorf("Expected timeout 3s, got %v", f.config.Timeout)
	}
	if f.client == nil {
		t.Error("Expected client to be initialized")
	}
}

func TestDefaultForwarderConfig(t *testing.T) {
	cfg := DefaultForwarderConfig()
	if cfg.Enabled != false {
		t.Error("Expected Enabled to be false by default")
	}
	if len(cfg.Servers) != 1 || cfg.Servers[0] != "1.1.1.1:53" {
		t.Errorf("Expected default server 1.1.1.1:53, got %v", cfg.Servers)
	}
	if cfg.Timeout != 5*time.Second {
		t.Errorf("Expected default timeout 5s, got %v", cfg.Timeout)
	}
}

func TestForwarder_Forward_Disabled(t *testing.T) {
	cfg := ForwarderConfig{
		Enabled: false,
		Servers: []string{"1.1.1.1:53"},
		Timeout: 5 * time.Second,
	}
	f := NewForwarder(cfg)

	ctx := context.Background()
	records, err := f.Forward(ctx, "example.com.", dns.TypeA)

	if err != types.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound when disabled, got %v", err)
	}
	if records != nil {
		t.Error("Expected nil records when disabled")
	}
}

func TestForwarder_Forward_NoServers(t *testing.T) {
	cfg := ForwarderConfig{
		Enabled: true,
		Servers: []string{},
		Timeout: 5 * time.Second,
	}
	f := NewForwarder(cfg)

	ctx := context.Background()
	records, err := f.Forward(ctx, "example.com.", dns.TypeA)

	if err != types.ErrRecordNotFound {
		t.Errorf("Expected ErrRecordNotFound with no servers, got %v", err)
	}
	if records != nil {
		t.Error("Expected nil records with no servers")
	}
}

func TestForwarder_rrToRecords_A(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.A{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: net.ParseIP("1.2.3.4"),
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Name != "example.com." {
		t.Errorf("Expected name example.com., got %s", rec.Name)
	}
	if rec.Type != types.RecordTypeA {
		t.Errorf("Expected type A, got %s", rec.Type)
	}
	if rec.TTL != 300 {
		t.Errorf("Expected TTL 300, got %d", rec.TTL)
	}
	if len(rec.Value) != 1 || rec.Value[0] != "1.2.3.4" {
		t.Errorf("Expected value [1.2.3.4], got %v", rec.Value)
	}
}

func TestForwarder_rrToRecords_AAAA(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.AAAA{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeAAAA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		AAAA: net.ParseIP("2001:db8::1"),
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeAAAA {
		t.Errorf("Expected type AAAA, got %s", rec.Type)
	}
	if len(rec.Value) != 1 || rec.Value[0] != "2001:db8::1" {
		t.Errorf("Expected value [2001:db8::1], got %v", rec.Value)
	}
}

func TestForwarder_rrToRecords_CNAME(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.CNAME{
		Hdr: dns.RR_Header{
			Name:   "www.example.com.",
			Rrtype: dns.TypeCNAME,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Target: "example.com.",
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeCNAME {
		t.Errorf("Expected type CNAME, got %s", rec.Type)
	}
	if len(rec.Value) != 1 || rec.Value[0] != "example.com." {
		t.Errorf("Expected value [example.com.], got %v", rec.Value)
	}
}

func TestForwarder_rrToRecords_TXT(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.TXT{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeTXT,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Txt: []string{"v=spf1 include:_spf.example.com ~all"},
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeTXT {
		t.Errorf("Expected type TXT, got %s", rec.Type)
	}
	if len(rec.Value) != 1 || rec.Value[0] != "v=spf1 include:_spf.example.com ~all" {
		t.Errorf("Expected SPF value, got %v", rec.Value)
	}
}

func TestForwarder_rrToRecords_MX(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.MX{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeMX,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Preference: 10,
		Mx:         "mail.example.com.",
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeMX {
		t.Errorf("Expected type MX, got %s", rec.Type)
	}
	if len(rec.Value) != 1 || rec.Value[0] != "10 mail.example.com." {
		t.Errorf("Expected value [10 mail.example.com.], got %v", rec.Value)
	}
}

func TestForwarder_rrToRecords_NS(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.NS{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeNS,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Ns: "ns1.example.com.",
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeNS {
		t.Errorf("Expected type NS, got %s", rec.Type)
	}
	if len(rec.Value) != 1 || rec.Value[0] != "ns1.example.com." {
		t.Errorf("Expected value [ns1.example.com.], got %v", rec.Value)
	}
}

func TestForwarder_rrToRecords_Mixed(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rrs := []dns.RR{
		&dns.A{
			Hdr: dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeA, Ttl: 300},
			A:   net.ParseIP("1.2.3.4"),
		},
		&dns.AAAA{
			Hdr:  dns.RR_Header{Name: "example.com.", Rrtype: dns.TypeAAAA, Ttl: 300},
			AAAA: net.ParseIP("2001:db8::1"),
		},
		&dns.CNAME{
			Hdr:    dns.RR_Header{Name: "www.example.com.", Rrtype: dns.TypeCNAME, Ttl: 300},
			Target: "example.com.",
		},
	}

	records := f.rrToRecords(rrs)
	if len(records) != 3 {
		t.Fatalf("Expected 3 records, got %d", len(records))
	}

	// Verify types
	recordTypes := []types.RecordType{
		records[0].Type,
		records[1].Type,
		records[2].Type,
	}
	expectedTypes := []types.RecordType{
		types.RecordTypeA,
		types.RecordTypeAAAA,
		types.RecordTypeCNAME,
	}
	for i, rt := range recordTypes {
		if rt != expectedTypes[i] {
			t.Errorf("Record %d: expected type %s, got %s", i, expectedTypes[i], rt)
		}
	}
}

func TestForwarder_rrToRecords_UnsupportedType(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	// Create a mock RR with unsupported type
	rr := &dns.A{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: 999, // Unsupported type
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		A: net.ParseIP("1.2.3.4"),
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 0 {
		t.Errorf("Expected 0 records for unsupported type, got %d", len(records))
	}
}

func TestForwarder_rrToRecords_EmptyList(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	records := f.rrToRecords([]dns.RR{})
	if len(records) != 0 {
		t.Errorf("Expected 0 records for empty list, got %d", len(records))
	}
}

func TestForwarder_rrToRecords_SOA(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.SOA{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeSOA,
			Class:  dns.ClassINET,
			Ttl:    3600,
		},
		Ns:      "ns1.example.com.",
		Mbox:    "admin.example.com.",
		Serial:  2024010101,
		Refresh: 3600,
		Retry:   1800,
		Expire:  604800,
		Minttl:  86400,
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeSOA {
		t.Errorf("Expected type SOA, got %s", rec.Type)
	}
	expected := "ns1.example.com. admin.example.com. 2024010101 3600 1800 604800 86400"
	if len(rec.Value) != 1 || rec.Value[0] != expected {
		t.Errorf("Expected SOA value [%s], got %v", expected, rec.Value)
	}
}

func TestForwarder_rrToRecords_SRV(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.SRV{
		Hdr: dns.RR_Header{
			Name:   "_http._tcp.example.com.",
			Rrtype: dns.TypeSRV,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Priority: 10,
		Weight:   60,
		Port:     80,
		Target:   "server.example.com.",
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeSRV {
		t.Errorf("Expected type SRV, got %s", rec.Type)
	}
	expected := "10 60 80 server.example.com."
	if len(rec.Value) != 1 || rec.Value[0] != expected {
		t.Errorf("Expected SRV value [%s], got %v", expected, rec.Value)
	}
}

func TestForwarder_rrToRecords_CAA(t *testing.T) {
	f := NewForwarder(DefaultForwarderConfig())

	rr := &dns.CAA{
		Hdr: dns.RR_Header{
			Name:   "example.com.",
			Rrtype: dns.TypeCAA,
			Class:  dns.ClassINET,
			Ttl:    300,
		},
		Flag:  0,
		Tag:   "issue",
		Value: "letsencrypt.org",
	}

	records := f.rrToRecords([]dns.RR{rr})
	if len(records) != 1 {
		t.Fatalf("Expected 1 record, got %d", len(records))
	}

	rec := records[0]
	if rec.Type != types.RecordTypeCAA {
		t.Errorf("Expected type CAA, got %s", rec.Type)
	}
	expected := "0 issue letsencrypt.org"
	if len(rec.Value) != 1 || rec.Value[0] != expected {
		t.Errorf("Expected CAA value [%s], got %v", expected, rec.Value)
	}
}
