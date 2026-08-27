package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/server"
)

var (
	version   = "dev"
	commit    = "unknown"
	buildTime = "unknown"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	httpHost := flag.String("http-host", "", "override HTTP listen host after runtime configuration restore")
	httpPort := flag.Int("http-port", 0, "override HTTP listen port after runtime configuration restore")
	migrateOnly := flag.Bool("migrate-only", false, "apply database migrations and exit")
	skipMigrate := flag.Bool("skip-migrate", false, "start without applying database migrations")
	flag.Parse()
	if *migrateOnly && *skipMigrate {
		log.Fatal("--migrate-only and --skip-migrate cannot be used together")
	}

	// If config.yaml does not exist, look for config.example.yaml
	if _, err := os.Stat(*configPath); os.IsNotExist(err) {
		if _, err := os.Stat("config.example.yaml"); err == nil {
			log.Printf("Config file %s not found, falling back to config.example.yaml", *configPath)
			*configPath = "config.example.yaml"
		} else {
			log.Fatalf("Configuration file not found. Please create config.yaml based on config.example.yaml")
		}
	}

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config from %s: %v", *configPath, err)
	}
	if err := applyHTTPAddressOverride(cfg, *httpHost, *httpPort); err != nil {
		log.Fatalf("Invalid HTTP address override: %v", err)
	}
	server.SetBuildInfo(version, commit, buildTime)
	if cfg.Database.RequiresSetup() {
		if *migrateOnly {
			log.Fatal("database setup is incomplete; finish the browser setup before running --migrate-only")
		}
		setupToken, err := server.ProvisionSetupToken(os.Getenv(server.SetupTokenEnvironment))
		if err != nil {
			log.Fatalf("Failed to prepare database setup token: %v", err)
		}
		setupServer, err := server.NewSetupServer(cfg, *configPath, setupToken.Token)
		if err != nil {
			if cleanupErr := setupToken.Cleanup(); cleanupErr != nil {
				log.Printf("Warning: %v", cleanupErr)
			}
			log.Fatalf("Failed to start database setup mode: %v", err)
		}
		if setupToken.Generated {
			log.Printf("Generated one-time database setup token: %s", setupToken.Token)
			log.Printf("Database setup token saved to %s (permissions 0600; removed when setup mode exits)", setupToken.FilePath)
		}
		log.Printf("Database setup mode is active; normal APIs remain disabled until PostgreSQL is configured")
		startErr := setupServer.Start()
		if cleanupErr := setupToken.Cleanup(); cleanupErr != nil {
			log.Printf("Warning: %v", cleanupErr)
		}
		if startErr != nil {
			log.Fatalf("Database setup server failed: %v", startErr)
		}
		if cfg.Database.RequiresSetup() {
			log.Printf("Database setup server stopped before configuration was completed")
		} else {
			log.Printf("Database setup completed; exiting so the service manager can restart normal mode")
		}
		return
	}

	databaseConfig, err := cfg.Database.Resolve()
	if err != nil {
		log.Fatalf("Invalid database configuration: %v", err)
	}
	autoMigrate := databaseConfig.AutoMigrate
	if *migrateOnly {
		autoMigrate = true
	}
	if *skipMigrate {
		autoMigrate = false
	}
	cfg.Database.AutoMigrate = &autoMigrate
	if err := db.Init(db.Options{
		Driver:                       databaseConfig.Driver,
		DSN:                          databaseConfig.DSN,
		AutoMigrate:                  autoMigrate,
		MaxOpenConnections:           databaseConfig.MaxOpenConnections,
		MaxIdleConnections:           databaseConfig.MaxIdleConnections,
		ConnectionMaxLifetimeMinutes: databaseConfig.ConnectionMaxLifetimeMinutes,
		ConnectionMaxIdleTimeMinutes: databaseConfig.ConnectionMaxIdleTimeMinutes,
	}); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Printf("Failed to close database: %v", err)
		}
	}()
	if autoMigrate {
		if err := server.MigrateReadModels(db.DB); err != nil {
			log.Fatalf("Failed to migrate read models: %v", err)
		}
	}
	if *migrateOnly {
		if err := server.BootstrapVersionedConfig(cfg); err != nil {
			log.Fatalf("Database migration completed but configuration synchronization failed: %v", err)
		}
		log.Printf("Database migration and configuration synchronization completed successfully using %s", databaseConfig.Driver)
		return
	}
	if err := server.BootstrapVersionedConfig(cfg); err != nil {
		log.Fatalf("Failed to initialize versioned configuration: %v", err)
	}
	if err := applyHTTPAddressOverride(cfg, *httpHost, *httpPort); err != nil {
		log.Fatalf("Invalid HTTP address override after runtime configuration restore: %v", err)
	}

	srv := server.NewServer(cfg, *configPath)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}

func applyHTTPAddressOverride(cfg *config.Config, host string, port int) error {
	if cfg == nil {
		return fmt.Errorf("config is required")
	}
	host = strings.TrimSpace(host)
	if port < 0 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if host != "" {
		cfg.Server.Host = host
	}
	if port != 0 {
		cfg.Server.Port = port
	}
	return nil
}
