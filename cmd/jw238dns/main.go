package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"jabberwocky238/jw238dns/acme"
	"jabberwocky238/jw238dns/dns"
	jwhttp "jabberwocky238/jw238dns/http"
	"jabberwocky238/jw238dns/storage"

	mdns "github.com/miekg/dns"
	"gopkg.in/yaml.v3"
)

func main() {
	// Parse command
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "healthcheck":
			os.Exit(0)
		case "serve":
			// Continue to serve
		case "version":
			fmt.Println("jw238dns v1.0.0")
			return
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			fmt.Println("Available commands: serve, healthcheck, version")
			os.Exit(1)
		}
	}

	// Setup logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	slog.Info("Starting jw238dns server")

	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "/app/config/app.yaml"
	}

	config, err := loadConfig(configPath)
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := validateConfig(config); err != nil {
		slog.Error("Configuration validation failed", "error", err)
		os.Exit(1)
	}

	// Initialize storage
	store := storage.NewMemoryStorage()

	// Create context for background tasks
	ctx, cancel := context.WithCancel(context.Background())

	// Load initial records based on storage type
	if config.Storage.Type == "file" {
		loader := storage.NewJSONFileLoader(config.Storage.File.Path, store)
		records, err := loader.Load()
		if err != nil {
			slog.Warn("Failed to load initial records", "error", err)
		} else {
			for _, record := range records {
				if err := store.Create(ctx, record); err != nil {
					slog.Warn("Failed to create record", "name", record.Name, "error", err)
				}
			}
			slog.Info("Loaded initial records", "count", len(records))
		}
	} else if config.Storage.Type == "configmap" {
		// Initialize Kubernetes client for ConfigMap storage
		k8sClient, err := storage.NewK8sClient()
		if err != nil {
			slog.Error("Failed to create Kubernetes client", "error", err)
			os.Exit(1)
		}

		// Create ConfigMap watcher
		watcher := storage.NewConfigMapWatcher(
			k8sClient,
			config.Storage.ConfigMap.Namespace,
			config.Storage.ConfigMap.Name,
			config.Storage.ConfigMap.DataKey,
			store,
		)

		// Start watching ConfigMap in background
		go func() {
			slog.Info("Starting ConfigMap watcher",
				"namespace", config.Storage.ConfigMap.Namespace,
				"name", config.Storage.ConfigMap.Name,
				"key", config.Storage.ConfigMap.DataKey)
			if err := watcher.WatchAndSync(ctx); err != nil {
				slog.Error("ConfigMap watcher failed", "error", err)
			}
		}()

		slog.Info("ConfigMap storage initialized")
	}

	// Initialize DNS backend with config
	backendConfig := dns.DefaultBackendConfig()
	if config.GeoIP.Enabled {
		backendConfig.EnableGeoIP = true
		backendConfig.MMDBPath = config.GeoIP.MMDBPath
	}

	// Wire upstream forwarding config.
	if config.DNS.Upstream.Enabled {
		backendConfig.Forwarder.Enabled = true
		if len(config.DNS.Upstream.Servers) > 0 {
			backendConfig.Forwarder.Servers = config.DNS.Upstream.Servers
		}
		if config.DNS.Upstream.Timeout != "" {
			d, err := time.ParseDuration(config.DNS.Upstream.Timeout)
			if err != nil {
				slog.Warn("Invalid upstream timeout, using default 5s",
					"value", config.DNS.Upstream.Timeout,
					"error", err,
				)
			} else {
				backendConfig.Forwarder.Timeout = d
			}
		}
		slog.Info("Upstream DNS forwarding enabled",
			"servers", backendConfig.Forwarder.Servers,
			"timeout", backendConfig.Forwarder.Timeout,
		)
	}

	backend := dns.NewBackend(store, backendConfig)
	defer backend.Close()
	frontend := dns.NewFrontend(backend)

	// Create DNS handler
	dnsHandler := &DNSHandler{frontend: frontend}

	// Start DNS servers
	defer cancel()

	if config.DNS.UDPEnabled {
		udpServer := &mdns.Server{
			Addr:    config.DNS.Listen,
			Net:     "udp",
			Handler: dnsHandler,
		}
		go func() {
			slog.Info("DNS UDP server starting", "address", config.DNS.Listen)
			if err := udpServer.ListenAndServe(); err != nil {
				slog.Error("DNS UDP server failed", "error", err)
			}
		}()
		defer udpServer.Shutdown()
	}

	if config.DNS.TCPEnabled {
		tcpServer := &mdns.Server{
			Addr:    config.DNS.Listen,
			Net:     "tcp",
			Handler: dnsHandler,
		}
		go func() {
			slog.Info("DNS TCP server starting", "address", config.DNS.Listen)
			if err := tcpServer.ListenAndServe(); err != nil {
				slog.Error("DNS TCP server failed", "error", err)
			}
		}()
		defer tcpServer.Shutdown()
	}

	// Start HTTP management server if enabled.
	if config.HTTP.Enabled {
		authToken := ""
		if config.HTTP.Auth.Enabled && config.HTTP.Auth.TokenEnv != "" {
			authToken = os.Getenv(config.HTTP.Auth.TokenEnv)
		}
		httpSrv := jwhttp.NewServer(jwhttp.ServerConfig{
			Listen:    config.HTTP.Listen,
			AuthToken: authToken,
		}, store)
		go func() {
			if err := httpSrv.Start(); err != nil {
				slog.Error("HTTP management server failed", "error", err)
			}
		}()
		defer httpSrv.Shutdown()
	}

	// Initialize ACME Manager if enabled
	if config.ACME.Enabled {
		slog.Info("Initializing ACME manager", "mode", config.ACME.Mode, "email", config.ACME.Email)

		// Create certificate storage
		var certStorage acme.CertificateStorage
		if config.ACME.Storage.Type == "kubernetes-secret" {
			k8sClient, err := storage.NewK8sClient()
			if err != nil {
				slog.Error("Failed to create Kubernetes client for ACME", "error", err)
			} else {
				certStorage = acme.NewKubernetesSecretStorage(k8sClient, config.ACME.Storage.Namespace)
				slog.Info("Using Kubernetes Secret storage for certificates", "namespace", config.ACME.Storage.Namespace)
			}
		} else if config.ACME.Storage.Type == "file" {
			certStorage = acme.NewFileStorage(config.ACME.Storage.Path)
			slog.Info("Using file storage for certificates", "path", config.ACME.Storage.Path)
		}

		if certStorage != nil {
			// Create ACME manager
			acmeConfig := config.ACME.ToACMEConfig()
			manager, err := acme.NewManager(acmeConfig, store, certStorage)
			if err != nil {
				slog.Error("Failed to create ACME manager", "error", err)
			} else {
				// Obtain certificates for configured domains
				if len(config.Domains) > 0 {
					for _, domainConfig := range config.Domains {
						if domainConfig.AutoObtain {
							slog.Info("Obtaining certificate", "domains", domainConfig.Domains)
							if err := manager.ObtainCertificate(ctx, domainConfig.Domains); err != nil {
								slog.Error("Failed to obtain certificate", "domains", domainConfig.Domains, "error", err)
							}
						}
					}
				}

				// Start auto-renewal if enabled
				if config.ACME.AutoRenew {
					manager.StartAutoRenewal(ctx)
					slog.Info("ACME auto-renewal started")
				}
			}
		}
	}

	// Wait for interrupt signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	<-sigCh
	slog.Info("Shutting down server...")

	cancel()
	slog.Info("Server stopped")
}

// DNSHandler implements dns.Handler interface
type DNSHandler struct {
	frontend *dns.Frontend
}

func (h *DNSHandler) ServeDNS(w mdns.ResponseWriter, r *mdns.Msg) {
	// Extract client IP
	var clientIP net.IP
	if addr, ok := w.RemoteAddr().(*net.UDPAddr); ok {
		clientIP = addr.IP
	} else if addr, ok := w.RemoteAddr().(*net.TCPAddr); ok {
		clientIP = addr.IP
	}

	// Create context with client IP
	ctx := dns.ContextWithClientIP(context.Background(), clientIP)

	// Process query
	resp, err := h.frontend.ReceiveQuery(ctx, r)
	if err != nil {
		slog.Error("Failed to process query", "error", err, "client", clientIP)
		resp = new(mdns.Msg)
		resp.SetRcode(r, mdns.RcodeServerFailure)
	}

	// Send response
	if err := w.WriteMsg(resp); err != nil {
		slog.Error("Failed to write response", "error", err)
	}
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &config, nil
}

// validateConfig validates the configuration and checks required environment variables
func validateConfig(config *Config) error {
	// Validate HTTP authentication
	if config.HTTP.Enabled && config.HTTP.Auth.Enabled {
		if config.HTTP.Auth.TokenEnv == "" {
			return fmt.Errorf("HTTP authentication is enabled but token_env is not configured")
		}
		token := os.Getenv(config.HTTP.Auth.TokenEnv)
		if token == "" {
			return fmt.Errorf("HTTP authentication is enabled but environment variable %s is not set or empty", config.HTTP.Auth.TokenEnv)
		}
		slog.Info("HTTP authentication validated", "token_env", config.HTTP.Auth.TokenEnv)
	}

	// Validate ACME configuration
	if config.ACME.Enabled {
		// Validate email
		if config.ACME.Email == "" {
			return fmt.Errorf("ACME is enabled but email is not configured")
		}

		// Validate EAB credentials for ZeroSSL
		if config.ACME.Mode == "zerossl" {
			if config.ACME.EAB.KidEnv == "" || config.ACME.EAB.HmacEnv == "" {
				return fmt.Errorf("ACME mode is 'zerossl' but EAB environment variable names are not configured")
			}

			kid := os.Getenv(config.ACME.EAB.KidEnv)
			hmac := os.Getenv(config.ACME.EAB.HmacEnv)

			if kid == "" {
				return fmt.Errorf("ACME mode is 'zerossl' but environment variable %s (EAB KID) is not set or empty", config.ACME.EAB.KidEnv)
			}
			if hmac == "" {
				return fmt.Errorf("ACME mode is 'zerossl' but environment variable %s (EAB HMAC) is not set or empty", config.ACME.EAB.HmacEnv)
			}

			slog.Info("ZeroSSL EAB credentials validated",
				"kid_env", config.ACME.EAB.KidEnv,
				"hmac_env", config.ACME.EAB.HmacEnv)
		}

		// Validate storage configuration
		if config.ACME.Storage.Type == "" {
			return fmt.Errorf("ACME is enabled but storage type is not configured")
		}
		if config.ACME.Storage.Type == "kubernetes-secret" && config.ACME.Storage.Namespace == "" {
			return fmt.Errorf("ACME storage type is 'kubernetes-secret' but namespace is not configured")
		}
		if config.ACME.Storage.Type == "file" && config.ACME.Storage.Path == "" {
			return fmt.Errorf("ACME storage type is 'file' but path is not configured")
		}
	}

	return nil
}

// Config represents the application configuration
type Config struct {
	DNS     DNSConfig       `yaml:"dns"`
	GeoIP   GeoIPConfig     `yaml:"geoip"`
	Storage StorageConfig   `yaml:"storage"`
	HTTP    HTTPConfig      `yaml:"http"`
	ACME    ACMEConfig      `yaml:"acme"`
	Domains []DomainConfig  `yaml:"domains"`
}

// DomainConfig represents a domain certificate configuration
type DomainConfig struct {
	Domains     []string `yaml:"domains"`
	AutoObtain  bool     `yaml:"auto_obtain"`
}

type DNSConfig struct {
	Listen     string         `yaml:"listen"`
	TCPEnabled bool           `yaml:"tcp_enabled"`
	UDPEnabled bool           `yaml:"udp_enabled"`
	Upstream   UpstreamConfig `yaml:"upstream"`
}

// UpstreamConfig controls forwarding of unresolved queries to upstream DNS servers.
type UpstreamConfig struct {
	Enabled bool     `yaml:"enabled"`
	Servers []string `yaml:"servers"`
	Timeout string   `yaml:"timeout"`
}

type GeoIPConfig struct {
	Enabled  bool   `yaml:"enabled"`
	MMDBPath string `yaml:"mmdb_path"`
}

type StorageConfig struct {
	Type      string                 `yaml:"type"`
	ConfigMap ConfigMapStorageConfig `yaml:"configmap"`
	File      FileStorageConfig      `yaml:"file"`
}

type ConfigMapStorageConfig struct {
	Namespace string `yaml:"namespace"`
	Name      string `yaml:"name"`
	DataKey   string `yaml:"data_key"`
}

type FileStorageConfig struct {
	Path string `yaml:"path"`
}

type HTTPConfig struct {
	Enabled bool       `yaml:"enabled"`
	Listen  string     `yaml:"listen"`
	Auth    AuthConfig `yaml:"auth"`
}

type AuthConfig struct {
	Enabled  bool   `yaml:"enabled"`
	TokenEnv string `yaml:"token_env"`
}

type EABEnvConfig struct {
	KidEnv  string `yaml:"kid_env"`
	HmacEnv string `yaml:"hmac_env"`
}

type ACMEConfig struct {
	Enabled         bool              `yaml:"enabled"`
	Mode            string            `yaml:"mode"`
	Server          string            `yaml:"server"`
	Email           string            `yaml:"email"`
	KeyType         string            `yaml:"key_type"`
	EAB             EABEnvConfig      `yaml:"eab"`
	AutoRenew       bool              `yaml:"auto_renew"`
	CheckInterval   string            `yaml:"check_interval"`
	RenewBefore     string            `yaml:"renew_before"`
	PropagationWait string            `yaml:"propagation_wait"`
	Storage         ACMEStorageConfig `yaml:"storage"`
}

type ACMEStorageConfig struct {
	Type      string `yaml:"type"`
	Namespace string `yaml:"namespace"`
	Path      string `yaml:"path"`
}

// ToACMEConfig converts the YAML-based ACMEConfig to an acme.Config,
// resolving EAB credentials from environment variables and server URL from mode.
func (c *ACMEConfig) ToACMEConfig() *acme.Config {
	serverURL := c.Server
	if serverURL == "" {
		switch c.Mode {
		case "zerossl":
			serverURL = acme.ZeroSSLProduction()
		case "letsencrypt":
			serverURL = acme.LetsEncryptProduction()
		default:
			// Default to Let's Encrypt production if no mode or server specified
			serverURL = acme.LetsEncryptProduction()
		}
	}

	// Parse duration strings with defaults
	checkInterval := 24 * time.Hour
	if c.CheckInterval != "" {
		if d, err := time.ParseDuration(c.CheckInterval); err == nil {
			checkInterval = d
		}
	}

	renewBefore := 30 * 24 * time.Hour // 30 days
	if c.RenewBefore != "" {
		if d, err := time.ParseDuration(c.RenewBefore); err == nil {
			renewBefore = d
		}
	}

	propagationWait := 60 * time.Second
	if c.PropagationWait != "" {
		if d, err := time.ParseDuration(c.PropagationWait); err == nil {
			propagationWait = d
		}
	}

	// Set default key type if not specified
	keyType := c.KeyType
	if keyType == "" {
		keyType = "RSA2048"
	}

	return &acme.Config{
		ServerURL:       serverURL,
		Email:           c.Email,
		KeyType:         keyType,
		AutoRenew:       c.AutoRenew,
		CheckInterval:   checkInterval,
		RenewBefore:     renewBefore,
		PropagationWait: propagationWait,
		EAB: acme.EABConfig{
			KID:     os.Getenv(c.EAB.KidEnv),
			HMACKey: os.Getenv(c.EAB.HmacEnv),
		},
		Storage: acme.StorageConfig{
			Type:      c.Storage.Type,
			Namespace: c.Storage.Namespace,
			Path:      c.Storage.Path,
		},
	}
}
