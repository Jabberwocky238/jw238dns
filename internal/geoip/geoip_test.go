package geoip

import (
	"fmt"
	"math"
	"net"
	"testing"

	"jabberwocky238/jw238dns/internal/types"
)

func TestHaversine_KnownDistances(t *testing.T) {
	tests := []struct {
		name      string
		a         Coordinates
		b         Coordinates
		wantKm    float64
		tolerance float64
	}{
		{
			name:      "same point",
			a:         Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			b:         Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			wantKm:    0,
			tolerance: 0.01,
		},
		{
			name:      "New York to London",
			a:         Coordinates{Latitude: 40.7128, Longitude: -74.0060},
			b:         Coordinates{Latitude: 51.5074, Longitude: -0.1278},
			wantKm:    5570,
			tolerance: 20,
		},
		{
			name:      "Tokyo to Sydney",
			a:         Coordinates{Latitude: 35.6762, Longitude: 139.6503},
			b:         Coordinates{Latitude: -33.8688, Longitude: 151.2093},
			wantKm:    7826,
			tolerance: 30,
		},
		{
			name:      "North Pole to South Pole",
			a:         Coordinates{Latitude: 90, Longitude: 0},
			b:         Coordinates{Latitude: -90, Longitude: 0},
			wantKm:    20015,
			tolerance: 20,
		},
		{
			name:      "equator antipodal points",
			a:         Coordinates{Latitude: 0, Longitude: 0},
			b:         Coordinates{Latitude: 0, Longitude: 180},
			wantKm:    20015,
			tolerance: 20,
		},
		{
			name:      "San Francisco to Tokyo",
			a:         Coordinates{Latitude: 37.7749, Longitude: -122.4194},
			b:         Coordinates{Latitude: 35.6762, Longitude: 139.6503},
			wantKm:    8280,
			tolerance: 30,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Haversine(tt.a, tt.b)
			if math.Abs(got-tt.wantKm) > tt.tolerance {
				t.Errorf("Haversine() = %.2f km, want ~%.2f km (tolerance %.2f)", got, tt.wantKm, tt.tolerance)
			}
		})
	}
}

func TestHaversine_Symmetry(t *testing.T) {
	a := Coordinates{Latitude: 48.8566, Longitude: 2.3522}  // Paris
	b := Coordinates{Latitude: 35.6762, Longitude: 139.6503} // Tokyo

	ab := Haversine(a, b)
	ba := Haversine(b, a)

	if math.Abs(ab-ba) > 0.001 {
		t.Errorf("Haversine not symmetric: a->b = %.4f, b->a = %.4f", ab, ba)
	}
}

func TestHaversine_NonNegative(t *testing.T) {
	coords := []Coordinates{
		{0, 0},
		{90, 0},
		{-90, 0},
		{0, 180},
		{45.0, -93.0},
	}

	for i, a := range coords {
		for j, b := range coords {
			d := Haversine(a, b)
			if d < 0 {
				t.Errorf("Haversine(coords[%d], coords[%d]) = %.4f, want >= 0", i, j, d)
			}
		}
	}
}

func TestHaversine_ZeroDistance(t *testing.T) {
	c := Coordinates{Latitude: 51.5074, Longitude: -0.1278}
	d := Haversine(c, c)
	if d != 0 {
		t.Errorf("distance from point to itself should be 0, got %f", d)
	}
}

func TestDegreesToRadians(t *testing.T) {
	tests := []struct {
		deg  float64
		want float64
	}{
		{0, 0},
		{90, math.Pi / 2},
		{180, math.Pi},
		{360, 2 * math.Pi},
		{-90, -math.Pi / 2},
	}

	for _, tt := range tests {
		got := degreesToRadians(tt.deg)
		if math.Abs(got-tt.want) > 1e-10 {
			t.Errorf("degreesToRadians(%.1f) = %f, want %f", tt.deg, got, tt.want)
		}
	}
}

func TestEarthRadiusKm(t *testing.T) {
	if earthRadiusKm != 6371.0 {
		t.Errorf("earthRadiusKm = %f, want 6371.0", earthRadiusKm)
	}
}

// mockLookup implements IPLookup for testing without an MMDB file.
type mockLookup struct {
	coords map[string]*Coordinates
}

func (m *mockLookup) Lookup(ip net.IP) (*Coordinates, error) {
	c, ok := m.coords[ip.String()]
	if !ok {
		return nil, fmt.Errorf("IP not found: %s", ip)
	}
	return c, nil
}

func newMockLookup() *mockLookup {
	return &mockLookup{
		coords: map[string]*Coordinates{
			// Toronto
			"10.0.0.1": {Latitude: 43.6532, Longitude: -79.3832},
			// London
			"10.0.0.2": {Latitude: 51.5074, Longitude: -0.1278},
			// Tokyo
			"10.0.0.3": {Latitude: 35.6762, Longitude: 139.6503},
			// Sydney
			"10.0.0.4": {Latitude: -33.8688, Longitude: 151.2093},
			// IPv6 Berlin
			"2001:db8::1": {Latitude: 52.5200, Longitude: 13.4050},
			// IPv6 São Paulo
			"2001:db8::2": {Latitude: -23.5505, Longitude: -46.6333},
		},
	}
}

func TestSortRecordsByDistance_ARecords(t *testing.T) {
	lookup := newMockLookup()
	// Client is in New York
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	rec := &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"10.0.0.3", "10.0.0.2", "10.0.0.1"}, // Tokyo, London, Toronto
	}

	SortRecordsByDistance([]*types.DNSRecord{rec}, client, lookup)

	// Expected order: Toronto (closest), London, Tokyo (farthest)
	if rec.Value[0] != "10.0.0.1" {
		t.Errorf("closest should be Toronto (10.0.0.1), got %s", rec.Value[0])
	}
	if rec.Value[1] != "10.0.0.2" {
		t.Errorf("middle should be London (10.0.0.2), got %s", rec.Value[1])
	}
	if rec.Value[2] != "10.0.0.3" {
		t.Errorf("farthest should be Tokyo (10.0.0.3), got %s", rec.Value[2])
	}
}

func TestSortRecordsByDistance_AAAARecords(t *testing.T) {
	lookup := newMockLookup()
	// Client is in London
	client := Coordinates{Latitude: 51.5074, Longitude: -0.1278}

	rec := &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeAAAA,
		TTL:   300,
		Value: []string{"2001:db8::2", "2001:db8::1"}, // São Paulo, Berlin
	}

	SortRecordsByDistance([]*types.DNSRecord{rec}, client, lookup)

	// Berlin is closer to London than São Paulo
	if rec.Value[0] != "2001:db8::1" {
		t.Errorf("closest should be Berlin (2001:db8::1), got %s", rec.Value[0])
	}
	if rec.Value[1] != "2001:db8::2" {
		t.Errorf("farthest should be São Paulo (2001:db8::2), got %s", rec.Value[1])
	}
}

func TestSortRecordsByDistance_SkipsNonIPTypes(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	records := []*types.DNSRecord{
		{Name: "example.com.", Type: types.RecordTypeTXT, TTL: 300, Value: []string{"z-value", "a-value"}},
		{Name: "example.com.", Type: types.RecordTypeMX, TTL: 300, Value: []string{"mail2.example.com.", "mail1.example.com."}},
		{Name: "example.com.", Type: types.RecordTypeCNAME, TTL: 300, Value: []string{"other.com.", "another.com."}},
	}

	// Save original order
	origTXT := make([]string, len(records[0].Value))
	copy(origTXT, records[0].Value)
	origMX := make([]string, len(records[1].Value))
	copy(origMX, records[1].Value)

	SortRecordsByDistance(records, client, lookup)

	for i, v := range records[0].Value {
		if v != origTXT[i] {
			t.Errorf("TXT values should not change, got %v", records[0].Value)
		}
	}
	for i, v := range records[1].Value {
		if v != origMX[i] {
			t.Errorf("MX values should not change, got %v", records[1].Value)
		}
	}
}

func TestSortRecordsByDistance_SkipsSingleValue(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	rec := &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"10.0.0.1"},
	}

	SortRecordsByDistance([]*types.DNSRecord{rec}, client, lookup)

	if rec.Value[0] != "10.0.0.1" {
		t.Errorf("single-value record should be unchanged, got %v", rec.Value)
	}
}

func TestSortRecordsByDistance_UnknownIPsSortLast(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	rec := &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"192.168.99.99", "10.0.0.1"}, // unknown IP, Toronto
	}

	SortRecordsByDistance([]*types.DNSRecord{rec}, client, lookup)

	// Toronto should come first, unknown IP last
	if rec.Value[0] != "10.0.0.1" {
		t.Errorf("known IP should sort first, got %s", rec.Value[0])
	}
	if rec.Value[1] != "192.168.99.99" {
		t.Errorf("unknown IP should sort last, got %s", rec.Value[1])
	}
}

func TestSortRecordsByDistance_UnparseableIPsSortLast(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	rec := &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"not-an-ip", "10.0.0.1"}, // invalid, Toronto
	}

	SortRecordsByDistance([]*types.DNSRecord{rec}, client, lookup)

	if rec.Value[0] != "10.0.0.1" {
		t.Errorf("valid IP should sort first, got %s", rec.Value[0])
	}
	if rec.Value[1] != "not-an-ip" {
		t.Errorf("unparseable value should sort last, got %s", rec.Value[1])
	}
}

func TestSortRecordsByDistance_MixedRecordTypes(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	records := []*types.DNSRecord{
		{Name: "example.com.", Type: types.RecordTypeA, TTL: 300, Value: []string{"10.0.0.3", "10.0.0.1"}},
		{Name: "example.com.", Type: types.RecordTypeTXT, TTL: 300, Value: []string{"some-text"}},
		{Name: "example.com.", Type: types.RecordTypeAAAA, TTL: 300, Value: []string{"2001:db8::2", "2001:db8::1"}},
	}

	SortRecordsByDistance(records, client, lookup)

	// A record: Toronto closer than Tokyo
	if records[0].Value[0] != "10.0.0.1" {
		t.Errorf("A record: closest should be 10.0.0.1, got %s", records[0].Value[0])
	}
	// TXT unchanged
	if records[1].Value[0] != "some-text" {
		t.Errorf("TXT should be unchanged")
	}
	// AAAA record: Berlin closer to NY than São Paulo
	if records[2].Value[0] != "2001:db8::1" {
		t.Errorf("AAAA record: closest should be 2001:db8::1, got %s", records[2].Value[0])
	}
}

func TestSortRecordsByDistance_AllUnknownIPs(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	rec := &types.DNSRecord{
		Name:  "example.com.",
		Type:  types.RecordTypeA,
		TTL:   300,
		Value: []string{"192.168.1.1", "192.168.1.2"},
	}

	// Should not panic; order preserved via stable sort
	SortRecordsByDistance([]*types.DNSRecord{rec}, client, lookup)

	if rec.Value[0] != "192.168.1.1" {
		t.Errorf("stable sort should preserve order for equal distances, got %v", rec.Value)
	}
}

func TestSortRecordsByDistance_EmptyRecords(t *testing.T) {
	lookup := newMockLookup()
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	// Should not panic on empty slice
	SortRecordsByDistance([]*types.DNSRecord{}, client, lookup)
	SortRecordsByDistance(nil, client, lookup)
}

func TestNewReader_InvalidPath(t *testing.T) {
	_, err := NewReader("/nonexistent/path/to/file.mmdb")
	if err == nil {
		t.Error("NewReader() with invalid path should return error")
	}
}

func TestReader_Close_NilDB(t *testing.T) {
	r := &Reader{db: nil}
	err := r.Close()
	if err != nil {
		t.Errorf("Close() on nil db should return nil, got %v", err)
	}
}

func TestSortValuesByDistance_Ordering(t *testing.T) {
	// Verify the Haversine-based ordering is correct for known cities.
	client := Coordinates{Latitude: 40.7128, Longitude: -74.0060} // New York

	london := Coordinates{Latitude: 51.5074, Longitude: -0.1278}
	tokyo := Coordinates{Latitude: 35.6762, Longitude: 139.6503}
	toronto := Coordinates{Latitude: 43.6532, Longitude: -79.3832}

	dToronto := Haversine(client, toronto)
	dLondon := Haversine(client, london)
	dTokyo := Haversine(client, tokyo)

	if dToronto >= dLondon {
		t.Errorf("Toronto (%.0f km) should be closer than London (%.0f km)", dToronto, dLondon)
	}
	if dLondon >= dTokyo {
		t.Errorf("London (%.0f km) should be closer than Tokyo (%.0f km)", dLondon, dTokyo)
	}
}

// TestRealWorldScenario_ThreeDatacenters tests a real-world scenario with three datacenters:
// Hong Kong (101.32.181.225), Silicon Valley (170.106.143.75), New York (163.245.215.17)
// Simulates access from each location and verifies correct distance-based sorting.
func TestRealWorldScenario_ThreeDatacenters(t *testing.T) {
	// Real datacenter locations (approximate coordinates)
	hongKong := Coordinates{Latitude: 22.3193, Longitude: 114.1694}
	siliconValley := Coordinates{Latitude: 37.3861, Longitude: -122.0839}
	newYork := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	// Create mock lookup with the three datacenter IPs
	lookup := &mockLookup{
		coords: map[string]*Coordinates{
			"101.32.181.225":  &hongKong,      // Hong Kong
			"170.106.143.75":  &siliconValley, // Silicon Valley
			"163.245.215.17":  &newYork,       // New York
		},
	}

	tests := []struct {
		name           string
		clientLocation Coordinates
		clientCity     string
		expectedOrder  []string
		distances      []float64 // Expected approximate distances in km
	}{
		{
			name:           "access from Hong Kong",
			clientLocation: hongKong,
			clientCity:     "Hong Kong",
			expectedOrder:  []string{"101.32.181.225", "170.106.143.75", "163.245.215.17"},
			distances:      []float64{0, 11130, 12970}, // HK->HK: 0km, HK->SV: ~11130km, HK->NY: ~12970km
		},
		{
			name:           "access from Silicon Valley",
			clientLocation: siliconValley,
			clientCity:     "Silicon Valley",
			expectedOrder:  []string{"170.106.143.75", "163.245.215.17", "101.32.181.225"},
			distances:      []float64{0, 4130, 11130}, // SV->SV: 0km, SV->NY: ~4130km, SV->HK: ~11130km
		},
		{
			name:           "access from New York",
			clientLocation: newYork,
			clientCity:     "New York",
			expectedOrder:  []string{"163.245.215.17", "170.106.143.75", "101.32.181.225"},
			distances:      []float64{0, 4130, 12970}, // NY->NY: 0km, NY->SV: ~4130km, NY->HK: ~12970km
		},
		{
			name:           "access from London (equidistant test)",
			clientLocation: Coordinates{Latitude: 51.5074, Longitude: -0.1278},
			clientCity:     "London",
			expectedOrder:  []string{"163.245.215.17", "170.106.143.75", "101.32.181.225"},
			distances:      []float64{5570, 8637, 9623}, // London->NY: ~5570km, London->SV: ~8637km, London->HK: ~9623km
		},
		{
			name:           "access from Tokyo",
			clientLocation: Coordinates{Latitude: 35.6762, Longitude: 139.6503},
			clientCity:     "Tokyo",
			expectedOrder:  []string{"101.32.181.225", "170.106.143.75", "163.245.215.17"},
			distances:      []float64{2900, 8280, 10850}, // Tokyo->HK: ~2900km, Tokyo->SV: ~8280km, Tokyo->NY: ~10850km
		},
		{
			name:           "access from Sydney",
			clientLocation: Coordinates{Latitude: -33.8688, Longitude: 151.2093},
			clientCity:     "Sydney",
			expectedOrder:  []string{"101.32.181.225", "170.106.143.75", "163.245.215.17"},
			distances:      []float64{7380, 11950, 15990}, // Sydney->HK: ~7380km, Sydney->SV: ~11950km, Sydney->NY: ~15990km
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create DNS record with all three IPs in random order
			rec := &types.DNSRecord{
				Name:  "example.com.",
				Type:  types.RecordTypeA,
				TTL:   300,
				Value: []string{"170.106.143.75", "101.32.181.225", "163.245.215.17"}, // Random initial order
			}

			// Sort by distance from client location
			SortRecordsByDistance([]*types.DNSRecord{rec}, tt.clientLocation, lookup)

			// Verify the order matches expected
			for i, expectedIP := range tt.expectedOrder {
				if rec.Value[i] != expectedIP {
					t.Errorf("Position %d: expected %s, got %s", i, expectedIP, rec.Value[i])
				}
			}

			// Log distances for verification
			t.Logf("Client location: %s", tt.clientCity)
			for i, ip := range rec.Value {
				serverCoords, _ := lookup.Lookup(net.ParseIP(ip))
				distance := Haversine(tt.clientLocation, *serverCoords)
				t.Logf("  %d. %s (%.0f km)", i+1, ip, distance)

				// Verify distance is within reasonable tolerance of expected
				expectedDist := tt.distances[i]
				tolerance := 100.0 // 100km tolerance for approximate coordinates
				if math.Abs(distance-expectedDist) > tolerance {
					t.Logf("    Warning: distance %.0f km differs from expected %.0f km (tolerance %.0f km)",
						distance, expectedDist, tolerance)
				}
			}

			// Verify distances are in ascending order (closest first)
			for i := 0; i < len(rec.Value)-1; i++ {
				ip1 := net.ParseIP(rec.Value[i])
				ip2 := net.ParseIP(rec.Value[i+1])
				coords1, _ := lookup.Lookup(ip1)
				coords2, _ := lookup.Lookup(ip2)
				dist1 := Haversine(tt.clientLocation, *coords1)
				dist2 := Haversine(tt.clientLocation, *coords2)

				if dist1 > dist2 {
					t.Errorf("Distance ordering incorrect: IP %s (%.0f km) should be before IP %s (%.0f km)",
						rec.Value[i], dist1, rec.Value[i+1], dist2)
				}
			}
		})
	}
}

// TestRealWorldScenario_MultipleRecords tests sorting multiple A records simultaneously
func TestRealWorldScenario_MultipleRecords(t *testing.T) {
	hongKong := Coordinates{Latitude: 22.3193, Longitude: 114.1694}
	siliconValley := Coordinates{Latitude: 37.3861, Longitude: -122.0839}
	newYork := Coordinates{Latitude: 40.7128, Longitude: -74.0060}

	lookup := &mockLookup{
		coords: map[string]*Coordinates{
			"101.32.181.225": &hongKong,
			"170.106.143.75": &siliconValley,
			"163.245.215.17": &newYork,
		},
	}

	// Client in Hong Kong
	client := hongKong

	records := []*types.DNSRecord{
		{
			Name:  "api.example.com.",
			Type:  types.RecordTypeA,
			TTL:   300,
			Value: []string{"163.245.215.17", "170.106.143.75", "101.32.181.225"},
		},
		{
			Name:  "cdn.example.com.",
			Type:  types.RecordTypeA,
			TTL:   300,
			Value: []string{"170.106.143.75", "163.245.215.17", "101.32.181.225"},
		},
		{
			Name:  "example.com.",
			Type:  types.RecordTypeTXT,
			TTL:   300,
			Value: []string{"v=spf1 include:_spf.example.com ~all"},
		},
	}

	SortRecordsByDistance(records, client, lookup)

	// Both A records should be sorted with Hong Kong first
	for i := 0; i < 2; i++ {
		if records[i].Value[0] != "101.32.181.225" {
			t.Errorf("Record %d: expected Hong Kong IP first, got %s", i, records[i].Value[0])
		}
		if records[i].Value[1] != "170.106.143.75" {
			t.Errorf("Record %d: expected Silicon Valley IP second, got %s", i, records[i].Value[1])
		}
		if records[i].Value[2] != "163.245.215.17" {
			t.Errorf("Record %d: expected New York IP third, got %s", i, records[i].Value[2])
		}
	}

	// TXT record should be unchanged
	if records[2].Value[0] != "v=spf1 include:_spf.example.com ~all" {
		t.Errorf("TXT record should be unchanged")
	}
}
