package types

import (
	"testing"
)

func TestRecordType_IsValid(t *testing.T) {
	tests := []struct {
		name     string
		rt       RecordType
		expected bool
	}{
		{name: "A record", rt: RecordTypeA, expected: true},
		{name: "AAAA record", rt: RecordTypeAAAA, expected: true},
		{name: "CNAME record", rt: RecordTypeCNAME, expected: true},
		{name: "MX record", rt: RecordTypeMX, expected: true},
		{name: "TXT record", rt: RecordTypeTXT, expected: true},
		{name: "NS record", rt: RecordTypeNS, expected: true},
		{name: "SRV record", rt: RecordTypeSRV, expected: true},
		{name: "PTR record", rt: RecordTypePTR, expected: true},
		{name: "SOA record", rt: RecordTypeSOA, expected: true},
		{name: "CAA record", rt: RecordTypeCAA, expected: true},
		{name: "empty string", rt: RecordType(""), expected: false},
		{name: "invalid type", rt: RecordType("INVALID"), expected: false},
		{name: "lowercase a", rt: RecordType("a"), expected: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.rt.IsValid()
			if got != tt.expected {
				t.Errorf("RecordType(%q).IsValid() = %v, want %v", tt.rt, got, tt.expected)
			}
		})
	}
}

func TestDNSRecord_Fields(t *testing.T) {
	record := &DNSRecord{
		Name:  "example.com.",
		Type:  RecordTypeA,
		TTL:   300,
		Value: []string{"192.168.1.1"},
	}

	if record.Name != "example.com." {
		t.Errorf("Name = %q, want %q", record.Name, "example.com.")
	}
	if record.Type != RecordTypeA {
		t.Errorf("Type = %q, want %q", record.Type, RecordTypeA)
	}
	if record.TTL != 300 {
		t.Errorf("TTL = %d, want %d", record.TTL, 300)
	}
	if len(record.Value) != 1 || record.Value[0] != "192.168.1.1" {
		t.Errorf("Value = %v, want [192.168.1.1]", record.Value)
	}
}

func TestRecordKey_Equality(t *testing.T) {
	k1 := RecordKey{Name: "example.com.", Type: RecordTypeA}
	k2 := RecordKey{Name: "example.com.", Type: RecordTypeA}
	k3 := RecordKey{Name: "example.com.", Type: RecordTypeAAAA}

	if k1 != k2 {
		t.Error("identical RecordKeys should be equal")
	}
	if k1 == k3 {
		t.Error("RecordKeys with different types should not be equal")
	}
}

func TestEventType_Constants(t *testing.T) {
	tests := []struct {
		name     string
		et       EventType
		expected string
	}{
		{name: "added", et: EventAdded, expected: "added"},
		{name: "updated", et: EventUpdated, expected: "updated"},
		{name: "deleted", et: EventDeleted, expected: "deleted"},
		{name: "reloaded", et: EventReloaded, expected: "reloaded"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if string(tt.et) != tt.expected {
				t.Errorf("EventType = %q, want %q", tt.et, tt.expected)
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		msg  string
	}{
		{name: "ErrRecordNotFound", err: ErrRecordNotFound, msg: "DNS record not found"},
		{name: "ErrRecordExists", err: ErrRecordExists, msg: "DNS record already exists"},
		{name: "ErrInvalidRecordType", err: ErrInvalidRecordType, msg: "invalid DNS record type"},
		{name: "ErrInvalidTTL", err: ErrInvalidTTL, msg: "TTL must be between 60 and 86400"},
		{name: "ErrInvalidName", err: ErrInvalidName, msg: "invalid domain name"},
		{name: "ErrReloadFailed", err: ErrReloadFailed, msg: "hot reload failed"},
		{name: "ErrStorageLocked", err: ErrStorageLocked, msg: "storage is locked during update"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Fatal("error should not be nil")
			}
			if tt.err.Error() != tt.msg {
				t.Errorf("error message = %q, want %q", tt.err.Error(), tt.msg)
			}
		})
	}
}
