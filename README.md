# 🐘 dbcp-agent

**dbcp-agent** is a modular, secure, and configurable agent written in Go for managing open-source database clusters — starting with PostgreSQL. It installs, configures, and monitors PostgreSQL and related services like ETCD, Patroni, HAProxy, and pgBackRest, following the best practices for automation and cluster resilience.

---

## 🚀 Features

- 🔧 Install and configure PostgreSQL, ETCD, Patroni, and more
- 🌐 OS detection and package management for Debian/Ubuntu and RHEL-based systems
- 📦 Support for both official and custom repositories
- 🗃️ Modular configuration per node with cluster coordination
- 🔐 TLS-enabled secure communication and certificate validation
- 🔍 Health checks and service orchestration
- 🪵 Structured logging with log levels and rotation
- 🧪 Dry-run capability (coming soon)
- 📈 Metrics and observability (eBPF/OpenTelemetry planned)

---

## 🗂️ Project Structure

```
dbcp-agent/
├── cmd/
│   └── dbcp-agent/        # CLI entrypoint
├── internal/
│   ├── agent/             # Core coordination logic (TBD)
│   ├── config/            # YAML config loading and validation
│   ├── pkg/               # PostgreSQL and ETCD logic
│   ├── logger/            # Structured logger with levels
│   └── system/            # OS detection
├── configs/               # Example agent-config.yaml
├── scripts/               # TLS & helper scripts
├── .devcontainer/         # VSCode Dev Container setup (multi-node)
├── docker-compose.yml     # Cluster orchestration
├── Makefile               # Build/test/run
├── go.mod / go.sum        # Go module files
└── README.md              # You are here :)
```

---

## ⚙️ Configuration

Each node has its own config:

```yaml
node:
  host: "db-node-1"
  role: "database"
  postgresql:
    version: "15"
    data_dir: "/var/lib/postgresql/15/data"
    bin_path: "/usr/lib/postgresql/15/bin"
    user: "postgres"
  etcd:
    version: "3.5.9"
    data_dir: "/var/lib/etcd"
    bin_path: "/usr/local/bin"
    cert_file: "/etc/etcd/certs/etcd.crt"
    key_file: "/etc/etcd/certs/etcd.key"
    ca_file: "/etc/etcd/certs/ca.crt"
    peer_port: 2380
    client_port: 2379
    cluster_mode: "bootstrap"  # or "join"

cluster:
  name: "pg-cluster"
  nodes:
    - host: "db-node-1"
    - host: "db-node-2"
    - host: "db-node-3"

repositories:
  postgresql:
    default: "official"
    sources:
      official:
        debian: "https://apt.postgresql.org/pub/repos/apt"
        rhel: "https://download.postgresql.org/pub/repos/yum"
      custom:
        debian: "https://internal.example.com/postgres/apt"
        rhel: "https://internal.example.com/postgres/yum"
  etcd:
    default: "official"
    sources:
      official:
        url: "https://storage.googleapis.com/etcd"
      custom:
        url: "https://internal.example.com/etcd"

log_level: "info"
log_output: "stdout"  # or "file"
log_file_path: "/var/log/dbcp-agent.log"
log_max_size_mb: 10
log_max_backups: 3
log_max_age_days: 7
```

---

## 🐳 Running with Docker (Dev Mode)

```bash
# Build dev containers
docker compose up -d --build

# Attach to node
docker exec -it dbcp-node-1 bash

# Run agent (automatically loads config from configs/agent-config.yaml)
./dbcp-agent
```

---

## 🛠️ Building & Testing

```bash
make build      # Compile the agent
make test       # Run unit tests
make lint       # Lint the code (optional)
```

---

## 📋 Roadmap

- [x] PostgreSQL installer and service
- [x] ETCD TLS cluster bootstrap/join
- [x] Patroni installer
- [ ] Cluster control
- [ ] HAProxy installer & PostgreSQL routing config
- [ ] Logical backups with pgBackRest
- [ ] API Server for central orchestration
- [ ] Live metrics export via OpenTelemetry

---

## 🤝 Contributing

Want to contribute PostgreSQL/ETCD cluster logic, help build metrics exporters, or improve the agent-core? Feel free to open issues, fork, and send PRs!

---

## 🧑‍💻 Maintainer

**Charly Batista**  

---

## 📜 License

MIT License. See `LICENSE` file for details.
