package main

import (
	"flag"
	"log"
	"os"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/server"
)

func main() {
	configPath := flag.String("config", "config.yaml", "path to config file")
	flag.Parse()

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

	if err := db.InitDB("well-ambient.db"); err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	srv := server.NewServer(cfg, *configPath)
	if err := srv.Start(); err != nil {
		log.Fatalf("Server startup failed: %v", err)
	}
}
