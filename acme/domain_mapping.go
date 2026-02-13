// Package acme provides domain-to-secret mapping utilities.
//
// Domain Naming Convention:
//
// Normal domains (api.example.com):
//   - Prefix: tls-normal--
//   - Transformation: dots → underscores
//   - Example: api.example.com → tls-normal--api_example_com
//
// Wildcard domains (*.example.com):
//   - Prefix: tls-wildcard--
//   - Transformation: *. → __, dots → underscores
//   - Example: *.example.com → tls-wildcard--__example_com
//
// The mapping is bidirectional and symmetric:
//   domain → DomainToSecret() → secret
//   secret → SecretToDomain() → domain
package acme

import (
	"fmt"
	"regexp"
	"strings"
)

// DomainType represents the type of domain
type DomainType string

const (
	// DomainTypeNormal represents a normal domain (e.g., api.example.com)
	DomainTypeNormal DomainType = "normal"

	// DomainTypeWildcard represents a wildcard domain (e.g., *.example.com)
	DomainTypeWildcard DomainType = "wildcard"
)

const (
	// Secret name prefixes
	secretPrefixNormal   = "tls-normal--"
	secretPrefixWildcard = "tls-wildcard--"

	// Maximum length for Kubernetes Secret names (DNS-1123 subdomain)
	maxSecretNameLength = 253
)

var (
	// domainRegex validates domain names
	// Allows: a-z, 0-9, dots, hyphens, and optional leading wildcard
	domainRegex = regexp.MustCompile(`^(\*\.)?([a-z0-9]([a-z0-9-]*[a-z0-9])?\.)*[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

	// secretNameRegex validates secret names according to our convention
	secretNameRegex = regexp.MustCompile(`^tls-(normal|wildcard)--[a-z0-9_-]+$`)
)

// DomainSecretMapping represents the bidirectional mapping between domain and secret
type DomainSecretMapping struct {
	// OriginalDomain is the original domain name (e.g., "*.example.com", "api.example.com")
	OriginalDomain string

	// DomainType indicates whether it's a normal or wildcard domain
	DomainType DomainType

	// SecretName is the Kubernetes Secret name (e.g., "tls-wildcard--__example_com")
	SecretName string

	// NormalizedDomain is the domain with special characters replaced
	// For normal: "api.example.com" → "api_example_com"
	// For wildcard: "*.example.com" → "__example_com"
	NormalizedDomain string
}

// DomainToSecret converts a domain name to a Kubernetes Secret name.
//
// Examples:
//   "api.example.com"      → "tls-normal--api_example_com"
//   "*.example.com"        → "tls-wildcard--__example_com"
//   "*.api.example.com"    → "tls-wildcard--__api_example_com"
//
// Returns an error if the domain is invalid or the resulting secret name
// exceeds Kubernetes naming limits.
func DomainToSecret(domain string) (string, error) {
	mapping, err := ParseDomain(domain)
	if err != nil {
		return "", err
	}
	return mapping.SecretName, nil
}

// SecretToDomain converts a Kubernetes Secret name back to the original domain.
//
// Examples:
//   "tls-normal--api_example_com"      → "api.example.com"
//   "tls-wildcard--__example_com"      → "*.example.com"
//   "tls-wildcard--__api_example_com"  → "*.api.example.com"
//
// Returns an error if the secret name format is invalid.
func SecretToDomain(secretName string) (string, error) {
	if err := ValidateSecretName(secretName); err != nil {
		return "", err
	}

	var domainType DomainType
	var normalized string

	// Determine type and extract normalized part
	if strings.HasPrefix(secretName, secretPrefixNormal) {
		domainType = DomainTypeNormal
		normalized = strings.TrimPrefix(secretName, secretPrefixNormal)
	} else if strings.HasPrefix(secretName, secretPrefixWildcard) {
		domainType = DomainTypeWildcard
		normalized = strings.TrimPrefix(secretName, secretPrefixWildcard)
	} else {
		return "", fmt.Errorf("invalid secret name prefix: %s", secretName)
	}

	// Convert back to domain
	var domain string
	if domainType == DomainTypeWildcard {
		// Replace __ with *.
		if !strings.HasPrefix(normalized, "__") {
			return "", fmt.Errorf("wildcard secret must start with __: %s", secretName)
		}
		domain = "*." + strings.TrimPrefix(normalized, "__")
	} else {
		domain = normalized
	}

	// Replace underscores with dots
	domain = strings.ReplaceAll(domain, "_", ".")

	// Validate the recovered domain
	if !isValidDomain(domain) {
		return "", fmt.Errorf("recovered domain is invalid: %s", domain)
	}

	return domain, nil
}

// ParseDomain parses a domain and returns its mapping information.
//
// Returns an error if the domain is invalid.
func ParseDomain(domain string) (*DomainSecretMapping, error) {
	if domain == "" {
		return nil, fmt.Errorf("domain cannot be empty")
	}

	// Normalize to lowercase
	domain = strings.ToLower(domain)

	// Validate domain format
	if !isValidDomain(domain) {
		return nil, fmt.Errorf("invalid domain format: %s", domain)
	}

	var domainType DomainType
	var normalized string
	var prefix string

	// Check if wildcard
	if strings.HasPrefix(domain, "*.") {
		domainType = DomainTypeWildcard
		prefix = secretPrefixWildcard

		// Replace *. with __
		normalized = "__" + strings.TrimPrefix(domain, "*.")
	} else {
		domainType = DomainTypeNormal
		prefix = secretPrefixNormal
		normalized = domain
	}

	// Replace dots with underscores
	normalized = strings.ReplaceAll(normalized, ".", "_")

	// Build secret name
	secretName := prefix + normalized

	// Validate secret name length
	if len(secretName) > maxSecretNameLength {
		return nil, fmt.Errorf("secret name too long (%d > %d): %s", len(secretName), maxSecretNameLength, secretName)
	}

	return &DomainSecretMapping{
		OriginalDomain:   domain,
		DomainType:       domainType,
		SecretName:       secretName,
		NormalizedDomain: normalized,
	}, nil
}

// ValidateSecretName checks if a secret name is valid according to our naming convention.
//
// Returns an error if the secret name is invalid.
func ValidateSecretName(secretName string) error {
	if secretName == "" {
		return fmt.Errorf("secret name cannot be empty")
	}

	if len(secretName) > maxSecretNameLength {
		return fmt.Errorf("secret name too long (%d > %d)", len(secretName), maxSecretNameLength)
	}

	if !secretNameRegex.MatchString(secretName) {
		return fmt.Errorf("secret name does not match naming convention: %s", secretName)
	}

	return nil
}

// isValidDomain checks if a domain name is valid.
func isValidDomain(domain string) bool {
	if domain == "" {
		return false
	}

	// Check for invalid patterns
	if strings.Contains(domain, "..") {
		return false
	}
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return false
	}
	if strings.Contains(domain, "**") {
		return false
	}
	if strings.Contains(domain, "*") && !strings.HasPrefix(domain, "*.") {
		return false
	}

	// Check against regex
	return domainRegex.MatchString(domain)
}
