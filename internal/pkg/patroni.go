package pkg

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
	EtcdHosts string // comma-separated list of etcd endpoints for etcd3

	// Patroni API
	APIListen string

	// PostgreSQL details
	PGPort    int
	PGDataDir string
	PGBinDir  string

	// Users
	PGUsers     map[string]config.PostgresUser // All defined PostgreSQL users
	SuperUser   config.UserCredentials         // for authentication.superuser
	Replication config.UserCredentials         // for authentication.replication

	// Init and DCS
	InitDB      []string         // initdb steps
	PGHBA       []string         // initdb steps
	UsePGRewind bool             // from parameters
	UseSlots    bool             // from parameters
	DCS         config.DCSConfig // ttl, loop_wait, etc.

	// PostgreSQL runtime parameters
	Parameters config.PostgresSettings
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

func GeneratePatroniConfig(cfg *config.AgentConfig) error {
	p := cfg.Node.Patroni

	// Construct ETCD hosts list from all cluster nodes
	etcdPort := cfg.Node.ETCD.ClientPort
	var etcdHosts []string
	for _, node := range cfg.Cluster.Nodes {
		etcdHosts = append(etcdHosts, fmt.Sprintf(" %s:%d", node.Host, etcdPort))
	}

	var initDB []string
	for _, item := range cfg.Node.PostgreSQL.InitDB {
		for k, v := range item {
			if v == "" {
				initDB = append(initDB, k)
			} else {
				initDB = append(initDB, fmt.Sprintf("%s: %s", k, v))
			}
		}
	}

	// var pgHBA []string
	// for _, item := range cfg.Node.PostgreSQL.PGHBA {
	// 	pgHBA = append(pgHBA, item)
	// }

	tmplData := PatroniTemplateData{
		Cluster:   cfg.Cluster,
		Node:      cfg.Node,
		Host:      cfg.Node.Host,
		EtcdHosts: strings.Join(etcdHosts, ","), // pre-formatted string: "http://host1:2379","http://host2:2379"
		APIListen: cfg.Node.Patroni.APIListen,
		PGPort:    cfg.Node.PostgreSQL.Parameters.Port,
		PGDataDir: cfg.Node.PostgreSQL.DataDir,
		PGBinDir:  cfg.Node.PostgreSQL.BinPath,

		// Users
		PGUsers:     cfg.Node.PostgreSQL.Users,
		SuperUser:   cfg.Node.Patroni.Authentication.Superuser,
		Replication: cfg.Node.Patroni.Authentication.Replication,

		// Init & cluster settings
		InitDB:      initDB,
		PGHBA:       cfg.Node.PostgreSQL.PGHBA,
		UsePGRewind: cfg.Node.PostgreSQL.Parameters.UsePGRewind,
		UseSlots:    cfg.Node.PostgreSQL.Parameters.UseSlots,
		DCS:         cfg.Node.Patroni.DCS,
		Parameters:  cfg.Node.PostgreSQL.Parameters,
	}

	tmpl, err := template.ParseFiles(cfg.Node.Patroni.TemplatePath)
	logger.Debug("Template path: %s", cfg.Node.Patroni.TemplatePath)
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

func StartPatroni2(cfg *config.AgentConfig) error {
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

func StartPatroni(cfg *config.AgentConfig) error {
	bin := "patroni"
	user := cfg.Node.User
	configPath := cfg.Node.Patroni.ConfigPath

	if configPath == "" {
		return fmt.Errorf("patroni.config_path must be defined")
	}

	if _, err := os.Stat(configPath); err != nil {
		return fmt.Errorf("patroni config not found at %s: %v", configPath, err)
	}

	logger.Info("Starting Patroni using user: %s", user)
	cmd := exec.Command("sudo", "-u", user, bin, configPath)

	logFile := filepath.Join(cfg.Node.TmpPath, "patroni.log")
	log, err := os.OpenFile(logFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open patroni log file: %v", err)
	}
	cmd.Stdout = log
	cmd.Stderr = log

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Patroni: %w", err)
	}

	logger.Info("Patroni started with PID %d", cmd.Process.Pid)

	// Set PG data dir to 0700
	dataDir := cfg.Node.PostgreSQL.DataDir
	if err := os.Chmod(dataDir, 0700); err != nil {
		logger.Warn("Failed to chmod PostgreSQL data dir (%s) to 0700: %v", dataDir, err)
	} else {
		logger.Info("PostgreSQL data directory permissions set to 0700")
	}

	return nil
}

func StartPatroniDaemon(cfg *config.AgentConfig) error {
	configPath := cfg.Node.Patroni.ConfigPath
	binary := "patroni"

	logger.Info("Launching Patroni as daemon with config: %s", configPath)

	cmd := exec.Command(binary, configPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start Patroni daemon: %w", err)
	}

	logger.Info("Patroni started with PID: %d", cmd.Process.Pid)

	// Set PG data dir to 0700
	dataDir := cfg.Node.PostgreSQL.DataDir
	if err := os.Chmod(dataDir, 0700); err != nil {
		logger.Warn("Failed to chmod PostgreSQL data dir (%s) to 0700: %v", dataDir, err)
	} else {
		logger.Info("PostgreSQL data directory permissions set to 0700")
	}

	return nil
}
