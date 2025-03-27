package pkg

import (
	"os"
	"testing"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
)

func TestGeneratePatroniConfig(t *testing.T) {
	cfg := &config.AgentConfig{
		Cluster: config.ClusterConfig{Name: "pg-test"},
		Node: config.NodeConfig{
			Host: "127.0.0.1",
			ETCD: config.EtcdConfig{PeerName: "node1"},
			PostgreSQL: config.PostgreSQLConfig{
				DataDir: "/tmp/pgdata",
				BinPath: "/usr/lib/postgresql/16/bin",
			},
			Patroni: config.PatroniConfig{
				ConfigPath:   "/tmp/patroni.yml",
				TemplatePath: "testdata/patroni-template.yml",
				APIListen:    "127.0.0.1:8008",
				EtcdHost:     "127.0.0.1:2379",
				PGPort:       5432,
				Superuser:    config.PatroniUser{Username: "postgres", Password: "secret"},
				Replication:  config.PatroniUser{Username: "rep", Password: "rep-pass"},
				AdminUser: config.PatroniUser{
					Username: "admin", Password: "admin", Options: []string{"createrole"},
				},
				InitDB:      []string{"encoding: UTF8"},
				UsePGRewind: true,
				UseSlots:    true,
				DCS: config.DCSSettings{
					TTL: 30, LoopWait: 10, RetryTimeout: 10, MaximumLagOnFailover: 1048576,
				},
			},
		},
	}

	err := GeneratePatroniConfig(cfg, cfg.Node.Patroni.TemplatePath)
	if err != nil {
		t.Fatalf("Failed to generate Patroni config: %v", err)
	}

	// Verify config file was written
	if _, err := os.Stat(cfg.Node.Patroni.ConfigPath); os.IsNotExist(err) {
		t.Errorf("Expected patroni config to exist at %s", cfg.Node.Patroni.ConfigPath)
	}
}
