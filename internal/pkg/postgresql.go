package pkg

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/virtlabs-io/dbcp-client/internal/config"
	"github.com/virtlabs-io/dbcp-client/internal/logger"
	"github.com/virtlabs-io/dbcp-client/internal/system"
)

func InstallPostgreSQL(cfg *config.AgentConfig, osInfo *system.OSInfo) error {
	logger.Info("Preparing to install PostgreSQL version %s...", cfg.Node.PostgreSQL.Version)

	pgRepo := cfg.Repositories.PostgreSQL.Sources[cfg.Repositories.PostgreSQL.Default]
	pgRepoURL := pgRepo[osInfo.Family]

	switch osInfo.ID {
	case "debian", "ubuntu":
		if err := installPostgresApt(cfg.Node.PostgreSQL.Version, pgRepoURL); err != nil {
			return err
		}
	case "rhel", "centos", "rocky", "almalinux", "oracle", "fedora":
		if err := installPostgresRpm(cfg.Node.PostgreSQL.Version, osInfo.VersionID, pgRepoURL, cfg.Node.TmpPath); err != nil {
			return err
		}
	default:
		return fmt.Errorf("unsupported OS: %s", osInfo.ID)
	}

	if err := startPostgreSQLService(cfg); err != nil {
		return err
	}

	logger.Info("PostgreSQL %s installation and startup complete.", cfg.Node.PostgreSQL.Version)
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
		"systemctl stop postgresql",
		"systemctl disable postgresql",
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

func startPostgreSQLService(cfg *config.AgentConfig) error {
	if isPortInUse(5432) {
		if !cfg.Node.AllowRestartServices {
			logger.Warn("PostgreSQL appears to be running. Aborting due to config.")
			return fmt.Errorf("port 5432 is already in use")
		}

		logger.Warn("PostgreSQL appears to be running. Attempting to stop existing process...")
		stopPostgresProcess(cfg) // maybe using pg_ctl stop or systemd
	}

	if _, err := exec.LookPath("systemctl"); err == nil {
		logger.Info("Using systemctl to start PostgreSQL")
		cmd := exec.Command("bash", "-c", "systemctl start postgresql")
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("Failed to start PostgreSQL with systemctl: %v\nOutput: %s", err, string(output))
			return err
		}
		return nil
	}

	logger.Info("systemctl not found")

	if err := ensureUser(cfg.Node.PostgreSQL.User); err != nil {
		return err
	}

	chownCmd := exec.Command("chown", "-R", fmt.Sprintf("%s:%s",
		cfg.Node.PostgreSQL.User,
		cfg.Node.PostgreSQL.User),
		cfg.Node.PostgreSQL.DataDir)
	if output, err := chownCmd.CombinedOutput(); err != nil {
		logger.Warn("Failed to chown data dir: %v\nOutput: %s", err, string(output))
		return err
	}

	if _, err := os.Stat(filepath.Join(cfg.Node.PostgreSQL.DataDir, "PG_VERSION")); os.IsNotExist(err) {
		logger.Warn("Initializing PostgreSQL data directory...")
		cmd := exec.Command("su", "-",
			cfg.Node.PostgreSQL.User,
			"-c",
			fmt.Sprintf("PATH=%s:$PATH %s/initdb -D %s",
				cfg.Node.PostgreSQL.BinPath,
				cfg.Node.PostgreSQL.BinPath,
				cfg.Node.PostgreSQL.DataDir))
		output, err := cmd.CombinedOutput()
		if err != nil {
			logger.Error("initdb failed: %v\nOutput: %s", err, string(output))
			return err
		}
		logger.Info("PostgreSQL data directory initialized.")
	}

	logFile := filepath.Join(cfg.Node.PostgreSQL.DataDir, "postgres.log")
	cmd := exec.Command("su", "-",
		cfg.Node.PostgreSQL.User,
		"-c",
		fmt.Sprintf("PATH=%s:$PATH %s/pg_ctl -D %s -l %s start",
			cfg.Node.PostgreSQL.BinPath,
			cfg.Node.PostgreSQL.BinPath,
			cfg.Node.PostgreSQL.DataDir, logFile))
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

func isPortInUse(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return true // port is already in use
	}
	_ = ln.Close()
	return false
}

func stopPostgresProcess(cfg *config.AgentConfig) {
	cmd := exec.Command(filepath.Join(cfg.Node.PostgreSQL.BinPath, "pg_ctl"),
		"-D", cfg.Node.PostgreSQL.DataDir,
		"stop")
	if err := cmd.Run(); err != nil {
		logger.Error("Failed to stop PostgreSQL gracefully: %v", err)
	} else {
		logger.Info("Stopped running PostgreSQL instance")
	}
}
