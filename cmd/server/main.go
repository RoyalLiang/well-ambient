package main

import (
	"flag"
	"log"
	"os"
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
	server.SetBuildInfo(version, commit, buildTime)
	if cfg.Database.RequiresSetup() {
		if *migrateOnly {
			log.Fatal("database setup is incomplete; finish the browser setup before running --migrate-only")
		}
		setupServer, err := server.NewSetupServer(cfg, *configPath, os.Getenv(server.SetupTokenEnvironment))
		if err != nil {
			log.Fatalf("Failed to start database setup mode: %v", err)
		}
		log.Printf("Database setup mode is active; normal APIs remain disabled until PostgreSQL is configured")
		if err := setupServer.Start(); err != nil {
			log.Fatalf("Database setup server failed: %v", err)
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
		log.Printf("Database migration completed successfully using %s", databaseConfig.Driver)
		return
	}
	if err := server.BootstrapVersionedConfig(cfg); err != nil {
		log.Fatalf("Failed to initialize versioned configuration: %v", err)
	}

	srv := server.NewServer(cfg, *configPath)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}
