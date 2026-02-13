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

	return nil
}

// Config represents the application configuration
type Config struct {
	DNS     DNSConfig     `yaml:"dns"`
	GeoIP   GeoIPConfig   `yaml:"geoip"`
	Storage StorageConfig `yaml:"storage"`
	HTTP    HTTPConfig    `yaml:"http"`
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
