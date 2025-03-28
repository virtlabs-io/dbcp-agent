package pkg

import (
	"os"
	"testing"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
)

func TestGeneratePatroniConfig(t *testing.T) {
	cfg := &config.AgentConfig{
		Cluster: config.ClusterConfig{
			Name: "pg-test",
			Nodes: []config.ClusterNode{
				{Name: "node1", Host: "127.0.0.1"},
			},
		},
		Node: config.NodeConfig{
			Name:    "node1",
			Host:    "127.0.0.1",
			User:    "vagrant",
			TmpPath: "/tmp",
			PostgreSQL: config.PostgreSQLConfig{
				DataDir: "/tmp/pgdata",
				BinPath: "/usr/lib/postgresql/16/bin",
				Users: map[string]config.PostgresUser{
					"postgres": {
						Password: "secret",
						Options:  []string{"superuser"},
					},
				},
				Parameters: config.PostgresSettings{
					Port:                    5432,
					UsePGRewind:             true,
					UseSlots:                true,
					WALLevel:                "logical",
					HotStandby:              "on",
					SynchronousCommit:       "remote_apply",
					SynchronousStandbyNames: "1 (node2,node3)",
				},
			},
			Patroni: config.PatroniConfig{
				ConfigPath:   "/tmp/patroni.yml",
				TemplatePath: "../../configs/patroni-template.yml",
				APIListen:    "127.0.0.1",
				Port:         8008,
				Authentication: config.PatroniAuthConfig{
					Superuser: config.UserCredentials{
						Username: "postgres", Password: "secret",
					},
					Replication: config.UserCredentials{
						Username: "rep", Password: "rep-pass",
					},
				},
				DCS: config.DCSConfig{
					TTL:                  30,
					LoopWait:             10,
					RetryTimeout:         10,
					MaximumLagOnFailover: 1048576,
				},
			},
			ETCD: config.EtcdConfig{
				ClientPort: 2379,
			},
		},
	}

	err := GeneratePatroniConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to generate Patroni config: %v", err)
	}

	// Verify config file was written
	if _, err := os.Stat(cfg.Node.Patroni.ConfigPath); os.IsNotExist(err) {
		t.Errorf("Expected Patroni config to exist at %s", cfg.Node.Patroni.ConfigPath)
	}
}
