# Add GeoIP Distance-Based Sorting

## Goal
Add GeoIP support to DNS resolution. When users query DNS, use GeoIP to determine client location and sort all IP addresses in the response by physical distance from the client (closest first).

## Requirements

### 1. GeoIP Package (`internal/geoip/`)
- Create MMDB reader for MaxMind GeoLite2-City database
- Implement IP-to-coordinates lookup (latitude/longitude)
- Implement Haversine distance calculation for great-circle distance
- Provide sorting function to order IP addresses by distance from client location
- MMDB file path: `/assets/GeoLite2-City.mmdb`

### 2. Type Updates (`internal/types/`)
- Add `ClientIP net.IP` field to `QueryInfo` struct
- This allows client location to flow from Frontend to Backend

### 3. Backend Integration (`internal/dns/`)
- Add GeoIP reader to `Backend` struct
- Add GeoIP configuration to `BackendConfig`:
  - `EnableGeoIP bool` - feature flag
  - `MMDBPath string` - path to MMDB file
- Integrate GeoIP sorting into `ApplyRules` or create new sorting method
- Only sort A and AAAA record types (not CNAME, TXT, etc.)
- Sort IP addresses by distance (closest first)

### 4. Frontend Integration (`internal/dns/`)
- Extract client IP from DNS query context
- Populate `QueryInfo.ClientIP` field
- Pass client IP through to Backend for GeoIP sorting

## Acceptance Criteria

- [ ] `internal/geoip/geoip.go` created with MMDB reader
- [ ] `internal/geoip/geoip_test.go` with distance calculation tests
- [ ] `QueryInfo` has `ClientIP` field
- [ ] Backend config has `EnableGeoIP` and `MMDBPath` fields
- [ ] GeoIP sorting integrated into Backend
- [ ] Only A and AAAA records are sorted by distance
- [ ] Frontend extracts and passes client IP
- [ ] All tests pass with `go test ./...`
- [ ] Test coverage >85% for geoip package
- [ ] Distance calculation uses Haversine formula
- [ ] Handles missing/invalid client IPs gracefully (no sorting)
- [ ] Handles IPs not in MMDB gracefully (no sorting)

## Technical Notes

### Libraries
- Use `github.com/oschwald/geoip2-golang` for MMDB reading
- Provides typed structs with `City.Location.Latitude/Longitude`

### Distance Formula
- Haversine formula for great-circle distance
- Returns distance in kilometers
- Standard library `math` package sufficient

### Integration Points
1. **Frontend**: Extract client IP from `dns.ResponseWriter.RemoteAddr()`
2. **Backend**: Apply GeoIP sorting in `ApplyRules` after default TTL logic
3. **Storage**: No changes needed (read-only operation)

### Configuration Example
```go
config := &BackendConfig{
    EnableGeoIP: true,
    MMDBPath: "/assets/GeoLite2-City.mmdb",
    DefaultTTL: 300,
}
```

### Error Handling
- If MMDB file not found: log warning, disable GeoIP
- If client IP not in MMDB: skip sorting, return original order
- If client IP invalid/missing: skip sorting, return original order

## Out of Scope
- ASN-based routing
- Country-level filtering
- Custom distance algorithms
- Caching of GeoIP lookups
- Dynamic MMDB reloading

## Dependencies
- Depends on: DNS Core (completed in commit 8861029)
- MMDB file must exist at `/assets/GeoLite2-City.mmdb`
