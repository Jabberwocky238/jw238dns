package acme

import (
	"strings"
	"testing"
)

// TestDomainToSecret tests domain to secret conversion
func TestDomainToSecret(t *testing.T) {
	tests := []struct {
		name       string
		domain     string
		wantSecret string
		wantErr    bool
	}{
		// Normal domains - basic cases
		{
			name:       "normal domain - single level",
			domain:     "example.com",
			wantSecret: "tls-normal--example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - two levels",
			domain:     "api.example.com",
			wantSecret: "tls-normal--api_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - three levels",
			domain:     "v1.api.example.com",
			wantSecret: "tls-normal--v1_api_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - four levels",
			domain:     "service.v1.api.example.com",
			wantSecret: "tls-normal--service_v1_api_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - with hyphen",
			domain:     "my-api.example.com",
			wantSecret: "tls-normal--my-api_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - multiple hyphens",
			domain:     "my-cool-api.example.com",
			wantSecret: "tls-normal--my-cool-api_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - hyphen in multiple parts",
			domain:     "my-api.my-service.example.com",
			wantSecret: "tls-normal--my-api_my-service_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - numbers",
			domain:     "api1.example.com",
			wantSecret: "tls-normal--api1_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - mixed",
			domain:     "api-v2.service-1.example.com",
			wantSecret: "tls-normal--api-v2_service-1_example_com",
			wantErr:    false,
		},
		{
			name:       "normal domain - short",
			domain:     "a.b.c",
			wantSecret: "tls-normal--a_b_c",
			wantErr:    false,
		},

		// Wildcard domains - basic cases
		{
			name:       "wildcard domain - root",
			domain:     "*.example.com",
			wantSecret: "tls-wildcard--__example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - subdomain",
			domain:     "*.api.example.com",
			wantSecret: "tls-wildcard--__api_example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - deep subdomain",
			domain:     "*.v1.api.example.com",
			wantSecret: "tls-wildcard--__v1_api_example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - with hyphen",
			domain:     "*.my-api.example.com",
			wantSecret: "tls-wildcard--__my-api_example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - multiple hyphens",
			domain:     "*.my-cool-api.example.com",
			wantSecret: "tls-wildcard--__my-cool-api_example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - numbers",
			domain:     "*.api1.example.com",
			wantSecret: "tls-wildcard--__api1_example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - mixed",
			domain:     "*.api-v2.service-1.example.com",
			wantSecret: "tls-wildcard--__api-v2_service-1_example_com",
			wantErr:    false,
		},
		{
			name:       "wildcard domain - short",
			domain:     "*.b.c",
			wantSecret: "tls-wildcard--__b_c",
			wantErr:    false,
		},

		// Case insensitivity
		{
			name:       "uppercase domain",
			domain:     "API.EXAMPLE.COM",
			wantSecret: "tls-normal--api_example_com",
			wantErr:    false,
		},
		{
			name:       "mixed case domain",
			domain:     "Api.Example.Com",
			wantSecret: "tls-normal--api_example_com",
			wantErr:    false,
		},
		{
			name:       "uppercase wildcard",
			domain:     "*.API.EXAMPLE.COM",
			wantSecret: "tls-wildcard--__api_example_com",
			wantErr:    false,
		},

		// Error cases - empty
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},

		// Error cases - invalid characters
		{
			name:    "domain with @",
			domain:  "api@example.com",
			wantErr: true,
		},
		{
			name:    "domain with space",
			domain:  "api example.com",
			wantErr: true,
		},
		{
			name:    "domain with slash",
			domain:  "api/example.com",
			wantErr: true,
		},
		{
			name:    "domain with underscore",
			domain:  "api_example.com",
			wantErr: true,
		},

		// Error cases - format errors
		{
			name:    "double wildcard",
			domain:  "**.example.com",
			wantErr: true,
		},
		{
			name:    "multi-level wildcard",
			domain:  "*.*.example.com",
			wantErr: true,
		},
		{
			name:    "wildcard without dot",
			domain:  "*example.com",
			wantErr: true,
		},
		{
			name:    "wildcard in middle",
			domain:  "api.*.example.com",
			wantErr: true,
		},
		{
			name:    "starts with dot",
			domain:  ".example.com",
			wantErr: true,
		},
		{
			name:    "ends with dot",
			domain:  "example.com.",
			wantErr: true,
		},
		{
			name:    "consecutive dots",
			domain:  "api..example.com",
			wantErr: true,
		},
		{
			name:    "starts with hyphen",
			domain:  "-api.example.com",
			wantErr: true,
		},
		{
			name:    "ends with hyphen",
			domain:  "api-.example.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DomainToSecret(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("DomainToSecret(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantSecret {
				t.Errorf("DomainToSecret(%q) = %q, want %q", tt.domain, got, tt.wantSecret)
			}
		})
	}
}

// TestSecretToDomain tests secret to domain conversion
func TestSecretToDomain(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		wantDomain string
		wantErr    bool
	}{
		// Normal domains
		{
			name:       "normal secret - single level",
			secretName: "tls-normal--example_com",
			wantDomain: "example.com",
			wantErr:    false,
		},
		{
			name:       "normal secret - two levels",
			secretName: "tls-normal--api_example_com",
			wantDomain: "api.example.com",
			wantErr:    false,
		},
		{
			name:       "normal secret - three levels",
			secretName: "tls-normal--v1_api_example_com",
			wantDomain: "v1.api.example.com",
			wantErr:    false,
		},
		{
			name:       "normal secret - with hyphen",
			secretName: "tls-normal--my-api_example_com",
			wantDomain: "my-api.example.com",
			wantErr:    false,
		},
		{
			name:       "normal secret - multiple hyphens",
			secretName: "tls-normal--my-cool-api_example_com",
			wantDomain: "my-cool-api.example.com",
			wantErr:    false,
		},

		// Wildcard domains
		{
			name:       "wildcard secret - root",
			secretName: "tls-wildcard--__example_com",
			wantDomain: "*.example.com",
			wantErr:    false,
		},
		{
			name:       "wildcard secret - subdomain",
			secretName: "tls-wildcard--__api_example_com",
			wantDomain: "*.api.example.com",
			wantErr:    false,
		},
		{
			name:       "wildcard secret - deep subdomain",
			secretName: "tls-wildcard--__v1_api_example_com",
			wantDomain: "*.v1.api.example.com",
			wantErr:    false,
		},
		{
			name:       "wildcard secret - with hyphen",
			secretName: "tls-wildcard--__my-api_example_com",
			wantDomain: "*.my-api.example.com",
			wantErr:    false,
		},

		// Error cases
		{
			name:       "empty secret name",
			secretName: "",
			wantErr:    true,
		},
		{
			name:       "invalid prefix",
			secretName: "tls-invalid--example_com",
			wantErr:    true,
		},
		{
			name:       "missing prefix",
			secretName: "example_com",
			wantErr:    true,
		},
		{
			name:       "wildcard without double underscore",
			secretName: "tls-wildcard--_example_com",
			wantErr:    true,
		},
		{
			name:       "invalid characters",
			secretName: "tls-normal--example@com",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SecretToDomain(tt.secretName)
			if (err != nil) != tt.wantErr {
				t.Errorf("SecretToDomain(%q) error = %v, wantErr %v", tt.secretName, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantDomain {
				t.Errorf("SecretToDomain(%q) = %q, want %q", tt.secretName, got, tt.wantDomain)
			}
		})
	}
}

// TestRoundTrip tests bidirectional conversion
func TestRoundTrip(t *testing.T) {
	domains := []string{
		// Normal domains
		"example.com",
		"api.example.com",
		"v1.api.example.com",
		"service.v1.api.example.com",
		"my-api.example.com",
		"my-cool-api.example.com",
		"my-api.my-service.example.com",
		"api1.example.com",
		"api-v2.service-1.example.com",
		"a.b.c",

		// Wildcard domains
		"*.example.com",
		"*.api.example.com",
		"*.v1.api.example.com",
		"*.my-api.example.com",
		"*.my-cool-api.example.com",
		"*.api1.example.com",
		"*.api-v2.service-1.example.com",
		"*.b.c",

		// Case variations (should normalize to lowercase)
		"API.EXAMPLE.COM",
		"*.API.EXAMPLE.COM",
	}

	for _, domain := range domains {
		t.Run(domain, func(t *testing.T) {
			// Normalize expected domain to lowercase
			expectedDomain := strings.ToLower(domain)

			// Domain → Secret
			secret, err := DomainToSecret(domain)
			if err != nil {
				t.Fatalf("DomainToSecret(%q) failed: %v", domain, err)
			}

			// Secret → Domain
			recovered, err := SecretToDomain(secret)
			if err != nil {
				t.Fatalf("SecretToDomain(%q) failed: %v", secret, err)
			}

			// Check if recovered matches original (normalized)
			if recovered != expectedDomain {
				t.Errorf("Round trip failed: %q → %q → %q (expected %q)", domain, secret, recovered, expectedDomain)
			}

			t.Logf("✓ %q → %q → %q", domain, secret, recovered)
		})
	}
}

// TestParseDomain tests domain parsing
func TestParseDomain(t *testing.T) {
	tests := []struct {
		name               string
		domain             string
		wantType           DomainType
		wantSecret         string
		wantNormalized     string
		wantErr            bool
	}{
		{
			name:           "normal domain",
			domain:         "api.example.com",
			wantType:       DomainTypeNormal,
			wantSecret:     "tls-normal--api_example_com",
			wantNormalized: "api_example_com",
			wantErr:        false,
		},
		{
			name:           "wildcard domain",
			domain:         "*.example.com",
			wantType:       DomainTypeWildcard,
			wantSecret:     "tls-wildcard--__example_com",
			wantNormalized: "__example_com",
			wantErr:        false,
		},
		{
			name:    "empty domain",
			domain:  "",
			wantErr: true,
		},
		{
			name:    "invalid domain",
			domain:  "invalid@domain.com",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseDomain(tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseDomain(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if got.DomainType != tt.wantType {
				t.Errorf("ParseDomain(%q).DomainType = %v, want %v", tt.domain, got.DomainType, tt.wantType)
			}
			if got.SecretName != tt.wantSecret {
				t.Errorf("ParseDomain(%q).SecretName = %q, want %q", tt.domain, got.SecretName, tt.wantSecret)
			}
			if got.NormalizedDomain != tt.wantNormalized {
				t.Errorf("ParseDomain(%q).NormalizedDomain = %q, want %q", tt.domain, got.NormalizedDomain, tt.wantNormalized)
			}
			if got.OriginalDomain != strings.ToLower(tt.domain) {
				t.Errorf("ParseDomain(%q).OriginalDomain = %q, want %q", tt.domain, got.OriginalDomain, strings.ToLower(tt.domain))
			}
		})
	}
}

// TestValidateSecretName tests secret name validation
func TestValidateSecretName(t *testing.T) {
	tests := []struct {
		name       string
		secretName string
		wantErr    bool
	}{
		{
			name:       "valid normal secret",
			secretName: "tls-normal--example_com",
			wantErr:    false,
		},
		{
			name:       "valid wildcard secret",
			secretName: "tls-wildcard--__example_com",
			wantErr:    false,
		},
		{
			name:       "valid with hyphens",
			secretName: "tls-normal--my-api_example_com",
			wantErr:    false,
		},
		{
			name:       "empty secret name",
			secretName: "",
			wantErr:    true,
		},
		{
			name:       "invalid prefix",
			secretName: "tls-invalid--example_com",
			wantErr:    true,
		},
		{
			name:       "missing prefix",
			secretName: "example_com",
			wantErr:    true,
		},
		{
			name:       "invalid characters",
			secretName: "tls-normal--example@com",
			wantErr:    true,
		},
		{
			name:       "too long",
			secretName: "tls-normal--" + strings.Repeat("a", 300),
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateSecretName(tt.secretName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateSecretName(%q) error = %v, wantErr %v", tt.secretName, err, tt.wantErr)
			}
		})
	}
}

// TestIsValidDomain tests domain validation
func TestIsValidDomain(t *testing.T) {
	tests := []struct {
		name   string
		domain string
		want   bool
	}{
		// Valid domains
		{"valid single level", "example.com", true},
		{"valid two levels", "api.example.com", true},
		{"valid three levels", "v1.api.example.com", true},
		{"valid with hyphen", "my-api.example.com", true},
		{"valid with numbers", "api1.example.com", true},
		{"valid wildcard", "*.example.com", true},
		{"valid wildcard subdomain", "*.api.example.com", true},

		// Invalid domains
		{"empty", "", false},
		{"starts with dot", ".example.com", false},
		{"ends with dot", "example.com.", false},
		{"consecutive dots", "api..example.com", false},
		{"double wildcard", "**.example.com", false},
		{"multi-level wildcard", "*.*.example.com", false},
		{"wildcard without dot", "*example.com", false},
		{"wildcard in middle", "api.*.example.com", false},
		{"starts with hyphen", "-api.example.com", false},
		{"ends with hyphen", "api-.example.com", false},
		{"with underscore", "api_example.com", false},
		{"with space", "api example.com", false},
		{"with @", "api@example.com", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isValidDomain(tt.domain)
			if got != tt.want {
				t.Errorf("isValidDomain(%q) = %v, want %v", tt.domain, got, tt.want)
			}
		})
	}
}

// TestMaxLength tests maximum length handling
func TestMaxLength(t *testing.T) {
	// Create a very long domain name that will exceed 253 chars after conversion
	// Secret name format: "tls-normal--" (12 chars) + domain with dots replaced by underscores
	// So we need domain length > 241 chars to exceed the limit
	longPart := strings.Repeat("a", 120)
	longDomain := longPart + "." + longPart + ".com"

	_, err := DomainToSecret(longDomain)
	if err == nil {
		t.Error("Expected error for overly long domain, got nil")
	}
}

// BenchmarkDomainToSecret benchmarks domain to secret conversion
func BenchmarkDomainToSecret(b *testing.B) {
	domain := "api.example.com"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = DomainToSecret(domain)
	}
}

// BenchmarkSecretToDomain benchmarks secret to domain conversion
func BenchmarkSecretToDomain(b *testing.B) {
	secret := "tls-normal--api_example_com"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = SecretToDomain(secret)
	}
}

// BenchmarkRoundTrip benchmarks full round trip conversion
func BenchmarkRoundTrip(b *testing.B) {
	domain := "api.example.com"
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		secret, _ := DomainToSecret(domain)
		_, _ = SecretToDomain(secret)
	}
}
