package dns

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"jabberwocky238/jw238dns/types"

	"github.com/miekg/dns"
)

// ForwarderConfig holds configuration for upstream DNS forwarding.
type ForwarderConfig struct {
	Enabled bool          // Enable upstream forwarding
	Servers []string      // Upstream DNS server addresses (e.g. "1.1.1.1:53")
	Timeout time.Duration // Timeout for upstream queries
}

// DefaultForwarderConfig returns a ForwarderConfig with sensible defaults.
func DefaultForwarderConfig() ForwarderConfig {
	return ForwarderConfig{
		Enabled: false,
		Servers: []string{"1.1.1.1:53"},
		Timeout: 5 * time.Second,
	}
}

// Forwarder handles forwarding DNS queries to upstream servers.
type Forwarder struct {
	config ForwarderConfig
	client *dns.Client
}

// NewForwarder creates a new Forwarder with the given configuration.
func NewForwarder(cfg ForwarderConfig) *Forwarder {
	return &Forwarder{
		config: cfg,
		client: &dns.Client{
			Net:     "udp",
			Timeout: cfg.Timeout,
		},
	}
}

// Forward queries upstream DNS servers for the given domain and query type.
// It tries each configured server in order. On timeout or network error it
// falls through to the next server. On authoritative failures (NXDOMAIN,
// SERVFAIL) it returns immediately without retrying.
func (f *Forwarder) Forward(ctx context.Context, domain string, qtype uint16) ([]*types.DNSRecord, error) {
	if !f.config.Enabled {
		return nil, types.ErrRecordNotFound
	}

	if len(f.config.Servers) == 0 {
		return nil, types.ErrRecordNotFound
	}

	query := new(dns.Msg)
	query.SetQuestion(domain, qtype)
	query.RecursionDesired = true

	for _, server := range f.config.Servers {
		resp, _, err := f.client.ExchangeContext(ctx, query, server)
		if err != nil {
			slog.Debug("upstream query failed, trying next server",
				"server", server,
				"domain", domain,
				"error", err,
			)
			continue
		}

		// Authoritative negative responses are final; don't retry.
		if resp.Rcode == dns.RcodeNameError || resp.Rcode == dns.RcodeServerFailure {
			slog.Debug("upstream returned negative response",
				"server", server,
				"domain", domain,
				"rcode", dns.RcodeToString[resp.Rcode],
			)
			return nil, fmt.Errorf("upstream %s: %s", server, dns.RcodeToString[resp.Rcode])
		}

		records := f.rrToRecords(resp.Answer)
		if len(records) > 0 {
			slog.Debug("upstream query succeeded",
				"server", server,
				"domain", domain,
				"records", len(records),
			)
			return records, nil
		}
	}

	return nil, types.ErrRecordNotFound
}

// rrToRecords converts a slice of dns.RR answer records into DNSRecord
// structs. Unsupported RR types are silently skipped.
func (f *Forwarder) rrToRecords(rrs []dns.RR) []*types.DNSRecord {
	var records []*types.DNSRecord
	for _, rr := range rrs {
		hdr := rr.Header()
		rt := uint16ToRecordType(hdr.Rrtype)
		if rt == "" {
			continue
		}

		rec := &types.DNSRecord{
			Name: hdr.Name,
			Type: rt,
			TTL:  hdr.Ttl,
		}

		switch v := rr.(type) {
		case *dns.A:
			rec.Value = []string{v.A.String()}
		case *dns.AAAA:
			rec.Value = []string{v.AAAA.String()}
		case *dns.CNAME:
			rec.Value = []string{v.Target}
		case *dns.MX:
			rec.Value = []string{fmt.Sprintf("%d %s", v.Preference, v.Mx)}
		case *dns.TXT:
			rec.Value = v.Txt
		case *dns.NS:
			rec.Value = []string{v.Ns}
		case *dns.PTR:
			rec.Value = []string{v.Ptr}
		case *dns.SOA:
			rec.Value = []string{fmt.Sprintf("%s %s %d %d %d %d %d",
				v.Ns, v.Mbox, v.Serial, v.Refresh, v.Retry, v.Expire, v.Minttl)}
		case *dns.SRV:
			rec.Value = []string{fmt.Sprintf("%d %d %d %s",
				v.Priority, v.Weight, v.Port, v.Target)}
		case *dns.CAA:
			rec.Value = []string{fmt.Sprintf("%d %s %s", v.Flag, v.Tag, v.Value)}
		default:
			continue
		}

		records = append(records, rec)
	}
	return records
}
