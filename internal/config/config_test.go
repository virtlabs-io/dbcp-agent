package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const testYAML = `
############ Agent Log Configuration
log_level: "debug"
log_output: "stdout" # or "file"
log_file_path: "/var/log/dbcp-agent.log"
log_max_size_mb: 10 # max log file size before rotating
log_max_backups: 3 # number of old logs to keep
log_max_age_days: 7 # days to keep old logs


############ Local Node Configuration
node:
  name: "node1"
  host: "192.168.56.101"
  role: "database"
  os_user: "vagrant"   # The OS user that will run all the services
  tmp_path: /dbcp/tmp
  allow_restart_services: true  # or false


############ PostgreSQL Configuration
  postgresql:
    version: "17"
    data_dir: "/dbcp/data/pgsql"
    bin_path: "/usr/lib/postgresql/16/bin"
    users:
      postgres:
        password: "qaz123"
        options:
          - superuser
      monitor:  # Monitoring user
        password: "qaz123"
        options:
          - login

    parameters:
      port: 5432
      max_connections: 200
      use_pg_rewind: true
      use_slots: true
      wal_level: logical
      hot_standby: true
      synchronous_commit: "remote_apply"
      synchronous_standby_names: "1 (node2,node3)"

    initdb:
      - encoding: UTF8
      - locale: en_US.UTF-8
      - data-checksums: ""    # Need to make data-checksums a key with a null or empty value to avoid unmarshal errors!!

    pg_hba:
      - host replication replicator 192.168.56.0/24 scram-sha-256
      - host all         all        192.168.56.0/24 scram-sha-256


############ Patroni Configuration
  patroni:
    version: "4.0.4"
    namespace: "dbcp"
    api_listen: "0.0.0.0"
    port: 8008
    config_path: "/etc/patroni/patroni.yml"
    template_path: "/dbcp/config/patroni-template.yml"
    dcs:
      ttl: 30
      loop_wait: 10
      retry_timeout: 10
      maximum_lag_on_failover: 1048576  # 1MB in bytes
    authentication:
      replication:
        username: replicator
        password: "qaz123"
      superuser:
        username: postgres
        password: "qaz123"
    create_replica_methods:
      - basebackup
      - pgbackrest
    tags:
      nofailover: false
      noloadbalance: false
      clonefrom: true        # Allows cloning from this node
      nosync: false


############ ETCD Configuration
  etcd:
    version: "3.5.20"
    cluster_mode: "bootstrap"  # bootstrap or join
    data_dir: "/dbcp/data/etcd"
    bin_path: "/opt/etcd/bin"
    # cert_file: "/etc/etcd/certs/etcd.crt"
    # key_file: "/etc/etcd/certs/etcd.key"
    # ca_file: "/etc/etcd/certs/ca.crt"
    cert_file: ""
    key_file: ""
    ca_file: ""
    peer_port: 2380
    client_port: 2379


############ Cluster Configuration
cluster:
  name: "pg-cluster-01"
  nodes:
    - name: node1
      host: "192.168.56.101"
    - name: node2
      host: "192.168.56.102"
    - name: node3
      host: "192.168.56.103"


############ Repositories and Packages Configuration
repositories:
  postgresql:
    default: "official" # or "custom"
    sources:
      official:
        debian: "https://apt.postgresql.org/pub/repos/apt"
        rhel: "https://download.postgresql.org/pub/repos/yum"
      custom:
        base_url: "https://internal-repo.example.com/postgresql"
        debian_path: "/debian/$(lsb_release -cs)-pgdg"
        rhel_path: "/rhel/$(releasever)/$(basearch)"

  etcd:
    default: "official" # or "custom"
    sources:
      official:
        url: "https://storage.googleapis.com/etcd"
      custom:
        url: "https://github.com/etcd-io/etcd/releases/download"

  patroni:
    default: "official" # or "custom"
    sources:
      official:
        debian_package: "patroni"
        rhel_pip: "patroni[etcd]"
      custom:
        debian_url: "https://internal-repo.example.com/patroni/deb"
        rhel_rpm: "https://internal-repo.example.com/patroni/rpm"

`

func TestRepositorySelection(t *testing.T) {
	var cfg AgentConfig
	if err := yaml.NewDecoder(strings.NewReader(testYAML)).Decode(&cfg); err != nil {
		t.Fatalf("failed to parse test YAML: %v", err)
	}

	if cfg.Node.PostgreSQL.Version != "17" {
		t.Errorf("expected PostgreSQL version 17, got %s", cfg.Node.PostgreSQL.Version)
	}

	repoType := cfg.Repositories.PostgreSQL.Default
	sources := cfg.Repositories.PostgreSQL.Sources[repoType]

	debianRepo := sources["debian"]
	rhelRepo := sources["rhel"]

	if debianRepo != "https://apt.postgresql.org/pub/repos/apt" {
		t.Errorf("unexpected Debian repo: %s", debianRepo)
	}
	if rhelRepo != "https://download.postgresql.org/pub/repos/yum" {
		t.Errorf("unexpected RHEL repo: %s", rhelRepo)
	}
}

func TestValidConfigValidation(t *testing.T) {
	var cfg AgentConfig
	err := yaml.NewDecoder(strings.NewReader(testYAML)).Decode(&cfg)
	if err != nil {
		t.Fatalf("failed to parse test YAML: %v", err)
	}

	err = cfg.Validate()
	if err != nil {
		t.Errorf("expected valid config, got validation error: %v", err)
	}
}
