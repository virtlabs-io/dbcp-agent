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

	etcdURL := fmt.Sprintf("%s/v%s/etcd-v%s-linux-amd64.tar.gz", strings.TrimSuffix(repoURL, "/"), cfg.Node.ETCD.Version, cfg.Node.ETCD.Version)
	archivePath := filepath.Join(cfg.Node.TmpPath, fmt.Sprintf("etcd-v%s-linux-amd64.tar.gz", cfg.Node.ETCD.Version))
	extractDir := filepath.Join(cfg.Node.TmpPath, fmt.Sprintf("etcd-v%s", cfg.Node.ETCD.Version))

	logger.Info("Downloading ETCD from %s", etcdURL)
	if err := downloadFile(archivePath, etcdURL); err != nil {
		return fmt.Errorf("failed to download etcd: %w", err)
	}

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
	}

	logger.Info("ETCD binaries installed to %s", binDir)
	return nil
}

func StartETCD(cfg *config.AgentConfig) error {
	node := cfg.Node
	cluster := cfg.Cluster

	nodeName := strings.ReplaceAll(node.Host, ".", "-")
	dataDir := node.ETCD.DataDir

	initialCluster := []string{}
	for _, peer := range cluster.Nodes {
		peerName := strings.ReplaceAll(peer.Host, ".", "-")
		entry := fmt.Sprintf("%s=https://%s:%d", peerName, peer.Host, node.ETCD.PeerPort)
		initialCluster = append(initialCluster, entry)
	}

	args := []string{
		fmt.Sprintf("--name=%s", nodeName),
		fmt.Sprintf("--data-dir=%s", dataDir),
		fmt.Sprintf("--initial-advertise-peer-urls=https://%s:%d", node.Host, node.ETCD.PeerPort),
		fmt.Sprintf("--listen-peer-urls=https://0.0.0.0:%d", node.ETCD.PeerPort),
		fmt.Sprintf("--listen-client-urls=https://0.0.0.0:%d", node.ETCD.ClientPort),
		fmt.Sprintf("--advertise-client-urls=https://%s:%d", node.Host, node.ETCD.ClientPort),
		fmt.Sprintf("--initial-cluster=%s", strings.Join(initialCluster, ",")),
		"--initial-cluster-state=new",
		fmt.Sprintf("--cert-file=%s", node.ETCD.CertFile),
		fmt.Sprintf("--key-file=%s", node.ETCD.KeyFile),
		fmt.Sprintf("--client-cert-auth=true"),
		fmt.Sprintf("--trusted-ca-file=%s", node.ETCD.CAFile),
		fmt.Sprintf("--peer-cert-file=%s", node.ETCD.CertFile),
		fmt.Sprintf("--peer-key-file=%s", node.ETCD.KeyFile),
		fmt.Sprintf("--peer-client-cert-auth=true"),
		fmt.Sprintf("--peer-trusted-ca-file=%s", node.ETCD.CAFile),
	}

	cmd := exec.Command(filepath.Join(node.ETCD.BinPath, "etcd"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Starting ETCD as '%s'...", nodeName)
	return cmd.Start()
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

func StartETCDCluster(cfg *config.AgentConfig) error {
	mode := cfg.Node.ETCD.ClusterMode
	if mode == "bootstrap" {
		logger.Info("ETCD cluster mode: bootstrap")
		return StartETCD(cfg)
	}

	logger.Info("ETCD cluster mode: join — attempting to discover peers...")
	var foundPeer string
	for _, peer := range cfg.Cluster.Nodes {
		if peer.Host == cfg.Node.Host {
			continue
		}

		healthURL := fmt.Sprintf("https://%s:%d/health", peer.Host, cfg.Node.ETCD.ClientPort)
		logger.Info("Checking ETCD peer health at %s", healthURL)

		cmd := exec.Command("curl", "--cacert", cfg.Node.ETCD.CAFile,
			"--cert", cfg.Node.ETCD.CertFile,
			"--key", cfg.Node.ETCD.KeyFile,
			"-s", healthURL)
		output, err := cmd.CombinedOutput()
		if err == nil && strings.Contains(string(output), "true") {
			foundPeer = peer.Host
			break
		}
	}

	if foundPeer == "" {
		return fmt.Errorf("could not find any healthy ETCD peers to join")
	}

	logger.Info("Found healthy ETCD node at %s — attempting to join...", foundPeer)
	nodeName := strings.ReplaceAll(cfg.Node.Host, ".", "-")
	peerURL := fmt.Sprintf("https://%s:%d", cfg.Node.Host, cfg.Node.ETCD.PeerPort)

	cmd := exec.Command(filepath.Join(cfg.Node.ETCD.BinPath, "etcdctl"),
		"--endpoints", fmt.Sprintf("https://%s:%d", foundPeer, cfg.Node.ETCD.ClientPort),
		"--cacert", cfg.Node.ETCD.CAFile,
		"--cert", cfg.Node.ETCD.CertFile,
		"--key", cfg.Node.ETCD.KeyFile,
		"member", "add", nodeName,
		fmt.Sprintf("--peer-urls=%s", peerURL),
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Error("Failed to add node to cluster: %v\nOutput: %s", err, string(output))
		return err
	}

	logger.Info("Successfully added node to ETCD cluster: %s", string(output))
	return StartETCDWithState(cfg, "existing")
}

func StartETCDWithState(cfg *config.AgentConfig, state string) error {
	node := cfg.Node
	cluster := cfg.Cluster

	nodeName := strings.ReplaceAll(node.Host, ".", "-")
	dataDir := node.ETCD.DataDir

	initialCluster := []string{}
	for _, peer := range cluster.Nodes {
		peerName := strings.ReplaceAll(peer.Host, ".", "-")
		entry := fmt.Sprintf("%s=https://%s:%d", peerName, peer.Host, node.ETCD.PeerPort)
		initialCluster = append(initialCluster, entry)
	}

	args := []string{
		fmt.Sprintf("--name=%s", nodeName),
		fmt.Sprintf("--data-dir=%s", dataDir),
		fmt.Sprintf("--initial-advertise-peer-urls=https://%s:%d", node.Host, node.ETCD.PeerPort),
		fmt.Sprintf("--listen-peer-urls=https://0.0.0.0:%d", node.ETCD.PeerPort),
		fmt.Sprintf("--listen-client-urls=https://0.0.0.0:%d", node.ETCD.ClientPort),
		fmt.Sprintf("--advertise-client-urls=https://%s:%d", node.Host, node.ETCD.ClientPort),
		fmt.Sprintf("--initial-cluster=%s", strings.Join(initialCluster, ",")),
		fmt.Sprintf("--initial-cluster-state=%s", state),
		fmt.Sprintf("--cert-file=%s", node.ETCD.CertFile),
		fmt.Sprintf("--key-file=%s", node.ETCD.KeyFile),
		fmt.Sprintf("--client-cert-auth=true"),
		fmt.Sprintf("--trusted-ca-file=%s", node.ETCD.CAFile),
		fmt.Sprintf("--peer-cert-file=%s", node.ETCD.CertFile),
		fmt.Sprintf("--peer-key-file=%s", node.ETCD.KeyFile),
		fmt.Sprintf("--peer-client-cert-auth=true"),
		fmt.Sprintf("--peer-trusted-ca-file=%s", node.ETCD.CAFile),
	}

	cmd := exec.Command(filepath.Join(node.ETCD.BinPath, "etcd"), args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	logger.Info("Starting ETCD node '%s' with state '%s'...", nodeName, state)
	return cmd.Start()
}
