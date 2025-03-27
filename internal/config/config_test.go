package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const testYAML = `
cluster:
  name: "pg-cluster-01"
  nodes:
    - host: "node1"
      role: "database"
      etcd_member: true

repositories:
  postgresql:
    default: "official"
    sources:
      official:
        debian: "https://apt.postgresql.org/pub/repos/apt"
        rhel: "https://download.postgresql.org/pub/repos/yum"
      custom:
        debian: "https://internal.example.com/pg/debian"
        rhel: "https://internal.example.com/pg/rhel"

versions:
  postgresql: "17"
  etcd: "3.5.9"
  patroni: "2.1.4"
`

func TestRepositorySelection(t *testing.T) {
	var cfg AgentConfig
	if err := yaml.NewDecoder(strings.NewReader(testYAML)).Decode(&cfg); err != nil {
		t.Fatalf("failed to parse test YAML: %v", err)
	}

	if cfg.Versions.PostgreSQL != "17" {
		t.Errorf("expected PostgreSQL version 17, got %s", cfg.Versions.PostgreSQL)
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

func TestInvalidMissingVersion(t *testing.T) {
	invalidYAML := strings.Replace(testYAML, "postgresql: \"17\"", "postgresql: \"\"", 1)
	var cfg AgentConfig
	err := yaml.NewDecoder(strings.NewReader(invalidYAML)).Decode(&cfg)
	if err != nil {
		t.Fatalf("failed to parse test YAML: %v", err)
	}

	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "postgresql version is required") {
		t.Errorf("expected version validation error, got: %v", err)
	}
}

func TestInvalidMissingRepoEntry(t *testing.T) {
	badYAML := strings.Replace(testYAML, "rhel: \"https://download.postgresql.org/pub/repos/yum\"", "", 1)
	var cfg AgentConfig
	err := yaml.NewDecoder(strings.NewReader(badYAML)).Decode(&cfg)
	if err != nil {
		t.Fatalf("failed to parse bad YAML: %v", err)
	}

	err = cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "must include 'rhel'") {
		t.Errorf("expected missing 'rhel' repo validation error, got: %v", err)
	}
}
