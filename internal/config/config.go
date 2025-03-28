package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/virtlabs-io/dbcp-agent/internal/logger"
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
	Name                 string           `yaml:"name"`
	Host                 string           `yaml:"host"`
	Role                 string           `yaml:"role"`
	User                 string           `yaml:"os_user"` // OS-level user (e.g., "vagrant")
	TmpPath              string           `yaml:"tmp_path"`
	AllowRestartServices bool             `yaml:"allow_restart_services"`
	PostgreSQL           PostgreSQLConfig `yaml:"postgresql"`
	ETCD                 EtcdConfig       `yaml:"etcd"`
	Patroni              PatroniConfig    `yaml:"patroni"`
}

// --------------- PostgreSQL Configuration
type PostgreSQLConfig struct {
	Version    string                  `yaml:"version"`
	DataDir    string                  `yaml:"data_dir"`
	BinPath    string                  `yaml:"bin_path"`
	Users      map[string]PostgresUser `yaml:"users"`
	Parameters PostgresSettings        `yaml:"parameters"`
	InitDB     []map[string]string     `yaml:"initdb"`
	PGHBA      []string                `yaml:"pg_hba"`
}

type PostgresUser struct {
	Password string   `yaml:"password"`
	Options  []string `yaml:"options"` // e.g., createrole, createdb, superuser
}

type PostgresSettings struct {
	Port                    int    `yaml:"port"`
	MaxConnections          int    `yaml:"max_connections"`
	UsePGRewind             bool   `yaml:"use_pg_rewind"`
	UseSlots                bool   `yaml:"use_slots"`
	WALLevel                string `yaml:"wal_level"`
	HotStandby              string `yaml:"hot_standby"`
	SynchronousCommit       string `yaml:"synchronous_commit"`
	SynchronousStandbyNames string `yaml:"synchronous_standby_names"`
}

// --------------- ETCD Configuration
type EtcdConfig struct {
	Version     string `yaml:"version"`
	DataDir     string `yaml:"data_dir"`
	BinPath     string `yaml:"bin_path"`
	CertFile    string `yaml:"cert_file"`
	KeyFile     string `yaml:"key_file"`
	CAFile      string `yaml:"ca_file"`
	PeerPort    int    `yaml:"peer_port"`
	ClientPort  int    `yaml:"client_port"`
	ClusterMode string `yaml:"cluster_mode"`
}

// --------------- Patroni
type PatroniConfig struct {
	Version              string            `yaml:"version"`
	Namespace            string            `yaml:"namespace"`
	APIListen            string            `yaml:"api_listen"`
	Port                 int               `yaml:"port"`
	ConfigPath           string            `yaml:"config_path"`
	TemplatePath         string            `yaml:"template_path"`
	DCS                  DCSConfig         `yaml:"dcs"`
	Authentication       PatroniAuthConfig `yaml:"authentication"`
	CreateReplicaMethods []string          `yaml:"create_replica_methods"`
	Tags                 PatroniTags       `yaml:"tags"`
}

type PatroniAuthConfig struct {
	Replication UserCredentials `yaml:"replication"`
	Superuser   UserCredentials `yaml:"superuser"`
}

type UserCredentials struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type DCSConfig struct {
	TTL                  int `yaml:"ttl"`
	LoopWait             int `yaml:"loop_wait"`
	RetryTimeout         int `yaml:"retry_timeout"`
	MaximumLagOnFailover int `yaml:"maximum_lag_on_failover"`
}

type PatroniTags struct {
	NoFailover    bool `yaml:"nofailover"`
	NoLoadBalance bool `yaml:"noloadbalance"`
	CloneFrom     bool `yaml:"clonefrom"`
	NoSync        bool `yaml:"nosync"`
}

// ---------------
type ClusterConfig struct {
	Name  string        `yaml:"name"`
	Nodes []ClusterNode `yaml:"nodes"`
}

type ClusterNode struct {
	Name string `yaml:"name"`
	Host string `yaml:"host"`
}

// ---------------
type Repositories struct {
	PostgreSQL RepoEntry `yaml:"postgresql"`
	ETCD       RepoEntry `yaml:"etcd"`
}

type RepoEntry struct {
	Default string                       `yaml:"default"`
	Sources map[string]map[string]string `yaml:"sources"`
}

// -----------------------

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
	if err := cfg.validateNode(); err != nil {
		return err
	}

	if err := cfg.validatePostgreSQL(); err != nil {
		return err
	}

	if err := cfg.validaPatroni(); err != nil {
		return err
	}

	if err := cfg.validateETCD(); err != nil {
		return err
	}

	if err := cfg.validateCluster(); err != nil {
		return err
	}

	if err := cfg.validateRepositories(); err != nil {
		return err
	}

	return nil
}

func (cfg *AgentConfig) validateNode() error {
	if cfg.Node.Name == "" {
		return fmt.Errorf("node.name is required")
	}

	if cfg.Node.Host == "" || cfg.Node.Role == "" {
		return fmt.Errorf("node.host and node.role are required")
	}

	if cfg.Node.User == "" {
		return fmt.Errorf("node.user is required (OS-level user)")
	}

	if cfg.Node.TmpPath == "" {
		return fmt.Errorf("node.tmp_path is required")
	}

	return nil
}

func (cfg *AgentConfig) validatePostgreSQL() error {
	pg := cfg.Node.PostgreSQL

	if pg.Version == "" {
		return fmt.Errorf("postgresql.version is required")
	}

	if pg.DataDir == "" {
		return fmt.Errorf("postgresql.data_dir is required")
	}

	if pg.BinPath == "" {
		logger.Warn("postgresql.bin_path is not specified, will use OS-detected default")
		pg.BinPath = guessPostgresBinPath()
	}

	// Validate parameters
	params := pg.Parameters
	if params.Port == 0 {
		return fmt.Errorf("postgresql.parameters.port must be a valid port number")
	}

	if params.WALLevel == "" {
		return fmt.Errorf("postgresql.parameters.wal_level is required")
	}

	if params.HotStandby == "" {
		return fmt.Errorf("postgresql.parameters.hot_standby is required")
	}

	if params.SynchronousCommit == "" {
		return fmt.Errorf("postgresql.parameters.synchronous_commit is required")
	}

	if params.SynchronousStandbyNames == "" {
		return fmt.Errorf("postgresql.parameters.synchronous_standby_names is required")
	}

	// Validate users
	if len(pg.Users) == 0 {
		return fmt.Errorf("at least one postgresql.users entry is required")
	}

	hasSuperuser := false
	for username, user := range pg.Users {
		if user.Password == "" {
			return fmt.Errorf("postgresql.users[%s] is missing a password", username)
		}

		for _, opt := range user.Options {
			if opt == "superuser" {
				hasSuperuser = true
			}
		}
	}

	if !hasSuperuser {
		return fmt.Errorf("at least one user must have the 'superuser' option")
	}

	// Validate initdb
	for i, entry := range pg.InitDB {
		if len(entry) == 0 {
			return fmt.Errorf("patroni.initdb[%d] must not be empty", i)
		}
	}

	// Validate pg_hba
	if len(pg.PGHBA) == 0 {
		return fmt.Errorf("patroni.pg_hba must contain at least one entry")
	}

	return nil
}

func (cfg *AgentConfig) validaPatroni() error {
	// PatroniConfig
	p := cfg.Node.Patroni

	if p.APIListen == "" {
		return fmt.Errorf("patroni.api_listen is required")
	}

	if p.Port == 0 {
		return fmt.Errorf("patroni.port must be a valid port number")
	}

	if p.ConfigPath == "" {
		return fmt.Errorf("patroni.config_path is required")
	}

	if p.TemplatePath == "" {
		return fmt.Errorf("patroni.template_path is required")
	}

	// Validate DCS settings
	if p.DCS.TTL <= 0 {
		return fmt.Errorf("patroni.dcs.ttl must be greater than 0")
	}

	if p.DCS.LoopWait <= 0 {
		return fmt.Errorf("patroni.dcs.loop_wait must be greater than 0")
	}

	if p.DCS.RetryTimeout <= 0 {
		return fmt.Errorf("patroni.dcs.retry_timeout must be greater than 0")
	}

	if p.DCS.MaximumLagOnFailover < 0 {
		return fmt.Errorf("patroni.dcs.maximum_lag_on_failover must be non-negative")
	}

	// Validate authentication
	if p.Authentication.Superuser.Username == "" || p.Authentication.Superuser.Password == "" {
		return fmt.Errorf("patroni.authentication.superuser.username and password are required")
	}

	if p.Authentication.Replication.Username == "" || p.Authentication.Replication.Password == "" {
		return fmt.Errorf("patroni.authentication.replication.username and password are required")
	}

	return nil
}

func (cfg *AgentConfig) validateETCD() error {
	etcd := cfg.Node.ETCD
	if etcd.Version == "" || etcd.DataDir == "" || etcd.PeerPort == 0 || etcd.ClientPort == 0 {
		return fmt.Errorf("etcd.version, data_dir, peer_port, and client_port are required")
	}

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

	return nil
}

func (cfg *AgentConfig) validateCluster() error {
	if cfg.Cluster.Name == "" || len(cfg.Cluster.Nodes) == 0 {
		return fmt.Errorf("cluster.name and at least one node are required")
	}

	for _, node := range cfg.Cluster.Nodes {
		if node.Name == "" || node.Host == "" {
			return fmt.Errorf("each cluster node must have name and host")
		}
	}

	return nil
}

func (cfg *AgentConfig) validateRepositories() error {
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
