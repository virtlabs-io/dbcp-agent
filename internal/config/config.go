package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/virtlabs-io/dbcp-client/internal/logger"
	"gopkg.in/yaml.v3"
)

type AgentConfig struct {
	LogLevel      string `yaml:"log_level"`
	LogOutput     string `yaml:"log_output"`
	LogFilePath   string `yaml:"log_file_path"`
	LogMaxSizeMB  int    `yaml:"log_max_size_mb"`
	LogMaxBackups int    `yaml:"log_max_backups"`
	LogMaxAgeDays int    `yaml:"log_max_age_days"`

	Node         NodeConfig    `yaml:"node"`
	Cluster      ClusterConfig `yaml:"cluster"`
	Repositories Repositories  `yaml:"repositories"`
}

type NodeConfig struct {
	Host                 string           `yaml:"host"`
	Role                 string           `yaml:"role"`
	TmpPath              string           `yaml:"tmp_path"`
	AllowRestartServices bool             `yaml:"allow_restart_services"`
	PostgreSQL           PostgreSQLConfig `yaml:"postgresql"`
	ETCD                 EtcdConfig       `yaml:"etcd"`
	Patroni              PatroniConfig    `yaml:"patroni"`
}

type PostgreSQLConfig struct {
	Version string `yaml:"version"`
	DataDir string `yaml:"data_dir"`
	BinPath string `yaml:"bin_path"`
	User    string `yaml:"user"`
}

type EtcdConfig struct {
	Version     string `yaml:"version"`
	DataDir     string `yaml:"data_dir"`
	BinPath     string `yaml:"bin_path"`
	CertFile    string `yaml:"cert_file"`
	KeyFile     string `yaml:"key_file"`
	CAFile      string `yaml:"ca_file"`
	PeerName    string `yaml:"peer_name"`
	PeerPort    int    `yaml:"peer_port"`
	ClientPort  int    `yaml:"client_port"`
	ClusterMode string `yaml:"cluster_mode"`
}

type PatroniConfig struct {
	ConfigPath string `yaml:"config_path"`
}

type ClusterConfig struct {
	Name  string        `yaml:"name"`
	Nodes []ClusterNode `yaml:"nodes"`
}

type ClusterNode struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
}

type Repositories struct {
	PostgreSQL RepoEntry `yaml:"postgresql"`
	ETCD       RepoEntry `yaml:"etcd"`
}

type RepoEntry struct {
	Default string                       `yaml:"default"`
	Sources map[string]map[string]string `yaml:"sources"`
}

func Load(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return &cfg, nil
}

func (cfg *AgentConfig) Validate() error {
	if cfg.Node.Host == "" || cfg.Node.Role == "" {
		return fmt.Errorf("node.host and node.role are required")
	}

	if cfg.Node.TmpPath == "" {
		return fmt.Errorf("node.tmp_path is required")
	}

	// PostgreSQL
	pg := cfg.Node.PostgreSQL
	if pg.Version == "" || pg.DataDir == "" || pg.User == "" {
		return fmt.Errorf("postgresql.version, data_dir, and user are required")
	}
	if pg.BinPath == "" {
		defaultPath := guessPostgresBinPath()
		logger.Warn("postgresql.bin_path not specified — using default: %s", defaultPath)
		cfg.Node.PostgreSQL.BinPath = defaultPath
	}

	// ETCD
	etcd := cfg.Node.ETCD
	if etcd.Version == "" || etcd.DataDir == "" || etcd.PeerPort == 0 || etcd.ClientPort == 0 {
		return fmt.Errorf("etcd.version, data_dir, peer_port, and client_port are required")
	}
	if etcd.PeerName == "" {
		return fmt.Errorf("etcd.peer_name is required")
	}

	// Likely better to have a parameter to force or not TLS, but for now I'll leave it commented out
	// if etcd.CertFile == "" || etcd.KeyFile == "" || etcd.CAFile == "" {
	// 	return fmt.Errorf("etcd cert_file, key_file, and ca_file are required")
	// }

	if etcd.BinPath == "" {
		logger.Warn("etcd.bin_path not specified — using default: /usr/local/bin")
		cfg.Node.ETCD.BinPath = "/usr/local/bin"
	}
	if etcd.ClusterMode == "" {
		cfg.Node.ETCD.ClusterMode = "bootstrap"
		logger.Warn("etcd.cluster_mode not set, defaulting to 'bootstrap'")
	} else if etcd.ClusterMode != "bootstrap" && etcd.ClusterMode != "join" {
		return fmt.Errorf("invalid etcd.cluster_mode: must be 'bootstrap' or 'join'")
	}

	// Cluster nodes
	if cfg.Cluster.Name == "" || len(cfg.Cluster.Nodes) == 0 {
		return fmt.Errorf("cluster.name and at least one node are required")
	}
	for _, node := range cfg.Cluster.Nodes {
		if node.Name == "" || node.Host == "" {
			return fmt.Errorf("each cluster node must have name and host")
		}
	}

	// Repositories
	if repo := cfg.Repositories.PostgreSQL.Sources[cfg.Repositories.PostgreSQL.Default]; repo == nil {
		return fmt.Errorf("postgresql repositories not found for default: %s", cfg.Repositories.PostgreSQL.Default)
	}
	if repo := cfg.Repositories.ETCD.Sources[cfg.Repositories.ETCD.Default]; repo == nil {
		return fmt.Errorf("etcd repositories not found for default: %s", cfg.Repositories.ETCD.Default)
	}

	return nil
}

func guessPostgresBinPath() string {
	osRelease, _ := os.ReadFile("/etc/os-release")
	content := string(osRelease)

	switch {
	case strings.Contains(content, "ID=debian"), strings.Contains(content, "ID=ubuntu"):
		return "/usr/lib/postgresql/16/bin"
	case strings.Contains(content, "ID=fedora"),
		strings.Contains(content, "ID=centos"),
		strings.Contains(content, "ID=rocky"),
		strings.Contains(content, "ID=almalinux"),
		strings.Contains(content, "ID=rhel"),
		strings.Contains(content, "ID=oracle"):
		return "/usr/pgsql-16/bin"
	default:
		return "/usr/local/pgsql/bin"
	}
}
