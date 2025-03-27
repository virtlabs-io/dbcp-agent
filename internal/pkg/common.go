package pkg

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/virtlabs-io/dbcp-client/internal/logger"
)

// CreateDirs ensures required directories exist and are properly permissioned.
func CreateDirs(paths ...string) error {
	for _, path := range paths {
		abs, err := filepath.Abs(path)
		if err != nil {
			return fmt.Errorf("failed to get absolute path for %s: %w", path, err)
		}

		if err := os.MkdirAll(abs, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", abs, err)
		}

		logger.Info("Ensured directory exists: %s", abs)
	}
	return nil
}
