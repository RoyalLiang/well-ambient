package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
	"well-ambient/internal/config"
	"well-ambient/internal/db"
	"well-ambient/internal/openaccess"
)

func main() {
	log.SetFlags(0)
	global := flag.NewFlagSet("open-access-admin", flag.ExitOnError)
	configPath := global.String("config", "config.yaml", "path to config file")
	_ = global.Parse(os.Args[1:])
	args := global.Args()
	if len(args) == 0 {
		usage()
		os.Exit(2)
	}
	service, closeDatabase, err := openService(*configPath)
	if err != nil {
		log.Fatal(err)
	}
	defer closeDatabase()
	ctx := context.Background()

	switch args[0] {
	case "source-create":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		id := flags.String("id", "", "stable source id")
		name := flags.String("name", "", "display name")
		owner := flags.String("owner", "", "owning team or contact")
		quota := flags.String("quota", openaccess.DefaultQuotaProfile, "quota profile")
		_ = flags.Parse(args[1:])
		value, err := service.CreateSource(ctx, *id, *name, *owner, *quota)
		output(value, err)
	case "source-status":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		id := flags.String("id", "", "source id")
		status := flags.String("status", "", "active or disabled")
		_ = flags.Parse(args[1:])
		output(map[string]string{"source_id": *id, "status": *status}, service.SetSourceStatus(ctx, *id, *status))
	case "credential-issue":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		sourceID := flags.String("source", "", "source id")
		ttlText := flags.String("ttl", "720h", "credential lifetime, for example 720h")
		_ = flags.Parse(args[1:])
		ttl, err := time.ParseDuration(strings.TrimSpace(*ttlText))
		if err != nil {
			log.Fatalf("invalid ttl: %v", err)
		}
		value, err := service.IssueCredential(ctx, *sourceID, ttl)
		output(value, err)
	case "credential-revoke":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		keyID := flags.String("key-id", "", "credential key id")
		_ = flags.Parse(args[1:])
		output(map[string]string{"key_id": *keyID, "status": openaccess.CredentialRevoked}, service.RevokeCredential(ctx, *keyID))
	case "credential-status":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		keyID := flags.String("key-id", "", "credential key id")
		status := flags.String("status", "", "active or disabled")
		_ = flags.Parse(args[1:])
		output(map[string]string{"key_id": *keyID, "status": *status}, service.SetCredentialStatus(ctx, *keyID, *status))
	case "credential-list":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		sourceID := flags.String("source", "", "optional source id")
		_ = flags.Parse(args[1:])
		value, err := service.Credentials(ctx, *sourceID)
		output(value, err)
	case "credential-show":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		keyID := flags.String("key-id", "", "credential key id")
		_ = flags.Parse(args[1:])
		value, err := service.Credential(ctx, *keyID)
		output(value, err)
	case "policy-activate":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		file := flags.String("file", "", "JSON or YAML policy file")
		actor := flags.String("actor", "open-access-admin", "audit actor")
		_ = flags.Parse(args[1:])
		spec, err := readPolicy(*file)
		if err != nil {
			log.Fatal(err)
		}
		value, err := service.ActivatePolicy(ctx, spec, *actor)
		output(value, err)
	case "binding-upsert":
		flags := flag.NewFlagSet(args[0], flag.ExitOnError)
		project := flags.String("project", "", "Jira project key")
		action := flags.String("action", "", "reassign or reschedule")
		connector := flags.String("connector", "jira-primary", "connector reference")
		executor := flags.String("executor", "jira-service", "executor reference")
		version := flags.Int("version", 1, "binding version")
		status := flags.String("status", openaccess.SourceActive, "active or disabled")
		_ = flags.Parse(args[1:])
		binding := db.JiraExecutionBinding{
			ProjectRef: *project, ActionClass: *action, ConnectorRef: *connector,
			ExecutorRef: *executor, Version: *version, Status: *status,
		}
		output(binding, service.UpsertExecutionBinding(ctx, binding))
	default:
		usage()
		os.Exit(2)
	}
}

func openService(configPath string) (*openaccess.Service, func(), error) {
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		return nil, nil, fmt.Errorf("load configuration: %w", err)
	}
	if cfg.Database.RequiresSetup() {
		return nil, nil, fmt.Errorf("database setup is incomplete")
	}
	databaseConfig, err := cfg.Database.Resolve()
	if err != nil {
		return nil, nil, fmt.Errorf("resolve database configuration: %w", err)
	}
	if err := db.Init(db.Options{
		Driver: databaseConfig.Driver, DSN: databaseConfig.DSN,
		AutoMigrate:                  databaseConfig.AutoMigrate,
		MaxOpenConnections:           databaseConfig.MaxOpenConnections,
		MaxIdleConnections:           databaseConfig.MaxIdleConnections,
		ConnectionMaxLifetimeMinutes: databaseConfig.ConnectionMaxLifetimeMinutes,
		ConnectionMaxIdleTimeMinutes: databaseConfig.ConnectionMaxIdleTimeMinutes,
	}); err != nil {
		return nil, nil, fmt.Errorf("open database: %w", err)
	}
	return openaccess.New(db.DB), func() { _ = db.Close() }, nil
}

func readPolicy(path string) (openaccess.PolicySpec, error) {
	if strings.TrimSpace(path) == "" {
		return openaccess.PolicySpec{}, fmt.Errorf("policy file is required")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return openaccess.PolicySpec{}, err
	}
	var spec openaccess.PolicySpec
	if err := yaml.Unmarshal(data, &spec); err != nil {
		return openaccess.PolicySpec{}, fmt.Errorf("parse policy file: %w", err)
	}
	return spec, nil
}

func output(value any, err error) {
	if err != nil {
		log.Fatal(err)
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		log.Fatal(err)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `usage: open-access-admin [-config path] <command> [flags]

commands:
  source-create
  source-status
  credential-issue
  credential-status
  credential-list
  credential-show
  credential-revoke
  policy-activate
  binding-upsert`)
}
