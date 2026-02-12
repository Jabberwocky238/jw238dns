package main

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	"jabberwocky238/jw238dns/dns"
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

	// Initialize storage
	store := storage.NewMemoryStorage()

	// Load initial records from file if configured
	if config.Storage.Type == "file" {
		loader := storage.NewJSONFileLoader(config.Storage.File.Path, store)
		records, err := loader.Load()
		if err != nil {
			slog.Warn("Failed to load initial records", "error", err)
		} else {
			ctx := context.Background()
			for _, record := range records {
				if err := store.Create(ctx, record); err != nil {
					slog.Warn("Failed to create record", "name", record.Name, "error", err)
				}
			}
			slog.Info("Loaded initial records", "count", len(records))
		}
	}

	// Initialize DNS backend with config
	backendConfig := dns.DefaultBackendConfig()
	if config.GeoIP.Enabled {
		backendConfig.EnableGeoIP = true
		backendConfig.MMDBPath = config.GeoIP.MMDBPath
	}

	backend := dns.NewBackend(store, backendConfig)
	defer backend.Close()
	frontend := dns.NewFrontend(backend)

	// Create DNS handler
	dnsHandler := &DNSHandler{frontend: frontend}

	// Start DNS servers
	_, cancel := context.WithCancel(context.Background())
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

// Config represents the application configuration
type Config struct {
	DNS     DNSConfig     `yaml:"dns"`
	GeoIP   GeoIPConfig   `yaml:"geoip"`
	Storage StorageConfig `yaml:"storage"`
	HTTP    HTTPConfig    `yaml:"http"`
	ACME    ACMEConfig    `yaml:"acme"`
}

type DNSConfig struct {
	Listen     string `yaml:"listen"`
	TCPEnabled bool   `yaml:"tcp_enabled"`
	UDPEnabled bool   `yaml:"udp_enabled"`
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

type ACMEConfig struct {
	Enabled   bool              `yaml:"enabled"`
	Server    string            `yaml:"server"`
	Email     string            `yaml:"email"`
	AutoRenew bool              `yaml:"auto_renew"`
	Storage   ACMEStorageConfig `yaml:"storage"`
}

type ACMEStorageConfig struct {
	Type      string `yaml:"type"`
	Namespace string `yaml:"namespace"`
	Path      string `yaml:"path"`
}
