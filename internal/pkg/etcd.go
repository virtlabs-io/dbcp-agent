package pkg

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/virtlabs-io/dbcp-agent/internal/config"
	"github.com/virtlabs-io/dbcp-agent/internal/logger"
)

func InstallETCD(cfg *config.AgentConfig, repoURL string) error {
	logger.Info("Installing ETCD version %s...", cfg.Node.ETCD.Version)

	etcdURL := fmt.Sprintf("%s/v%s/etcd-v%s-linux-amd64.tar.gz",
		strings.TrimSuffix(repoURL, "/"),
		cfg.Node.ETCD.Version,
		cfg.Node.ETCD.Version,
	)

	archivePath := fmt.Sprintf("/tmp/etcd-v%s-linux-amd64.tar.gz", cfg.Node.ETCD.Version)

	logger.Info("Downloading ETCD from %s", etcdURL)
	if err := downloadFile(archivePath, etcdURL); err != nil {
		return fmt.Errorf("failed to download etcd: %w", err)
	}

	extractDir := filepath.Join("/tmp", fmt.Sprintf("etcd-v%s", cfg.Node.ETCD.Version))
	if err := extractTarGz(archivePath, extractDir); err != nil {
		return fmt.Errorf("failed to extract etcd: %w", err)
	}

	binDir := cfg.Node.ETCD.BinPath
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create bin path: %w", err)
	}

	for _, bin := range []string{"etcd", "etcdctl"} {
		src := filepath.Join(extractDir, fmt.Sprintf("etcd-v%s-linux-amd64", cfg.Node.ETCD.Version), bin)
		dst := filepath.Join(binDir, bin)
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("failed to move %s: %w", bin, err)
		}
		if err := os.Chmod(dst, 0755); err != nil {
			return fmt.Errorf("failed to chmod %s: %w", dst, err)
		}
	}

	logger.Info("ETCD binaries installed to %s", binDir)
	return nil
}

func StartETCD(cfg *config.AgentConfig) error {
	node := cfg.Node
	dataDir := node.ETCD.DataDir
	bin := filepath.Join(node.ETCD.BinPath, "etcd")

	protocol := "http"
	args := []string{}

	// Use TLS if configured
	if cfg.Node.ETCD.CertFile != "" && cfg.Node.ETCD.KeyFile != "" && cfg.Node.ETCD.CAFile != "" {
		protocol = "https"
		args = append(args,
			"--cert-file", cfg.Node.ETCD.CertFile,
			"--key-file", cfg.Node.ETCD.KeyFile,
			"--trusted-ca-file", cfg.Node.ETCD.CAFile,
			"--client-cert-auth=true",
			"--peer-cert-file", cfg.Node.ETCD.CertFile,
			"--peer-key-file", cfg.Node.ETCD.KeyFile,
			"--peer-trusted-ca-file", cfg.Node.ETCD.CAFile,
			"--peer-client-cert-auth=true",
		)
	}

	// Initial cluster string
	initialCluster := []string{}
	for _, peer := range cfg.Cluster.Nodes {
		entry := fmt.Sprintf("%s=%s://%s:%d", peer.Name, protocol, peer.Host, node.ETCD.PeerPort)
		initialCluster = append(initialCluster, entry)
	}

	mode := "new"
	if cfg.Node.ETCD.ClusterMode == "join" {
		mode = "existing"
	}

	// Main ETCD args
	args = append(args,
		"--name", node.Name,
		"--data-dir", dataDir,
		"--initial-cluster", strings.Join(initialCluster, ","),
		"--initial-cluster-state", mode,
		fmt.Sprintf("--initial-advertise-peer-urls=%s://%s:%d", protocol, node.Host, node.ETCD.PeerPort),
		fmt.Sprintf("--listen-peer-urls=%s://0.0.0.0:%d", protocol, node.ETCD.PeerPort),
		fmt.Sprintf("--listen-client-urls=%s://0.0.0.0:%d", protocol, node.ETCD.ClientPort),
		fmt.Sprintf("--advertise-client-urls=%s://%s:%d", protocol, node.Host, node.ETCD.ClientPort),
	)

	// Create ETCD log file
	logFilePath := filepath.Join(node.TmpPath, "etcd.log")
	logFile, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to create ETCD log file: %w", err)
	}

	// Create the command
	cmd := exec.Command(bin, args...)
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	// Start in background
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ETCD: %w", err)
	}

	logger.Info("ETCD started in background with PID %d — logs at %s", cmd.Process.Pid, logFilePath)
	return nil
}

func downloadFile(target string, url string) error {
	out, err := os.Create(target)
	if err != nil {
		return err
	}
	defer out.Close()

	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func extractTarGz(gzPath string, dest string) error {
	f, err := os.Open(gzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tarReader := tar.NewReader(gzr)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		path := filepath.Join(dest, header.Name)
		switch header.Typeflag {
		case tar.TypeDir:
			os.MkdirAll(path, os.FileMode(header.Mode))
		case tar.TypeReg:
			outFile, err := os.Create(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return err
			}
			outFile.Close()
		}
	}
	return nil
}
