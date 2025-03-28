package pkg

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strconv"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
	"github.com/virtlabs-io/dbcp-agent/internal/logger"
)

// CreateDirs ensures required directories exist and are properly permissioned.
func CreateDirs(cfg *config.AgentConfig, paths ...string) error {
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
		}

		if err := MkdirAllAsUser(abs, cfg.Node.User, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", abs, err)
		}

		logger.Info("Ensured directory exists: %s", abs)
	}
	return nil
}

// MkdirAllAsUser creates the directory and sets ownership to the given username
func MkdirAllAsUser(path, username string, perm os.FileMode) error {
	// Create the directory with desired permissions
	if err := os.MkdirAll(path, perm); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", path, err)
	}

	// Lookup UID and GID
	usr, err := user.Lookup(username)
	if err != nil {
		return fmt.Errorf("failed to lookup user %s: %w", username, err)
	}

	uid, err := strconv.Atoi(usr.Uid)
	if err != nil {
		return fmt.Errorf("failed to convert UID: %w", err)
	}

	gid, err := strconv.Atoi(usr.Gid)
	if err != nil {
		return fmt.Errorf("failed to convert GID: %w", err)
	}

	// Walk recursively to chown everything
	return filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		return os.Chown(p, uid, gid)
	})
}
