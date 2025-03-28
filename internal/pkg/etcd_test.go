// internal/pkg/etcd_test.go
package pkg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
)

func TestInstallETCDMock(t *testing.T) {
	cfg := &config.AgentConfig{
		Node: config.NodeConfig{
			Host:    "localhost",
			TmpPath: "/tmp",
			ETCD: config.EtcdConfig{
				Version:    "3.5.9",
				BinPath:    "/tmp/etcd-test-bin",
				DataDir:    "/tmp/etcd-data",
				PeerPort:   2380,
				ClientPort: 2379,
			},
		},
		Repositories: config.Repositories{
			ETCD: config.RepoEntry{
				Default: "official",
				Sources: map[string]map[string]string{
					"official": {"url": "https://storage.googleapis.com/etcd"},
				},
			},
		},
	}

	repoURL := cfg.Repositories.ETCD.Sources["official"]["url"]

	err := InstallETCD(cfg, repoURL)
	if err != nil {
		t.Fatalf("failed to install ETCD: %v", err)
	}

	bin := filepath.Join(cfg.Node.ETCD.BinPath, "etcd")
	if _, err := os.Stat(bin); os.IsNotExist(err) {
		t.Errorf("expected etcd binary to exist at %s", bin)
	}
}
