package system

import (
	"bufio"
	"os"
	"strings"
)

// OSInfo holds basic operating system information
type OSInfo struct {
	ID        string
	VersionID string
	Name      string
	Pretty    string
	Family    string // e.g., debian, rhel
}

// DetectOS reads /etc/os-release and returns basic OS info
func DetectOS() (*OSInfo, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	info := &OSInfo{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "ID=") {
			info.ID = strings.Trim(strings.SplitN(line, "=", 2)[1], "\"")
		} else if strings.HasPrefix(line, "NAME=") {
			info.Name = strings.Trim(strings.SplitN(line, "=", 2)[1], "\"")
		} else if strings.HasPrefix(line, "PRETTY_NAME=") {
			info.Pretty = strings.Trim(strings.SplitN(line, "=", 2)[1], "\"")
		}
	}

	// Normalize ID into family
	switch info.ID {
	case "debian", "ubuntu":
		info.Family = "debian"
	case "centos", "rhel", "rocky", "almalinux", "oracle":
		info.Family = "rhel"
	case "fedora":
		info.Family = "fedora"
	default:
		info.Family = info.ID // fallback
	}

	return info, nil
}
