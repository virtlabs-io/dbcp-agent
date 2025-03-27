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

	"github.com/virtlabs-io/dbcp-client/internal/config"
	"github.com/virtlabs-io/dbcp-client/internal/logger"
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
	protocol := getETCDProtocol(cfg)

	args := []string{
		"--name", node.ETCD.PeerName,
		"--data-dir", node.ETCD.DataDir,
		"--initial-advertise-peer-urls", fmt.Sprintf("%s://%s:%d", protocol, node.Host, node.ETCD.PeerPort),
		"--listen-peer-urls", fmt.Sprintf("%s://0.0.0.0:%d", protocol, node.ETCD.PeerPort),
		"--listen-client-urls", fmt.Sprintf("%s://0.0.0.0:%d", protocol, node.ETCD.ClientPort),
		"--advertise-client-urls", fmt.Sprintf("%s://%s:%d", protocol, node.Host, node.ETCD.ClientPort),
		"--initial-cluster-state", "new",
	}

	// Build initial-cluster string
	var peers []string
	for _, peer := range cfg.Cluster.Nodes {
		peers = append(peers, fmt.Sprintf("%s=%s://%s:%d", peer.Name, protocol, peer.Host, node.ETCD.PeerPort))
	}
	initialCluster := strings.Join(peers, ",")
	args = append(args, "--initial-cluster", initialCluster)

	// TLS if configured
	appendETCDTLSArgs(cfg, &args)

	cmd := exec.Command(filepath.Join(node.ETCD.BinPath, "etcd"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Starting ETCD as '%s'...", node.ETCD.PeerName)
	return cmd.Start()
}

func StartETCDCluster(cfg *config.AgentConfig) error {
	switch cfg.Node.ETCD.ClusterMode {
	case "bootstrap":
		logger.Info("ETCD cluster mode: bootstrap")
		return StartETCD(cfg)
	case "join":
		logger.Info("ETCD cluster mode: join — discovering peers")
		return joinETCDCluster(cfg)
	default:
		return fmt.Errorf("unsupported etcd.cluster_mode: %s", cfg.Node.ETCD.ClusterMode)
	}
}

func joinETCDCluster(cfg *config.AgentConfig) error {
	protocol := getETCDProtocol(cfg)
	args := []string{}

	if cfg.Node.ETCD.CertFile != "" {
		args = append(args,
			"--cacert", cfg.Node.ETCD.CAFile,
			"--cert", cfg.Node.ETCD.CertFile,
			"--key", cfg.Node.ETCD.KeyFile,
		)
	}

	var foundPeer string
	for _, peer := range cfg.Cluster.Nodes {
		if peer.Host == cfg.Node.Host {
			continue
		}
		url := fmt.Sprintf("%s://%s:%d/health", protocol, peer.Host, cfg.Node.ETCD.ClientPort)
		logger.Info("Checking ETCD peer health at %s", url)

		curlArgs := append(args, "-s", url)
		cmd := exec.Command("curl", curlArgs...)
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "true") {
			foundPeer = peer.Host
			break
		}
	}

	if foundPeer == "" {
		return fmt.Errorf("could not find healthy peer to join")
	}

	logger.Info("Found peer %s — sending join request...", foundPeer)
	peerURL := fmt.Sprintf("%s://%s:%d", protocol, cfg.Node.Host, cfg.Node.ETCD.PeerPort)

	ctlArgs := append([]string{
		"--endpoints", fmt.Sprintf("%s://%s:%d", protocol, foundPeer, cfg.Node.ETCD.ClientPort),
		"member", "add", cfg.Node.ETCD.PeerName,
		fmt.Sprintf("--peer-urls=%s", peerURL),
	}, args...)

	cmd := exec.Command(filepath.Join(cfg.Node.ETCD.BinPath, "etcdctl"), ctlArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to add node to cluster: %v\nOutput: %s", err, string(output))
		return err
	}

	logger.Info("Node added to cluster. Starting with 'existing' state...")
	return StartETCDWithState(cfg, "existing")
}

func StartETCDWithState(cfg *config.AgentConfig, state string) error {
	node := cfg.Node
	protocol := getETCDProtocol(cfg)

	var peers []string
	for _, peer := range cfg.Cluster.Nodes {
		peers = append(peers, fmt.Sprintf("%s=%s://%s:%d", peer.Name, protocol, peer.Host, node.ETCD.PeerPort))
	}
	initialCluster := strings.Join(peers, ",")

	args := []string{
		"--name", node.ETCD.PeerName,
		"--data-dir", node.ETCD.DataDir,
		"--initial-advertise-peer-urls", fmt.Sprintf("%s://%s:%d", protocol, node.Host, node.ETCD.PeerPort),
		"--listen-peer-urls", fmt.Sprintf("%s://0.0.0.0:%d", protocol, node.ETCD.PeerPort),
		"--listen-client-urls", fmt.Sprintf("%s://0.0.0.0:%d", protocol, node.ETCD.ClientPort),
		"--advertise-client-urls", fmt.Sprintf("%s://%s:%d", protocol, node.Host, node.ETCD.ClientPort),
		"--initial-cluster", initialCluster,
		"--initial-cluster-state", state,
	}

	appendETCDTLSArgs(cfg, &args)

	cmd := exec.Command(filepath.Join(node.ETCD.BinPath, "etcd"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Starting ETCD '%s' with cluster-state=%s", node.ETCD.PeerName, state)
	return cmd.Start()
}

func getETCDProtocol(cfg *config.AgentConfig) string {
	if cfg.Node.ETCD.CertFile != "" && cfg.Node.ETCD.KeyFile != "" && cfg.Node.ETCD.CAFile != "" {
		return "https"
	}
	return "http"
}

func appendETCDTLSArgs(cfg *config.AgentConfig, args *[]string) {
	node := cfg.Node
	if node.ETCD.CertFile == "" || node.ETCD.KeyFile == "" || node.ETCD.CAFile == "" {
		logger.Warn("TLS is not configured — running ETCD in insecure mode")
		return
	}

	*args = append(*args,
		"--cert-file", node.ETCD.CertFile,
		"--key-file", node.ETCD.KeyFile,
		"--trusted-ca-file", node.ETCD.CAFile,
		"--client-cert-auth",
		"--peer-cert-file", node.ETCD.CertFile,
		"--peer-key-file", node.ETCD.KeyFile,
		"--peer-trusted-ca-file", node.ETCD.CAFile,
		"--peer-client-cert-auth",
	)
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
