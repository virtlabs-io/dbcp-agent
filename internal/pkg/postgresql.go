package pkg

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/virtlabs-io/dbcp-client/internal/logger"
	"github.com/virtlabs-io/dbcp-client/internal/system"
)

func InstallPostgreSQL(version string, osInfo *system.OSInfo, repoURL string, binPath string, dataDir string, user string, tmpPath string) error {
	logger.Info("Preparing to install PostgreSQL version %s...", version)

	switch osInfo.ID {
	case "debian", "ubuntu":
		if err := installPostgresApt(version, repoURL); err != nil {
			return err
		}
	case "rhel", "centos", "rocky", "almalinux", "oracle", "fedora":
		if err := installPostgresRpm(version, osInfo.VersionID, repoURL, tmpPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported OS: %s", osInfo.ID)
	}

	if err := startPostgreSQLService(binPath, dataDir, user); err != nil {
		return err
	}

	logger.Info("PostgreSQL %s installation and startup complete.", version)
	return nil
}

func installPostgresApt(version, repoURL string) error {
	cmds := []string{
		"apt-get update",
		"apt-get install -y curl ca-certificates gnupg lsb-release",
		"mkdir -p /usr/share/postgresql-common/pgdg",
		"curl -sSL https://www.postgresql.org/media/keys/ACCC4CF8.asc -o /usr/share/postgresql-common/pgdg/apt.postgresql.org.asc",
		fmt.Sprintf(`sh -c 'echo "deb [signed-by=/usr/share/postgresql-common/pgdg/apt.postgresql.org.asc] %s $(lsb_release -cs)-pgdg main" > /etc/apt/sources.list.d/pgdg.list'`, repoURL),
		"apt-get update",
		fmt.Sprintf("apt-get install -y postgresql-%s", version),
	}

	for _, cmd := range cmds {
		logger.Debug("Executing: %s", cmd)
		if err := runCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func installPostgresRpm(version, osVersion, repoBaseURL, tmpPath string) error {
	majorVersion := string(osVersion[0])
	rpmURL := fmt.Sprintf("%s/reporpms/EL-%s-x86_64/pgdg-redhat-repo-latest.noarch.rpm", repoBaseURL, majorVersion)
	tmpFile := filepath.Join(tmpPath, "pgdg-redhat-repo-latest.noarch.rpm")

	cmds := []string{
		fmt.Sprintf("curl -sSL -o %s %s", tmpFile, rpmURL),
		fmt.Sprintf("dnf install -y %s", tmpFile),
		"dnf -qy module disable postgresql",
		fmt.Sprintf("dnf install -y postgresql%s-server postgresql%s", version, version),
	}

	for _, cmd := range cmds {
		logger.Debug("Executing: %s", cmd)
		if err := runCommand(cmd); err != nil {
			return err
		}
	}
	return nil
}

func startPostgreSQLService(binPath, dataDir, user string) error {
	if _, err := exec.LookPath("systemctl"); err == nil {
		logger.Info("Using systemctl to start PostgreSQL")
		cmd := exec.Command("bash", "-c", "systemctl enable postgresql && systemctl start postgresql")
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("Failed to start PostgreSQL with systemctl: %v\nOutput: %s", err, string(output))
			return err
		}
		return nil
	}

	logger.Info("systemctl not found — running in Docker mode")

	if err := ensureUser(user); err != nil {
		return err
	}

	chownCmd := exec.Command("chown", "-R", fmt.Sprintf("%s:%s", user, user), dataDir)
	if output, err := chownCmd.CombinedOutput(); err != nil {
		logger.Warn("Failed to chown data dir: %v\nOutput: %s", err, string(output))
		return err
	}

	if _, err := os.Stat(filepath.Join(dataDir, "PG_VERSION")); os.IsNotExist(err) {
		logger.Warn("Initializing PostgreSQL data directory...")
		cmd := exec.Command("su", "-", user, "-c", fmt.Sprintf("PATH=%s:$PATH %s/initdb -D %s", binPath, binPath, dataDir))
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("initdb failed: %v\nOutput: %s", err, string(output))
			return err
		}
		logger.Info("PostgreSQL data directory initialized.")
	}

	logFile := filepath.Join(dataDir, "postgres.log")
	cmd := exec.Command("su", "-", user, "-c", fmt.Sprintf("PATH=%s:$PATH %s/pg_ctl -D %s -l %s start", binPath, binPath, dataDir, logFile))
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("pg_ctl failed: %v\nOutput: %s", err, string(output))
		return err
	}

	logger.Info("PostgreSQL started successfully using pg_ctl.")
	return nil
}

func ensureUser(user string) error {
	cmd := exec.Command("id", "-u", user)
	if err := cmd.Run(); err == nil {
		return nil
	}

	logger.Info("Creating '%s' user...", user)
	addUser := exec.Command("useradd", "-m", user)
	output, err := addUser.CombinedOutput()
	if err != nil {
		logger.Error("Failed to create user '%s': %v\nOutput: %s", user, err, string(output))
		return err
	}
	return nil
}

func runCommand(command string) error {
	cmd := exec.Command("bash", "-c", command)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Command failed: %s\nOutput: %s", command, string(output))
		return err
	}
	return nil
}
