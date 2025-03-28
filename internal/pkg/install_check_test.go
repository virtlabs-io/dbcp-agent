package pkg_test

import (
	"os"
	"os/exec"
	"testing"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
	"github.com/virtlabs-io/dbcp-agent/internal/pkg"
)

func TestIsPostgreSQLInstalledFalse(t *testing.T) {
	cfg := &config.AgentConfig{
		Node: config.NodeConfig{
			PostgreSQL: config.PostgreSQLConfig{
				Version: "17",
				BinPath: "/nonexistent/path",
			},
		},
	}

	if pkg.IsPostgreSQLInstalled(cfg) {
		t.Error("Expected PostgreSQL to not be installed")
	}
}

func TestIsETCDInstalledFalse(t *testing.T) {
	cfg := &config.AgentConfig{
		Node: config.NodeConfig{
			ETCD: config.EtcdConfig{
				Version: "3.5.9",
				BinPath: "/nonexistent/path",
			},
		},
	}

	if pkg.IsETCDInstalled(cfg) {
		t.Error("Expected ETCD to not be installed")
	}
}

func TestIsPatroniInstalledFalse(t *testing.T) {
	cfg := &config.AgentConfig{
		Node: config.NodeConfig{
			Patroni: config.PatroniConfig{
				Version: "2.1.4",
			},
		},
	}

	// Simulate missing Patroni by renaming it temporarily
	originalPath, _ := exec.LookPath("patroni")
	if originalPath != "" {
		renamed := originalPath + ".bak"
		_ = os.Rename(originalPath, renamed)
		defer os.Rename(renamed, originalPath)
	}

	if pkg.IsPatroniInstalled(cfg) {
		t.Error("Expected Patroni to not be installed")
	}
}
