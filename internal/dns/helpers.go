package dns

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/miekg/dns"

	"jabberwocky238/jw238dns/internal/types"
)

// parseIP parses an IP address string into a net.IP.
func parseIP(s string) net.IP {
	return net.ParseIP(s)
}

// buildSOA constructs a dns.SOA RR from the record values.
// Expected value format: ["ns.example.com. admin.example.com. 1 3600 900 604800 86400"]
// or individual fields as separate slice elements.
func buildSOA(hdr dns.RR_Header, values []string) *dns.SOA {
	soa := &dns.SOA{
		Hdr:     hdr,
		Ns:      "ns1.example.com.",
		Mbox:    "admin.example.com.",
		Serial:  1,
		Refresh: 3600,
		Retry:   900,
		Expire:  604800,
		Minttl:  86400,
	}

	if len(values) == 0 {
		return soa
	}

	// Try parsing space-separated fields from the first value.
	parts := strings.Fields(values[0])
	if len(parts) >= 7 {
		soa.Ns = parts[0]
		soa.Mbox = parts[1]
		if v, err := strconv.ParseUint(parts[2], 10, 32); err == nil {
			soa.Serial = uint32(v)
		}
		if v, err := strconv.ParseUint(parts[3], 10, 32); err == nil {
			soa.Refresh = uint32(v)
		}
		if v, err := strconv.ParseUint(parts[4], 10, 32); err == nil {
			soa.Retry = uint32(v)
		}
		if v, err := strconv.ParseUint(parts[5], 10, 32); err == nil {
			soa.Expire = uint32(v)
		}
		if v, err := strconv.ParseUint(parts[6], 10, 32); err == nil {
			soa.Minttl = uint32(v)
		}
	} else if len(parts) >= 2 {
		soa.Ns = parts[0]
		soa.Mbox = parts[1]
	}

	return soa
}

// buildSRV constructs a dns.SRV RR from the record values.
// Expected value format: ["priority weight port target"]
// e.g. ["10 60 5060 sip.example.com."]
func buildSRV(hdr dns.RR_Header, values []string) *dns.SRV {
	srv := &dns.SRV{Hdr: hdr}

	if len(values) == 0 {
		return srv
	}

	parts := strings.Fields(values[0])
	if len(parts) >= 4 {
		if v, err := strconv.ParseUint(parts[0], 10, 16); err == nil {
			srv.Priority = uint16(v)
		}
		if v, err := strconv.ParseUint(parts[1], 10, 16); err == nil {
			srv.Weight = uint16(v)
		}
		if v, err := strconv.ParseUint(parts[2], 10, 16); err == nil {
			srv.Port = uint16(v)
		}
		srv.Target = parts[3]
	} else if len(parts) >= 1 {
		srv.Target = parts[0]
	}

	return srv
}

// soaForZone returns a default SOA record for the given domain, used when
// returning NXDOMAIN responses.
func soaForZone(domain string) *types.DNSRecord {
	zone := extractZone(domain)
	return &types.DNSRecord{
		Name: zone,
		Type: types.RecordTypeSOA,
		TTL:  300,
		Value: []string{
			fmt.Sprintf("ns1.%s admin.%s 1 3600 900 604800 86400", zone, zone),
		},
	}
}

// extractZone returns the top two labels of a domain as the zone.
// For "sub.example.com." it returns "example.com."
func extractZone(domain string) string {
	domain = strings.TrimSuffix(domain, ".")
	parts := strings.Split(domain, ".")
	if len(parts) <= 2 {
		return domain + "."
	}
	return strings.Join(parts[len(parts)-2:], ".") + "."
}
