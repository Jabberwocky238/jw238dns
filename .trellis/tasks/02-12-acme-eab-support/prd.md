# Add ACME EAB Support with Mode Switching

## Goal
Add External Account Binding (EAB) support to the ACME client to enable ZeroSSL certificate issuance, and implement mode switching between ZeroSSL and Let's Encrypt.

## Background
- ZeroSSL requires EAB (External Account Binding) for ACME registration
- Current code only supports Let's Encrypt (no EAB required)
- K8s deployment YAML already has EAB config structure, but Go code doesn't implement it

## Requirements

### 1. Add EAB Configuration Support
- Add `EABConfig` struct to `acme/config.go` with `KID` and `HMACKey` fields
- Add `EAB` field to `Config` struct
- Add `ZeroSSLProduction()` helper function returning `https://acme.zerossl.com/v2/DV90`

### 2. Implement EAB Registration
- Modify `acme/client.go` `Register()` method to:
  - Check if EAB credentials are provided (`config.EAB.KID != ""`)
  - If yes: use `RegisterWithExternalAccountBinding()` with `registration.RegisterEABOptions`
  - If no: use existing plain `Register()` (for Let's Encrypt)

### 3. Wire EAB Config from YAML/Env
- Add `EABEnvConfig` struct to `cmd/jw238dns/main.go` with:
  - `KidEnv string` (yaml tag: `kid_env`)
  - `HmacEnv string` (yaml tag: `hmac_env`)
- Add `EAB EABEnvConfig` field to `ACMEConfig` struct
- In config loading, resolve env vars:
  ```go
  EAB: acme.EABConfig{
    KID:     os.Getenv(acmeConfig.EAB.KidEnv),
    HMACKey: os.Getenv(acmeConfig.EAB.HmacEnv),
  }
  ```

### 4. Add Mode Switching
- Update `assets/k8s-deployment.yaml` to support mode switching:
  - Add `mode` field to ACME config (values: `letsencrypt`, `zerossl`)
  - Conditionally set `server` URL based on mode
  - Keep EAB config optional (only required for ZeroSSL)

### 5. Update Documentation
- Update `assets/example-acme-config.yaml` with EAB section and ZeroSSL example
- Add comments explaining mode switching

### 6. Add Tests
- Test `ZeroSSLProduction()` helper
- Test `Config` with EAB fields populated
- Test EAB registration flow (if feasible with mocks)

## Acceptance Criteria
- [ ] `acme.Config` has `EAB` field with `KID` and `HMACKey`
- [ ] `acme.Client.Register()` uses EAB when credentials provided
- [ ] `cmd/jw238dns/main.go` loads EAB from env vars specified in YAML
- [ ] K8s deployment supports mode switching between `letsencrypt` and `zerossl`
- [ ] Tests pass for new EAB config and helpers
- [ ] Example config documents EAB usage
- [ ] Lint and typecheck pass

## Technical Notes
- Use lego library's `registration.RegisterEABOptions` for EAB registration
- Follow existing env var indirection pattern (like `AuthConfig.TokenEnv`)
- EAB credentials should only be resolved at runtime from env vars (not stored in YAML)
- Maintain backward compatibility: if no EAB config, use plain registration
