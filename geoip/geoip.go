// Package geoip provides GeoIP-based distance calculation and DNS record
// sorting using MaxMind MMDB databases.
package geoip

import (
	"math"
	"net"
	"sort"

	"github.com/oschwald/geoip2-golang"

	"jabberwocky238/jw238dns/types"
)

// earthRadiusKm is the mean radius of the Earth in kilometers.
const earthRadiusKm = 6371.0

// Coordinates represents a geographic location.
type Coordinates struct {
	Latitude  float64
	Longitude float64
}

// IPLookup defines the interface for looking up IP coordinates.
type IPLookup interface {
	Lookup(ip net.IP) (*Coordinates, error)
}

// Reader wraps a MaxMind MMDB database for IP geolocation lookups.
// It implements the IPLookup interface.
type Reader struct {
	db *geoip2.Reader
}

// NewReader opens the MMDB file at the given path and returns a Reader.
// The caller must call Close when finished.
func NewReader(mmdbPath string) (*Reader, error) {
	db, err := geoip2.Open(mmdbPath)
	if err != nil {
		return nil, err
	}
	return &Reader{db: db}, nil
}

// Close releases the underlying MMDB database resources.
func (r *Reader) Close() error {
	if r.db != nil {
		return r.db.Close()
	}
	return nil
}

// Lookup returns the geographic coordinates for the given IP address.
// Returns nil coordinates if the IP cannot be found in the database.
func (r *Reader) Lookup(ip net.IP) (*Coordinates, error) {
	city, err := r.db.City(ip)
	if err != nil {
		return nil, err
	}
	return &Coordinates{
		Latitude:  city.Location.Latitude,
		Longitude: city.Location.Longitude,
	}, nil
}

// Haversine calculates the great-circle distance in kilometers between
// two geographic coordinates using the Haversine formula.
func Haversine(a, b Coordinates) float64 {
	lat1 := degreesToRadians(a.Latitude)
	lat2 := degreesToRadians(b.Latitude)
	dLat := degreesToRadians(b.Latitude - a.Latitude)
	dLon := degreesToRadians(b.Longitude - a.Longitude)

	h := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(lat1)*math.Cos(lat2)*
			math.Sin(dLon/2)*math.Sin(dLon/2)

	c := 2 * math.Atan2(math.Sqrt(h), math.Sqrt(1-h))
	return earthRadiusKm * c
}

// degreesToRadians converts degrees to radians.
func degreesToRadians(deg float64) float64 {
	return deg * math.Pi / 180.0
}

// SortRecordsByDistance sorts A and AAAA DNS records by geographic distance
// from the client coordinates. Records of other types are left unchanged.
// Each DNSRecord may contain multiple values; this function reorders the
// Value slice so that the closest IP comes first.
func SortRecordsByDistance(records []*types.DNSRecord, client Coordinates, lookup IPLookup) {
	for _, rec := range records {
		if rec.Type != types.RecordTypeA && rec.Type != types.RecordTypeAAAA {
			continue
		}
		if len(rec.Value) <= 1 {
			continue
		}
		sortValuesByDistance(rec, client, lookup)
	}
}

// sortValuesByDistance reorders the Value slice of a single record by
// distance from the client.
func sortValuesByDistance(rec *types.DNSRecord, client Coordinates, lookup IPLookup) {
	type ipDist struct {
		value string
		dist  float64
	}

	entries := make([]ipDist, 0, len(rec.Value))
	for _, v := range rec.Value {
		ip := net.ParseIP(v)
		if ip == nil {
			// Keep unparseable values with max distance so they sort last.
			entries = append(entries, ipDist{value: v, dist: math.MaxFloat64})
			continue
		}
		coords, err := lookup.Lookup(ip)
		if err != nil || coords == nil {
			entries = append(entries, ipDist{value: v, dist: math.MaxFloat64})
			continue
		}
		entries = append(entries, ipDist{value: v, dist: Haversine(client, *coords)})
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].dist < entries[j].dist
	})

	for i, e := range entries {
		rec.Value[i] = e.value
	}
}
