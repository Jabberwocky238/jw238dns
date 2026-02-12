// Package types defines the core DNS record types and sentinel errors
// used throughout the jw238dns module.
package types

import "errors"

// RecordType represents a DNS record type.
type RecordType string

const (
	RecordTypeA     RecordType = "A"
	RecordTypeAAAA  RecordType = "AAAA"
	RecordTypeCNAME RecordType = "CNAME"
	RecordTypeMX    RecordType = "MX"
	RecordTypeTXT   RecordType = "TXT"
	RecordTypeNS    RecordType = "NS"
	RecordTypeSRV   RecordType = "SRV"
	RecordTypePTR   RecordType = "PTR"
	RecordTypeSOA   RecordType = "SOA"
	RecordTypeCAA   RecordType = "CAA"
)

// validRecordTypes is the set of all supported record types.
var validRecordTypes = map[RecordType]bool{
	RecordTypeA:     true,
	RecordTypeAAAA:  true,
	RecordTypeCNAME: true,
	RecordTypeMX:    true,
	RecordTypeTXT:   true,
	RecordTypeNS:    true,
	RecordTypeSRV:   true,
	RecordTypePTR:   true,
	RecordTypeSOA:   true,
	RecordTypeCAA:   true,
}

// IsValid reports whether the RecordType is a supported DNS record type.
func (rt RecordType) IsValid() bool {
	return validRecordTypes[rt]
}

// DNSRecord represents a single DNS record entry.
type DNSRecord struct {
	Name  string     `json:"name" yaml:"name"`   // FQDN (e.g., "example.com.")
	Type  RecordType `json:"type" yaml:"type"`   // Record type (A, AAAA, CNAME, etc.)
	TTL   uint32     `json:"ttl" yaml:"ttl"`     // Time to live in seconds
	Value []string   `json:"value" yaml:"value"` // Record values (can be multiple)
}

// QueryInfo holds parsed information from a DNS query.
type QueryInfo struct {
	Domain string // FQDN being queried
	Type   uint16 // DNS query type (dns.TypeA, dns.TypeAAAA, etc.)
	Class  uint16 // DNS class (usually dns.ClassINET)
}

// RecordKey uniquely identifies a DNS record by name and type.
type RecordKey struct {
	Name string
	Type RecordType
}

// RecordChanges describes a set of changes to apply during a partial reload.
type RecordChanges struct {
	Added   []*DNSRecord
	Updated []*DNSRecord
	Deleted []RecordKey
}

// EventType describes the kind of storage event.
type EventType string

const (
	EventAdded    EventType = "added"
	EventUpdated  EventType = "updated"
	EventDeleted  EventType = "deleted"
	EventReloaded EventType = "reloaded"
)

// StorageEvent represents a change notification from storage.
type StorageEvent struct {
	Type   EventType
	Record *DNSRecord
}

// Sentinel errors for DNS operations.
var (
	ErrRecordNotFound    = errors.New("DNS record not found")
	ErrRecordExists      = errors.New("DNS record already exists")
	ErrInvalidRecordType = errors.New("invalid DNS record type")
	ErrInvalidTTL        = errors.New("TTL must be between 60 and 86400")
	ErrInvalidName       = errors.New("invalid domain name")
	ErrReloadFailed      = errors.New("hot reload failed")
	ErrStorageLocked     = errors.New("storage is locked during update")
)
