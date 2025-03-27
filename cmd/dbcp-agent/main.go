package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/virtlabs-io/dbcp-client/internal/config"
	"github.com/virtlabs-io/dbcp-client/internal/logger"
	"github.com/virtlabs-io/dbcp-client/internal/pkg"
	"github.com/virtlabs-io/dbcp-client/internal/system"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "config", "./configs/agent-config.yaml", "Path to configuration file")
	flag.StringVar(&configPath, "c", "./configs/agent-config.yaml", "Path to configuration file (shorthand)")
	flag.Parse()

	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Printf("Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Printf("Invalid config: %v\n", err)
		os.Exit(1)
	}

	logger.Init(logger.Options{
		Level:       cfg.LogLevel,
		Output:      cfg.LogOutput,
		LogFilePath: cfg.LogFilePath,
		MaxSizeMB:   cfg.LogMaxSizeMB,
		MaxBackups:  cfg.LogMaxBackups,
		MaxAgeDays:  cfg.LogMaxAgeDays,
	})
	logger.Info("Agent starting...")

	if err := cfg.Validate(); err != nil {
		logger.Error("Configuration validation failed: %v", err)
		os.Exit(1)
	}

	// Create necessary folders
	infraPaths := []string{
		cfg.Node.PostgreSQL.DataDir,
		cfg.Node.ETCD.DataDir,
		filepath.Dir(cfg.Node.ETCD.CertFile),
		filepath.Dir(cfg.Node.ETCD.KeyFile),
		filepath.Dir(cfg.Node.ETCD.CAFile),
		filepath.Dir(cfg.Node.Patroni.ConfigPath),
		cfg.Node.TmpPath,
	}
	if err := pkg.CreateDirs(infraPaths...); err != nil {
		logger.Error("Failed to create infrastructure directories: %v", err)
		os.Exit(1)
	}

	// Detect OS
	osInfo, err := system.DetectOS()
	if err != nil {
		logger.Error("OS detection failed: %v", err)
		os.Exit(1)
	}

	// PostgreSQL installation
	if err := pkg.InstallPostgreSQL(
		cfg,
		osInfo,
	); err != nil {
		logger.Error("PostgreSQL installation failed: %v", err)
		os.Exit(1)
	}

	// ETCD installation
	etcdRepo := cfg.Repositories.ETCD.Sources[cfg.Repositories.ETCD.Default]
	etcdRepoURL := etcdRepo["url"]
	if err := pkg.InstallETCD(cfg, etcdRepoURL); err != nil {
		logger.Error("ETCD installation failed: %v", err)
		os.Exit(1)
	}

	// Start ETCD cluster (bootstrap or join)
	if err := pkg.StartETCDCluster(cfg); err != nil {
		logger.Error("ETCD failed to start: %v", err)
		os.Exit(1)
	}

	logger.Info("Agent finished successfully.")
}
