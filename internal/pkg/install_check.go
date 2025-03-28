package pkg

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
	"github.com/virtlabs-io/dbcp-agent/internal/logger"
)

func IsPostgreSQLInstalled(cfg *config.AgentConfig) bool {
	binPath := cfg.Node.PostgreSQL.BinPath
	postgres := filepath.Join(binPath, "postgres")
	initdb := filepath.Join(binPath, "initdb")

	if _, err := os.Stat(postgres); err != nil {
		return false
	}
	if _, err := os.Stat(initdb); err != nil {
		return false
	}

	cmd := exec.Command(postgres, "--version")
	output, err := cmd.Output()
	if err != nil {
		logger.Warn("postgres version check failed: %v", err)
		return false
	}

	return strings.Contains(string(output), cfg.Node.PostgreSQL.Version)
}

func IsETCDInstalled(cfg *config.AgentConfig) bool {
	binPath := cfg.Node.ETCD.BinPath
	etcd := filepath.Join(binPath, "etcd")
	etcdctl := filepath.Join(binPath, "etcdctl")

	if _, err := os.Stat(etcd); err != nil {
		return false
	}
	if _, err := os.Stat(etcdctl); err != nil {
		return false
	}

	cmd := exec.Command(etcd, "--version")
	output, err := cmd.Output()
	if err != nil {
		logger.Warn("etcd version check failed: %v", err)
		return false
	}

	return strings.Contains(string(output), cfg.Node.ETCD.Version)
}

func IsPatroniInstalled(cfg *config.AgentConfig) bool {
	path, err := exec.LookPath("patroni")
	if err != nil {
		return false
	}

	cmd := exec.Command(path, "--version")
	output, err := cmd.Output()
	if err != nil {
		logger.Warn("patroni version check failed: %v", err)
		return false
	}

	return strings.Contains(string(output), cfg.Node.Patroni.Version)
}

// Public wrappers: We should use these in main.go or during orchestration

func ShouldInstallPostgreSQL(cfg *config.AgentConfig) bool {
	if IsPostgreSQLInstalled(cfg) {
		logger.Info("PostgreSQL already installed, skipping installation")
		return false
	}
	return true
}

func ShouldInstallETCD(cfg *config.AgentConfig) bool {
	if IsETCDInstalled(cfg) {
		logger.Info("ETCD already installed, skipping installation")
		return false
	}
	return true
}

func ShouldInstallPatroni(cfg *config.AgentConfig) bool {
	if IsPatroniInstalled(cfg) {
		logger.Info("Patroni already installed, skipping installation")
		return false
	}
	return true
}
