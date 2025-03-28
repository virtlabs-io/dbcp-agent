package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
	"github.com/virtlabs-io/dbcp-agent/internal/logger"
	"github.com/virtlabs-io/dbcp-agent/internal/pkg"
	"github.com/virtlabs-io/dbcp-agent/internal/system"
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
	if err := pkg.CreateDirs(cfg, infraPaths...); err != nil {
		logger.Error("Failed to create infrastructure directories: %v", err)
		os.Exit(1)
	}

	// Detect OS
	osInfo, err := system.DetectOS()
	if err != nil {
		logger.Error("OS detection failed: %v", err)
		os.Exit(1)
	}

	// This is the installation block
	// TODO: We need to verify if the package is already installed before we try to install it, or else every time we start the application it will attemp to install
	{
		// PostgreSQL installation
		if pkg.ShouldInstallPostgreSQL(cfg) {
			logger.Info("Installing PostgreSQL...")
			if err := pkg.InstallPostgreSQL(cfg, osInfo); err != nil {
				logger.Error("PostgreSQL installation failed: %v", err)
				os.Exit(1)
			}
		} else {
			logger.Info("PostgreSQL already installed and version matched.")
		}

		// ETCD installation
		if pkg.ShouldInstallETCD(cfg) {
			logger.Info("Installing ETCD...")
			repoURL := cfg.Repositories.ETCD.Sources[cfg.Repositories.ETCD.Default]["url"]
			if err := pkg.InstallETCD(cfg, repoURL); err != nil {
				logger.Error("ETCD installation failed: %v", err)
				os.Exit(1)
			}
		} else {
			logger.Info("ETCD already installed and version matched.")
		}

		// Patroni installation and config
		if pkg.ShouldInstallPatroni(cfg) {
			logger.Info("Installing Patroni...")
			if err := pkg.InstallPatroni(cfg); err != nil {
				logger.Error("Patroni installation failed: %v", err)
				os.Exit(1)
			}
		} else {
			logger.Info("Patroni already installed and version matched.")
		}
	}

	// Initialization block
	{
		// Start ETCD cluster (bootstrap or join)
		logger.Info("Starting ETCD...")
		if err := pkg.StartETCD(cfg); err != nil {
			logger.Error("Failed to start ETCD cluster: %v", err)
			os.Exit(1)
		}

		// Patroni configuration and startup
		logger.Info("Generating Patroni config...")
		if err := pkg.GeneratePatroniConfig(cfg); err != nil {
			logger.Error("Failed to generate Patroni config: %v", err)
			os.Exit(1)
		}

		logger.Info("Starting Patroni...")
		if err := pkg.StartPatroni(cfg); err != nil {
			logger.Error("Failed to start Patroni: %v", err)
			os.Exit(1)
		}

	}

	// Handle shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	logger.Info("Agent finished successfully.")
}
