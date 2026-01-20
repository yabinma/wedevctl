# wedevctl

A command-line tool for managing WireGuard virtual networks with automatic IP allocation, configuration generation, and version history tracking.

## Features

- 🌐 **Virtual Network Management** - Create and manage isolated WireGuard virtual networks with CIDR notation
- 🖥️ **Server & Node Management** - Configure servers and nodes (peer/route types) with automatic IP assignment
- 🔄 **IP Pool Management** - Automatic IP allocation from network CIDR with recycling on entity deletion
- ⚙️ **Config Generation** - Generate WireGuard configurations with proper peer relationships
- 📚 **Version History** - Track configuration changes with hash-based versioning
- 💾 **Persistent Storage** - Embedded BoltDB database (no external dependencies)
- 🗑️ **Cascade Deletion** - Remove networks with automatic cleanup of servers, nodes, and configs

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [User Guide](#user-guide)
  - [Creating a Virtual Network](#creating-a-virtual-network)
  - [Adding a Server](#adding-a-server)
  - [Adding Nodes](#adding-nodes)
  - [Generating WireGuard Configs](#generating-wireguard-configs)
  - [Managing Configurations](#managing-configurations)
  - [Editing Resources](#editing-resources)
  - [Deleting Resources](#deleting-resources)
- [WireGuard Setup](#wireguard-setup)
- [CLI Reference](#cli-reference)
- [Development](#development)
- [Testing](#testing)
- [License](#license)

## Installation

### Prerequisites

- Go 1.21 or later
- WireGuard installed on target systems (for running generated configs)

### Build from Source

```bash
# Clone the repository
git clone <repository-url>
cd wedevctl

# Build the binary
go build -o wedevctl main.go

# (Optional) Install to system path
sudo mv wedevctl /usr/local/bin/
```

### Verify Installation

```bash
wedevctl --help
```

## Quick Start

Here's a complete example to create a WireGuard network with a server and two peer nodes:

```bash
# 1. Create a virtual network
wedevctl vn add mynetwork 10.0.0.0/24

# 2. Add a server (will get IP 10.0.0.1)
wedevctl vn mynetwork server add server1 vpn.example.com 51820

# 3. Add peer nodes (will get IPs 10.0.0.2, 10.0.0.3, etc.)
wedevctl vn mynetwork node add laptop peer laptop.local 51821
wedevctl vn mynetwork node add phone peer phone.local 51822

# 4. Generate WireGuard configuration files
wedevctl vn mynetwork config generate --output-dir ./configs

# 5. View generated configs
ls -l ./configs/
# server1.conf  laptop.conf  phone.conf
```

## Configuration

### Database Location

By default, wedevctl stores all data in `~/.wedevctl/wedevctl.db`.

To use a custom database location, set the `WEDEVCTL_DB_PATH` environment variable:

```bash
# Use project-local database
export WEDEVCTL_DB_PATH=./data
wedevctl vn add test-net 10.0.0.0/24

# Use system-wide database (requires appropriate permissions)
export WEDEVCTL_DB_PATH=/var/lib/wedevctl
wedevctl vn add prod-net 10.0.0.0/24

# Use absolute path
export WEDEVCTL_DB_PATH=/mnt/shared/wedevctl
wedevctl vn list
```

**Notes:**
- The database file name is always `wedevctl.db`
- Relative paths are converted to absolute paths based on current working directory
- Directory permissions are automatically set to `0700` (owner read/write/execute only)
- The database directory is created automatically if it doesn't exist

### Multi-Environment Setup

You can manage multiple environments by using different database paths:

```bash
# Development environment
export WEDEVCTL_DB_PATH=~/.wedevctl/dev
wedevctl vn add dev-net 10.0.0.0/24

# Production environment
export WEDEVCTL_DB_PATH=~/.wedevctl/prod
wedevctl vn add prod-net 10.0.0.0/24
```

### Shell Configuration

To permanently set a custom database path, add it to your shell configuration:

```bash
# For bash (~/.bashrc)
echo 'export WEDEVCTL_DB_PATH=/path/to/your/db' >> ~/.bashrc
source ~/.bashrc

# For zsh (~/.zshrc)
echo 'export WEDEVCTL_DB_PATH=/path/to/your/db' >> ~/.zshrc
source ~/.zshrc
```

## User Guide

### Creating a Virtual Network

Virtual networks define an isolated WireGuard network with its own CIDR block.

```bash
# Create a network with CIDR notation
wedevctl vn add <network-name> <cidr>

# Example
wedevctl vn add production 10.10.0.0/24
wedevctl vn add development 192.168.100.0/24

# List all networks
wedevctl vn list
```

**Naming Rules:**
- Must start with a letter
- Can contain letters, numbers, and hyphens
- No special characters except hyphens

### Adding a Server

Each virtual network requires exactly one server. The server always receives the first IP from the CIDR range.

```bash
# Add a server
wedevctl vn <network> server add <server-name> <endpoint> <listen-port>

# Example - server gets 10.10.0.1
wedevctl vn production server add server1 vpn.mycompany.com 51820

# View server information
wedevctl vn production server info
```

**Server Configuration:**
- Automatically gets first IP (e.g., 10.10.0.1 from 10.10.0.0/24)
- Default port: 51820
- Endpoint can be a hostname or IP address
- Server configs include IP forwarding (PostUp/PostDown rules)

### Adding Nodes

Nodes are clients that connect to the network. There are two types:

#### Peer Nodes
Peer nodes can communicate with both the server and other peer nodes. **Peer nodes require a public address.**

```bash
# Add a peer node (public-address is required)
wedevctl vn <network> node add <node-name> peer <public-address> [port]

# Examples
wedevctl vn production node add laptop1 peer laptop1.local
wedevctl vn production node add desktop1 peer desktop1.local 51822
wedevctl vn production node add phone1 peer phone1.local 51823
```

#### Route Nodes
Route nodes only communicate with the server (not with other nodes). **Route nodes can optionally have a public address.**

```bash
# Add a route node without public address (uses WireGuard auto-discovery)
wedevctl vn production node add iot-device route

# Add a route node with public address
wedevctl vn production node add router1 route 192.168.1.1 51824
```

**Key Differences:**
- **Peer nodes**: Must have public address, can connect peer-to-peer
- **Route nodes**: Public address optional, only connects to server
- In server config: peer nodes have Endpoint, route nodes don't (server waits for connection)

**IP Assignment:**
- Nodes automatically receive sequential IPs (10.10.0.2, 10.10.0.3, etc.)
- IPs are recycled when nodes are deleted

**List all nodes:**
```bash
wedevctl vn production node list
```

### Generating WireGuard Configs

Generate configuration files for all entities in a network:

```bash
# Generate configs to current directory
wedevctl vn <network> config generate

# Generate to specific directory
wedevctl vn production config generate --output-dir /etc/wireguard

# Force overwrite existing files
wedevctl vn production config generate --output-dir ./configs --force
```

**Generated Files:**
- One `.conf` file per server/node
- Named after the entity (e.g., `server1.conf`, `laptop1.conf`)
- Ready to use with WireGuard

**Configuration Features:**
- **Server config**: 
  - Includes IP forwarding rules (PostUp/PostDown)
  - Peer nodes: includes Endpoint (for direct connection)
  - Route nodes: no Endpoint (server waits for connection from route node)
- **Peer node config**: Includes peers for server + all other peer nodes with Endpoints
- **Route node config**: Only includes peer for the server with Endpoint

**Endpoint Behavior:**
- Server always needs to know peer node addresses (includes Endpoint)
- Server learns route node addresses dynamically (no Endpoint needed)
- All nodes know server address (always includes Endpoint in node configs)

**Version Tracking:**
Configurations are automatically versioned when generated. Each unique configuration gets a new version number with content hash tracking.

### Managing Configurations

#### View Configuration History

```bash
# View all configuration versions for a network
wedevctl vn production config history

# Output shows:
# Version | Created At           | Content Hash
# 1       | 2026-01-18 10:30:00 | a1b2c3d4...
# 2       | 2026-01-18 11:45:00 | e5f6g7h8...
```

#### View Specific Configuration

```bash
# View latest configuration info
wedevctl vn production config info

# View specific version
wedevctl vn production config info 1
```

### Editing Resources

#### Edit Server

```bash
# Edit server endpoint
wedevctl vn production server edit --endpoint new.vpn.example.com

# Edit server listen port
wedevctl vn production server edit --listen-port 51821

# Edit both
wedevctl vn production server edit --endpoint new.vpn.example.com --listen-port 51821
```

#### Edit Node

```bash
# Edit node public address
wedevctl vn production node edit laptop1 --public-address new-laptop.local

# Edit node listen port
wedevctl vn production node edit laptop1 --port 51830

# Change node type (peer requires public address)
wedevctl vn production node edit laptop1 --type route

# Edit multiple properties
wedevctl vn production node edit laptop1 --type peer --public-address new-laptop.local --port 51830
```

**Validation Rules:**
- When changing type to `peer`: public address is required
- When changing type to `route`: public address is optional and can be cleared
- Peer nodes cannot have their public address cleared (change to route type first)

**After Editing:**
Regenerate configurations to apply changes:
```bash
wedevctl vn production config generate --output-dir /etc/wireguard --force
```

### Deleting Resources

#### Delete a Node

```bash
# Delete a node (IP is returned to pool)
wedevctl vn production node delete laptop1
```

#### Delete a Server

```bash
# Delete the server (also removes all nodes)
wedevctl vn production server delete
```

#### Delete a Network

```bash
# Delete entire network (removes server, nodes, and all configs)
wedevctl vn delete production
```

**Cascade Deletion:**
- Deleting a network removes all servers, nodes, and configurations
- Deleting a server removes all nodes
- All IPs are returned to the pool for reuse

## WireGuard Setup

After generating configuration files, set up WireGuard on each machine:

### Linux

```bash
# 1. Copy the config file to WireGuard directory
sudo cp server1.conf /etc/wireguard/wg0.conf

# 2. Start WireGuard interface
sudo wg-quick up wg0

# 3. Enable on boot (optional)
sudo systemctl enable wg-quick@wg0

# 4. Check status
sudo wg show

# 5. Stop interface
sudo wg-quick down wg0
```

### macOS

```bash
# Using Homebrew
brew install wireguard-tools

# Copy config
sudo mkdir -p /usr/local/etc/wireguard
sudo cp server1.conf /usr/local/etc/wireguard/wg0.conf

# Start interface
sudo wg-quick up wg0

# Check status
sudo wg show
```

### Troubleshooting WireGuard

```bash
# Check if interface is up
ip link show wg0

# View WireGuard logs
sudo journalctl -u wg-quick@wg0

# Test connectivity
ping 10.0.0.1  # Ping server from node

# Check peer handshake
sudo wg show wg0 latest-handshakes
```

## CLI Reference

### Virtual Network Commands

```bash
vn add <name> <cidr>              # Create virtual network
vn list                            # List all networks
vn delete <name>                   # Delete network (cascade)
```

### Server Commands

```bash
vn <network> server add <name> <endpoint> <port>     # Add server
vn <network> server info                              # Show server info
vn <network> server edit [--endpoint] [--listen-port]  # Edit server
vn <network> server delete                            # Delete server
```

### Node Commands

```bash
vn <network> node add <name> <type> [public-address] [port]  # Add node (type: peer|route)
                                                              # peer: public-address required
                                                              # route: public-address optional
vn <network> node list                                        # List all nodes
vn <network> node edit <name> [--type] [--public-address] [--port]  # Edit node
vn <network> node delete <name>                               # Delete node
```

### Configuration Commands

```bash
vn <network> config generate [--output-dir dir] [--force]  # Generate configs
vn <network> config history                                 # View config history
vn <network> config info [version]                          # View config info
```

## Development

### Project Structure

```
wedevctl/
├── main.go                 # Entry point
├── cmd/
│   ├── root.go            # CLI commands (Cobra)
│   └── root_test.go       # CLI tests
├── wedev/
│   ├── manager.go         # Business logic
│   ├── manager_test.go    # Manager tests
│   ├── storage.go         # BoltDB persistence
│   └── storage_test.go    # Storage tests
├── util/
│   ├── util.go            # Utilities & IP pool
│   └── util_test.go       # Utility tests
└── go.mod                 # Dependencies
```

### Dependencies

- **spf13/cobra** v1.7.0 - CLI framework
- **go.etcd.io/bbolt** v1.3.8 - Embedded database
- **github.com/google/uuid** v1.5.0 - UUID generation

### Building

```bash
# Build
go build -o wedevctl main.go

# Build with optimizations
go build -ldflags="-s -w" -o wedevctl main.go
```

### CI/CD Pipeline

The project uses GitHub Actions for continuous integration. All pull requests to the `main` branch must pass the following checks before merging:

#### Automated Checks

1. **Static Analysis**
   - Go formatting check (`gofmt`)
   - Static analysis (`go vet`)
   - Comprehensive linting (`golangci-lint`)
   - Dependency verification

2. **Test Suite**
   - All unit tests with race detection
   - Code coverage verification (package-specific requirements)
   - Coverage reporting

3. **Build Verification**
   - Binary compilation check
   - Executable verification

4. **Security Scanning**
   - Gosec security analysis
   - Vulnerability detection

#### PR Workflow

```yaml
# Triggered on: Pull requests to main branch
# Required checks: All must pass (static-analysis, test, build, security-scan)
# Merge policy: Only allowed after all checks succeed
```

To ensure your PR passes all checks locally:

```bash
# Format code
go fmt ./...

# Run static analysis
go vet ./...

# Run tests with coverage
go test -v -race -coverprofile=coverage.out ./...

# Check coverage
go tool cover -func=coverage.out

# Build
go build -o wedevctl main.go
```

## Testing

The project includes comprehensive test coverage (~70% overall):

### Run All Tests

```bash
# Run all tests
go test ./...

# Run with verbose output
go test ./... -v

# Run with coverage
go test ./... -cover
```

### Run Specific Package Tests

```bash
# Utility tests (28 tests, 89.0% coverage)
go test ./util -v

# Storage tests (16 tests)
go test ./wedev -v

# CLI tests (14 tests)
go test ./cmd -v
```

### Test Categories

- **Validation Tests**: Network name, CIDR, endpoint, port validation
- **IP Pool Tests**: Allocation, recycling, persistence
- **Network Management Tests**: CRUD operations, cascade deletion
- **Configuration Tests**: Generation, versioning, history
- **Storage Tests**: BoltDB operations, transaction consistency

## Examples

### Example 1: Simple Office Network

```bash
# Create network for office (10.20.0.0/24)
wedevctl vn add office 10.20.0.0/24

# Add office VPN server
wedevctl vn office server add vpn-server vpn.company.com 51820

# Add employee devices
wedevctl vn office node add ceo-laptop ceo-laptop.local 51821 peer
wedevctl vn office node add cto-desktop cto-desktop.local 51822 peer
wedevctl vn office node add sales-laptop sales-laptop.local 51823 peer

# Generate configs
wedevctl vn office config generate --output-dir ~/wireguard-configs

# Check history
wedevctl vn office config history
```

### Example 2: IoT Network with Route Nodes

```bash
# Create IoT network
wedevctl vn add iot 192.168.50.0/24

# Add IoT gateway server
wedevctl vn iot server add iot-gateway gateway.iot.local 51820

# Add IoT devices as route nodes (only talk to gateway)
wedevctl vn iot node add camera1 camera1.iot.local 51821 route
wedevctl vn iot node add sensor1 sensor1.iot.local 51822 route
wedevctl vn iot node add thermostat thermostat.iot.local 51823 route

# Add admin laptop as peer (can access all devices)
wedevctl vn iot node add admin-laptop admin.local 51824 peer

# Generate configs
wedevctl vn iot config generate --output-dir /etc/wireguard --force
```

### Example 3: Multi-Environment Setup with Isolated Databases

Manage separate development and production environments with isolated databases:

```bash
# Development environment
export WEDEVCTL_DB_PATH=~/.wedevctl/dev

wedevctl vn add dev-network 10.100.0.0/24
wedevctl vn dev-network server add dev-server dev.example.com 51820
wedevctl vn dev-network node add dev-laptop1 dev1.local 51821 peer
wedevctl vn dev-network config generate --output-dir ./configs/dev

# Production environment (separate database)
export WEDEVCTL_DB_PATH=~/.wedevctl/prod

wedevctl vn add prod-network 10.200.0.0/24
wedevctl vn prod-network server add prod-server prod.example.com 51820
wedevctl vn prod-network node add prod-laptop1 prod1.local 51821 peer
wedevctl vn prod-network config generate --output-dir ./configs/prod

# List networks in current environment (production)
wedevctl vn list
```

## License

Copyright 2026

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass: `go test ./...`
2. Code follows Go conventions: `go fmt ./...`
3. Add tests for new features
4. Update documentation as needed

## Support

For issues, questions, or contributions, please refer to the project repository.

## Acknowledgments

Built with:
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [BoltDB](https://github.com/etcd-io/bbolt) - Embedded database
- [WireGuard](https://www.wireguard.com/) - Modern VPN protocol