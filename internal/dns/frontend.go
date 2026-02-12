// Package dns implements the DNS frontend (query receiver) and backend
// (response generator) for the jw238dns module.
package dns

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"jabberwocky238/jw238dns/internal/types"

	"github.com/miekg/dns"
)

// DNSFrontend receives and parses DNS queries.
type DNSFrontend interface {
	// ReceiveQuery accepts a DNS query and returns a response.
	ReceiveQuery(ctx context.Context, query *dns.Msg) (*dns.Msg, error)

	// ParseQuery validates and extracts query information from a DNS message.
	ParseQuery(query *dns.Msg) (*types.QueryInfo, error)
}

// Frontend implements DNSFrontend by parsing incoming queries and delegating
// resolution to a DNSBackend.
type Frontend struct {
	backend DNSBackend
}

// NewFrontend creates a Frontend that delegates resolution to the given backend.
func NewFrontend(backend DNSBackend) *Frontend {
	return &Frontend{backend: backend}
}

// ReceiveQuery parses the incoming DNS message, resolves it via the backend,
// and builds a wire-format response.
func (f *Frontend) ReceiveQuery(ctx context.Context, query *dns.Msg) (*dns.Msg, error) {
	info, err := f.ParseQuery(query)
	if err != nil {
		resp := new(dns.Msg)
		resp.SetRcode(query, dns.RcodeFormatError)
		return resp, err
	}

	slog.Debug("dns query received",
		"domain", info.Domain,
		"type", dns.TypeToString[info.Type],
		"class", info.Class,
	)

	records, err := f.backend.Resolve(ctx, info)

	resp := new(dns.Msg)
	resp.SetReply(query)
	resp.Authoritative = true

	if err != nil {
		if err == types.ErrRecordNotFound {
			resp.SetRcode(query, dns.RcodeNameError)
			// Attach SOA in authority section if backend provides one.
			soaRecords, soaErr := f.backend.Resolve(ctx, &types.QueryInfo{
				Domain: info.Domain,
				Type:   dns.TypeSOA,
				Class:  dns.ClassINET,
			})
			if soaErr == nil {
				for _, r := range soaRecords {
					rr := buildRR(r)
					if rr != nil {
						resp.Ns = append(resp.Ns, rr)
					}
				}
			}
			return resp, nil
		}
		resp.SetRcode(query, dns.RcodeServerFailure)
		return resp, err
	}

	for _, r := range records {
		rr := buildRR(r)
		if rr != nil {
			resp.Answer = append(resp.Answer, rr)
		}
	}

	return resp, nil
}

// ParseQuery validates a DNS message and extracts the query information.
// It returns an error if the message has no questions or the domain name
// is empty.
func (f *Frontend) ParseQuery(query *dns.Msg) (*types.QueryInfo, error) {
	if query == nil {
		return nil, fmt.Errorf("nil query message")
	}
	if len(query.Question) == 0 {
		return nil, fmt.Errorf("query has no questions")
	}

	q := query.Question[0]
	domain := q.Name
	if domain == "" {
		return nil, types.ErrInvalidName
	}

	// Ensure FQDN trailing dot.
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}

	return &types.QueryInfo{
		Domain: domain,
		Type:   q.Qtype,
		Class:  q.Qclass,
	}, nil
}

// buildRR converts a DNSRecord into a dns.RR suitable for a response message.
// Returns nil if the record type is unsupported or the value is empty.
func buildRR(record *types.DNSRecord) dns.RR {
	if len(record.Value) == 0 {
		return nil
	}

	hdr := dns.RR_Header{
		Name:   record.Name,
		Rrtype: recordTypeToUint16(record.Type),
		Class:  dns.ClassINET,
		Ttl:    record.TTL,
	}

	val := record.Value[0]

	switch record.Type {
	case types.RecordTypeA:
		return &dns.A{Hdr: hdr, A: parseIP(val)}
	case types.RecordTypeAAAA:
		return &dns.AAAA{Hdr: hdr, AAAA: parseIP(val)}
	case types.RecordTypeCNAME:
		return &dns.CNAME{Hdr: hdr, Target: val}
	case types.RecordTypeMX:
		return &dns.MX{Hdr: hdr, Preference: 10, Mx: val}
	case types.RecordTypeTXT:
		return &dns.TXT{Hdr: hdr, Txt: record.Value}
	case types.RecordTypeNS:
		return &dns.NS{Hdr: hdr, Ns: val}
	case types.RecordTypePTR:
		return &dns.PTR{Hdr: hdr, Ptr: val}
	case types.RecordTypeSOA:
		return buildSOA(hdr, record.Value)
	case types.RecordTypeCAA:
		return &dns.CAA{Hdr: hdr, Flag: 0, Tag: "issue", Value: val}
	case types.RecordTypeSRV:
		return buildSRV(hdr, record.Value)
	default:
		return nil
	}
}

// recordTypeToUint16 maps a RecordType to the miekg/dns type constant.
func recordTypeToUint16(rt types.RecordType) uint16 {
	switch rt {
	case types.RecordTypeA:
		return dns.TypeA
	case types.RecordTypeAAAA:
		return dns.TypeAAAA
	case types.RecordTypeCNAME:
		return dns.TypeCNAME
	case types.RecordTypeMX:
		return dns.TypeMX
	case types.RecordTypeTXT:
		return dns.TypeTXT
	case types.RecordTypeNS:
		return dns.TypeNS
	case types.RecordTypeSRV:
		return dns.TypeSRV
	case types.RecordTypePTR:
		return dns.TypePTR
	case types.RecordTypeSOA:
		return dns.TypeSOA
	case types.RecordTypeCAA:
		return dns.TypeCAA
	default:
		return 0
	}
}

// uint16ToRecordType maps a miekg/dns type constant to a RecordType.
func uint16ToRecordType(qtype uint16) types.RecordType {
	switch qtype {
	case dns.TypeA:
		return types.RecordTypeA
	case dns.TypeAAAA:
		return types.RecordTypeAAAA
	case dns.TypeCNAME:
		return types.RecordTypeCNAME
	case dns.TypeMX:
		return types.RecordTypeMX
	case dns.TypeTXT:
		return types.RecordTypeTXT
	case dns.TypeNS:
		return types.RecordTypeNS
	case dns.TypeSRV:
		return types.RecordTypeSRV
	case dns.TypePTR:
		return types.RecordTypePTR
	case dns.TypeSOA:
		return types.RecordTypeSOA
	case dns.TypeCAA:
		return types.RecordTypeCAA
	default:
		return types.RecordType("")
	}
}
