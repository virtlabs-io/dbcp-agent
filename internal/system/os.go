package system

import (
	"errors"
	"os"
	"strings"
)

type OSInfo struct {
	ID        string
	VersionID string
	Family    string
}

func DetectOS() (*OSInfo, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return nil, err
	}

	info := &OSInfo{}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "ID=") {
			info.ID = strings.Trim(strings.Split(line, "=")[1], "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			info.VersionID = strings.Trim(strings.Split(line, "=")[1], "\"")
		}
	}

	switch info.ID {
	case "debian", "ubuntu":
		info.Family = "debian"
	case "centos", "rhel", "rocky", "almalinux", "oracle", "fedora":
		info.Family = "rhel"
	default:
		return nil, errors.New("unsupported OS")
	}

	return info, nil
}
