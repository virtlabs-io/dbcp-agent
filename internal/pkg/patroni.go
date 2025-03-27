package pkg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"text/template"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
	"github.com/virtlabs-io/dbcp-agent/internal/logger"
	"github.com/virtlabs-io/dbcp-agent/internal/system"
)

type PatroniTemplateData struct {
	Cluster   config.ClusterConfig
	Node      config.NodeConfig
	Host      string
	EtcdHost  string
	PGPort    int
	PGDataDir string
	PGBinDir  string
	APIListen string

	SuperUser   config.PatroniUser
	Replication config.PatroniUser
	AdminUser   config.PatroniUser
	InitDB      []string
	UsePGRewind bool
	UseSlots    bool
	DCS         config.DCSSettings
}

func InstallPatroni(cfg *config.AgentConfig) error {
	logger.Info("Installing Patroni...")

	osInfo, err := system.DetectOS()
	if err != nil {
		return fmt.Errorf("failed to detect OS: %w", err)
	}

	switch osInfo.Family {
	case "debian":
		return installViaApt("patroni")
	case "rhel", "fedora", "centos", "rocky", "almalinux", "oracle":
		return installViaPip("patroni[etcd]")
	default:
		return fmt.Errorf("unsupported OS family: %s", osInfo.Family)
	}
}

func installViaApt(pkg string) error {
	logger.Info("Installing Patroni using apt...")
	if output, err := exec.Command("apt-get", "update").CombinedOutput(); err != nil {
		logger.Error("apt-get update failed: %s", string(output))
		return err
	}
	cmd := exec.Command("apt-get", "-y", "install", pkg)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Error("apt-get install failed: %s", string(output))
		return err
	}
	return nil
}

func installViaPip(pkg string) error {
	logger.Info("Installing Patroni using pip...")
	cmd := exec.Command("python3", "-m", "pip", "install", pkg)
	if output, err := cmd.CombinedOutput(); err != nil {
		logger.Error("pip install failed: %s", string(output))
		return err
	}
	return nil
}

func GeneratePatroniConfig(cfg *config.AgentConfig, templatePath string) error {
	p := cfg.Node.Patroni

	tmplData := PatroniTemplateData{
		Node:        cfg.Node,
		Cluster:     cfg.Cluster,
		Host:        cfg.Node.Host,
		EtcdHost:    p.EtcdHost,
		PGPort:      p.PGPort,
		PGDataDir:   cfg.Node.PostgreSQL.DataDir,
		PGBinDir:    cfg.Node.PostgreSQL.BinPath,
		APIListen:   p.APIListen,
		SuperUser:   p.Superuser,
		Replication: p.Replication,
		AdminUser:   p.AdminUser,
		InitDB:      p.InitDB,
		UsePGRewind: p.UsePGRewind,
		UseSlots:    p.UseSlots,
		DCS:         p.DCS,
	}

	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		return fmt.Errorf("failed to load template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, tmplData); err != nil {
		return fmt.Errorf("failed to execute template: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(p.ConfigPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	if err := os.WriteFile(p.ConfigPath, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("failed to write patroni.yml: %w", err)
	}

	logger.Info("Patroni configuration written to %s", p.ConfigPath)
	return nil
}

func StartPatroni(cfg *config.AgentConfig) error {
	configPath := cfg.Node.Patroni.ConfigPath
	binary := "patroni"

	logger.Info("Starting Patroni using config: %s", configPath)

	cmd := exec.Command(binary, configPath)

	// Pipe output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// Optional: set env or working dir if needed
	// cmd.Env = os.Environ()
	// cmd.Dir = filepath.Dir(configPath)

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Patroni: %w", err)
	}

	logger.Info("Patroni process started with PID: %d", cmd.Process.Pid)
	return nil
}

func StartPatroniDaemon(cfg *config.AgentConfig) error {
	configPath := cfg.Node.Patroni.ConfigPath
	binary := "patroni"

	logger.Info("Launching Patroni as daemon with config: %s", configPath)

	cmd := exec.Command(binary, configPath)

	// Detach process group (make Patroni independent from agent)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	// Redirect output
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Patroni daemon: %w", err)
	}

	logger.Info("Patroni started with PID: %d", cmd.Process.Pid)
	return nil
}
