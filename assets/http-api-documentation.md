# HTTP Management API Documentation

## Overview

The jw238dns HTTP management API provides endpoints for managing DNS records and monitoring system status. The API uses simple GET/POST methods (no RESTful style) and returns unified JSON responses.

**Base URL:** `http://localhost:8080` (configurable)

**Authentication:** Bearer Token (for `/dns/*` endpoints)

**Response Format:**
```json
{
  "code": 0,
  "message": "success",
  "data": {}
}
```

---

## Authentication

Protected endpoints require a Bearer token in the Authorization header:

```bash
Authorization: Bearer <your-token>
```

The token is configured via environment variable specified in the config file.

**Public Endpoints (no auth required):**
- `GET /health`
- `GET /status`

**Protected Endpoints (auth required):**
- All `/dns/*` endpoints

---

## DNS Record Management

### POST /dns/add

Add a new DNS record.

**Request Body:**
```json
{
  "name": "example.com.",
  "type": "A",
  "value": ["192.168.1.1"],
  "ttl": 300
}
```

**Parameters:**
- `name` (string, required) - Fully qualified domain name (must end with `.`)
- `type` (string, required) - Record type: A, AAAA, CNAME, MX, TXT, NS, SRV, PTR, SOA, CAA
- `value` (array of strings, required) - Record values
- `ttl` (integer, optional) - Time to live in seconds (default: 300)

**Success Response (200):**
```json
{
  "code": 0,
  "message": "record added successfully",
  "data": {
    "name": "example.com.",
    "type": "A",
    "value": ["192.168.1.1"],
    "ttl": 300
  }
}
```

**Error Responses:**
- `400` - Invalid request (missing fields, invalid type)
- `401` - Unauthorized (missing or invalid token)
- `409` - Record already exists

**Example:**
```bash
curl -X POST http://localhost:8080/dns/add \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test.example.com.",
    "type": "A",
    "value": ["10.0.0.1"],
    "ttl": 600
  }'
```

---

### POST /dns/delete

Delete an existing DNS record.

**Request Body:**
```json
{
  "name": "example.com.",
  "type": "A"
}
```

**Parameters:**
- `name` (string, required) - Domain name to delete
- `type` (string, required) - Record type to delete

**Success Response (200):**
```json
{
  "code": 0,
  "message": "record deleted successfully",
  "data": null
}
```

**Error Responses:**
- `400` - Invalid request
- `401` - Unauthorized
- `404` - Record not found

**Example:**
```bash
curl -X POST http://localhost:8080/dns/delete \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test.example.com.",
    "type": "A"
  }'
```

---

### POST /dns/update

Update an existing DNS record.

**Request Body:**
```json
{
  "name": "example.com.",
  "type": "A",
  "value": ["192.168.1.2"],
  "ttl": 600
}
```

**Parameters:**
- `name` (string, required) - Domain name to update
- `type` (string, required) - Record type
- `value` (array of strings, required) - New record values
- `ttl` (integer, optional) - New TTL value

**Success Response (200):**
```json
{
  "code": 0,
  "message": "record updated successfully",
  "data": {
    "name": "example.com.",
    "type": "A",
    "value": ["192.168.1.2"],
    "ttl": 600
  }
}
```

**Error Responses:**
- `400` - Invalid request
- `401` - Unauthorized
- `404` - Record not found

**Example:**
```bash
curl -X POST http://localhost:8080/dns/update \
  -H "Authorization: Bearer your-token-here" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "test.example.com.",
    "type": "A",
    "value": ["10.0.0.2", "10.0.0.3"],
    "ttl": 300
  }'
```

---

### GET /dns/list

List all DNS records with optional filtering.

**Query Parameters:**
- `name` (string, optional) - Filter by domain name (partial match)
- `type` (string, optional) - Filter by record type

**Success Response (200):**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "records": [
      {
        "name": "example.com.",
        "type": "A",
        "value": ["192.168.1.1"],
        "ttl": 300
      },
      {
        "name": "www.example.com.",
        "type": "CNAME",
        "value": ["example.com."],
        "ttl": 300
      }
    ],
    "total": 2
  }
}
```

**Example - List all records:**
```bash
curl -X GET http://localhost:8080/dns/list \
  -H "Authorization: Bearer your-token-here"
```

**Example - Filter by domain:**
```bash
curl -X GET "http://localhost:8080/dns/list?name=example.com" \
  -H "Authorization: Bearer your-token-here"
```

**Example - Filter by type:**
```bash
curl -X GET "http://localhost:8080/dns/list?type=A" \
  -H "Authorization: Bearer your-token-here"
```

---

### GET /dns/get

Get a specific DNS record.

**Query Parameters:**
- `name` (string, required) - Domain name
- `type` (string, required) - Record type

**Success Response (200):**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "name": "example.com.",
    "type": "A",
    "value": ["192.168.1.1"],
    "ttl": 300
  }
}
```

**Error Responses:**
- `400` - Missing required parameters or invalid type
- `401` - Unauthorized
- `404` - Record not found

**Example:**
```bash
curl -X GET "http://localhost:8080/dns/get?name=example.com.&type=A" \
  -H "Authorization: Bearer your-token-here"
```

---

## System Endpoints

### GET /health

Health check endpoint (public, no authentication required).

**Success Response (200):**
```json
{
  "code": 0,
  "message": "healthy",
  "data": {
    "status": "ok"
  }
}
```

**Example:**
```bash
curl -X GET http://localhost:8080/health
```

---

### GET /status

System status endpoint (public, no authentication required).

**Success Response (200):**
```json
{
  "code": 0,
  "message": "success",
  "data": {
    "status": "running",
    "version": "1.0.0"
  }
}
```

**Example:**
```bash
curl -X GET http://localhost:8080/status
```

---

## Error Codes

| Code | Description |
|------|-------------|
| 0    | Success |
| 400  | Bad Request - Invalid parameters or malformed JSON |
| 401  | Unauthorized - Missing or invalid authentication token |
| 404  | Not Found - Record does not exist |
| 409  | Conflict - Record already exists |
| 500  | Internal Server Error |

---

## Common Error Responses

**401 Unauthorized:**
```json
{
  "code": 401,
  "message": "unauthorized",
  "data": null
}
```

**400 Bad Request:**
```json
{
  "code": 400,
  "message": "invalid record type: INVALID",
  "data": null
}
```

**404 Not Found:**
```json
{
  "code": 404,
  "message": "record not found",
  "data": null
}
```

**409 Conflict:**
```json
{
  "code": 409,
  "message": "record already exists",
  "data": null
}
```

---

## Record Types

Supported DNS record types:

- **A** - IPv4 address
- **AAAA** - IPv6 address
- **CNAME** - Canonical name
- **MX** - Mail exchange
- **TXT** - Text record
- **NS** - Name server
- **SRV** - Service record
- **PTR** - Pointer record
- **SOA** - Start of authority
- **CAA** - Certification Authority Authorization

---

## Best Practices

1. **Always use FQDN:** Domain names must end with a dot (`.`)
   - ✓ Correct: `example.com.`
   - ✗ Incorrect: `example.com`

2. **TTL values:** Recommended range is 60-86400 seconds
   - Default: 300 seconds (5 minutes)
   - Minimum: 60 seconds
   - Maximum: 86400 seconds (24 hours)

3. **Multiple values:** Some record types support multiple values
   - A records: Multiple IP addresses for load balancing
   - TXT records: Multiple text strings

4. **Authentication:** Store tokens securely, never commit to version control

5. **Rate limiting:** Consider implementing rate limiting in production

---

## Examples

### Complete Workflow Example

```bash
# Set your token
TOKEN="your-secret-token"

# 1. Add a new A record
curl -X POST http://localhost:8080/dns/add \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api.example.com.",
    "type": "A",
    "value": ["10.0.0.1"],
    "ttl": 300
  }'

# 2. List all records
curl -X GET http://localhost:8080/dns/list \
  -H "Authorization: Bearer $TOKEN"

# 3. Get specific record
curl -X GET "http://localhost:8080/dns/get?name=api.example.com.&type=A" \
  -H "Authorization: Bearer $TOKEN"

# 4. Update the record
curl -X POST http://localhost:8080/dns/update \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api.example.com.",
    "type": "A",
    "value": ["10.0.0.2"],
    "ttl": 600
  }'

# 5. Delete the record
curl -X POST http://localhost:8080/dns/delete \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "api.example.com.",
    "type": "A"
  }'
```

### Python Example

```python
import requests

BASE_URL = "http://localhost:8080"
TOKEN = "your-secret-token"
HEADERS = {
    "Authorization": f"Bearer {TOKEN}",
    "Content-Type": "application/json"
}

# Add record
response = requests.post(
    f"{BASE_URL}/dns/add",
    headers=HEADERS,
    json={
        "name": "test.example.com.",
        "type": "A",
        "value": ["10.0.0.1"],
        "ttl": 300
    }
)
print(response.json())

# List records
response = requests.get(
    f"{BASE_URL}/dns/list",
    headers=HEADERS
)
print(response.json())
```

---

## Troubleshooting

### 401 Unauthorized
- Check that the Authorization header is present
- Verify the token matches the configured value
- Ensure the token is prefixed with "Bearer "

### 404 Not Found
- Verify the domain name is correct and ends with `.`
- Check that the record type matches exactly
- Use `/dns/list` to see all available records

### 400 Bad Request
- Validate JSON syntax
- Ensure all required fields are present
- Check that record type is valid
- Verify domain name format (must end with `.`)

---

**Last Updated:** 2026-02-13
**Version:** 1.0.0
