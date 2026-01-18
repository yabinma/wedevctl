package wedev

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/wedevctl/util"
)

func TestCreateVirtualNetwork_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	net, err := vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	if err != nil {
		t.Errorf("CreateVirtualNetwork() error = %v", err)
		return
	}

	if net.Name != "testnet" || net.CIDR != "10.0.0.0/24" {
		t.Errorf("CreateVirtualNetwork() returned unexpected values")
	}
}

func TestCreateVirtualNetwork_InvalidName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	tests := []struct {
		name  string
		cidr  string
		valid bool
	}{
		{"1invalid", "10.0.0.0/24", false},
		{"_invalid", "10.0.0.0/24", false},
		{"valid", "10.0.0.0/24", true},
		{"valid123", "10.0.0.0/24", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := vnm.CreateVirtualNetwork(tt.name, tt.cidr)
			if (err != nil) == tt.valid {
				t.Errorf("CreateVirtualNetwork(%q) got unexpected error: %v", tt.name, err)
			}
		})
	}
}

func TestCreateServer_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)

	if err != nil {
		t.Errorf("CreateServer() error = %v", err)
		return
	}

	if server.VirtualIP != "10.0.0.1" {
		t.Errorf("CreateServer() assigned IP = %s, want 10.0.0.1", server.VirtualIP)
	}

	if server.Port != 51820 {
		t.Errorf("CreateServer() port = %d, want 51820", server.Port)
	}
}

func TestCreateServer_DefaultPort(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	server, err := vnm.CreateServer("testnet", "server1", "192.168.1.1", 0)

	if err != nil {
		t.Errorf("CreateServer() error = %v", err)
		return
	}

	if server.Port != 51820 {
		t.Errorf("CreateServer() default port = %d, want 51820", server.Port)
	}
}

func TestCreateNode_Success(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	node, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)

	if err != nil {
		t.Errorf("CreateNode() error = %v", err)
		return
	}

	if node.VirtualIP != "10.0.0.2" {
		t.Errorf("CreateNode() assigned IP = %s, want 10.0.0.2", node.VirtualIP)
	}

	if node.Type != NodeTypePeer {
		t.Errorf("CreateNode() type = %v, want peer", node.Type)
	}
}

func TestCreateNode_DefaultType(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	node, err := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, "")

	if err != nil {
		t.Errorf("CreateNode() error = %v", err)
		return
	}

	if node.Type != NodeTypePeer {
		t.Errorf("CreateNode() default type = %v, want peer", node.Type)
	}
}

func TestDeleteNode_IPRecycling(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	node1, _ := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	_, _ = vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)

	// Delete node1
	err := vnm.DeleteNode("node1")
	if err != nil {
		t.Errorf("DeleteNode() error = %v", err)
		return
	}

	// Create another node - should reuse node1's IP
	node3, _ := vnm.CreateNode("testnet", "node3", "192.168.1.4", 51823, NodeTypePeer)

	if node3.VirtualIP != node1.VirtualIP {
		t.Errorf("CreateNode() should reuse recycled IP %s, got %s", node1.VirtualIP, node3.VirtualIP)
	}
}

func TestGenerateServerConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	server, _ := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	node1, _ := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	node2, _ := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypeRoute)

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, err := generator.GenerateConfigs("testnet", storage)
	if err != nil {
		t.Errorf("GenerateConfigs() error = %v", err)
		return
	}

	serverConfig := configs[server.Name]

	// Verify essential server config elements
	if !strings.Contains(serverConfig, "PrivateKey") {
		t.Errorf("Server config missing PrivateKey")
	}
	if !strings.Contains(serverConfig, "PostUp = sysctl -w net.ipv4.ip_forward=1") {
		t.Errorf("Server config missing PostUp directive")
	}
	if !strings.Contains(serverConfig, "PostDown = sysctl -w net.ipv4.ip_forward=0") {
		t.Errorf("Server config missing PostDown directive")
	}

	// Server should have peers for both nodes
	if !strings.Contains(serverConfig, node1.PublicKey) {
		t.Errorf("Server config missing peer for node1")
	}
	if !strings.Contains(serverConfig, node2.PublicKey) {
		t.Errorf("Server config missing peer for node2")
	}
}

func TestGeneratePeerNodeConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	server, _ := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	node1, _ := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	node2, _ := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, _ := generator.GenerateConfigs("testnet", storage)

	node1Config := configs[node1.Name]

	// Peer node should have server peer
	if !strings.Contains(node1Config, server.PublicKey) {
		t.Errorf("Peer node config missing server peer")
	}

	// Peer node should have other peer node as peer
	if !strings.Contains(node1Config, node2.PublicKey) {
		t.Errorf("Peer node config should include other peer node")
	}

	// Should have correct Endpoint format
	if !strings.Contains(node1Config, "192.168.1.1:51820") {
		t.Errorf("Peer node config missing correct server endpoint")
	}
}

func TestGenerateRouteNodeConfig(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	server, _ := vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)
	node1, _ := vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypeRoute)
	node2, _ := vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)

	generator := NewWireGuardConfigGenerator(storage)
	configs, _, _ := generator.GenerateConfigs("testnet", storage)

	routeNodeConfig := configs[node1.Name]

	// Route node should have server peer
	if !strings.Contains(routeNodeConfig, server.PublicKey) {
		t.Errorf("Route node config missing server peer")
	}

	// Route node should NOT have other nodes as peers
	if strings.Contains(routeNodeConfig, node2.PublicKey) {
		t.Errorf("Route node config should NOT include other nodes")
	}
}

func TestConfigVersionManagement(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)

	generator := NewWireGuardConfigGenerator(storage)

	// First save - should create v1
	config1, created1, _ := generator.SaveConfigVersion("testnet")
	if !created1 || config1.Version != 1 {
		t.Errorf("SaveConfigVersion() should create v1")
	}

	// Second save without changes - should not create new version
	config2, created2, _ := generator.SaveConfigVersion("testnet")
	if created2 {
		t.Errorf("SaveConfigVersion() should not create new version when content unchanged")
	}
	if config2.Version != 1 {
		t.Errorf("SaveConfigVersion() returned wrong version")
	}

	// Add node - should create v2
	vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	config3, created3, _ := generator.SaveConfigVersion("testnet")
	if !created3 || config3.Version != 2 {
		t.Errorf("SaveConfigVersion() should create v2 after node added")
	}
}

func TestConfigHistory(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)

	generator := NewWireGuardConfigGenerator(storage)

	// Create multiple versions
	generator.SaveConfigVersion("testnet")
	vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	generator.SaveConfigVersion("testnet")
	vnm.CreateNode("testnet", "node2", "192.168.1.3", 51822, NodeTypePeer)
	generator.SaveConfigVersion("testnet")

	// Get history
	history, err := generator.GetConfigHistory("testnet")
	if err != nil {
		t.Errorf("GetConfigHistory() error = %v", err)
		return
	}

	if len(history) != 3 {
		t.Errorf("GetConfigHistory() returned %d versions, want 3", len(history))
	}

	// Verify versions are in order
	for i, config := range history {
		if config.Version != i+1 {
			t.Errorf("GetConfigHistory() version %d at position %d", config.Version, i)
		}
	}
}

func TestGetSpecificConfigVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)

	generator := NewWireGuardConfigGenerator(storage)

	// Create multiple versions
	generator.SaveConfigVersion("testnet")
	vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	generator.SaveConfigVersion("testnet")

	// Get specific version
	config, err := generator.GetConfig("testnet", 1)
	if err != nil {
		t.Errorf("GetConfig() error = %v", err)
		return
	}

	if config.Version != 1 {
		t.Errorf("GetConfig() returned version %d, want 1", config.Version)
	}

	// Try to get non-existent version
	_, err = generator.GetConfig("testnet", 99)
	if err == nil {
		t.Errorf("GetConfig() should return error for non-existent version")
	}
}

func TestContentHashConsistency(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	storage, _ := NewStorageManager(dbPath)
	defer storage.Close()

	validator := util.NewDefaultIPValidator()
	vnm, _ := NewVirtualNetworkManager(storage, validator)

	vnm.CreateVirtualNetwork("testnet", "10.0.0.0/24")
	vnm.CreateServer("testnet", "server1", "192.168.1.1", 51820)

	generator := NewWireGuardConfigGenerator(storage)

	// Generate configs twice without changes
	_, hash1, _ := generator.GenerateConfigs("testnet", storage)
	_, hash2, _ := generator.GenerateConfigs("testnet", storage)

	if hash1 != hash2 {
		t.Errorf("Content hash should be consistent: %s != %s", hash1, hash2)
	}

	// Add node and regenerate
	vnm.CreateNode("testnet", "node1", "192.168.1.2", 51821, NodeTypePeer)
	_, hash3, _ := generator.GenerateConfigs("testnet", storage)

	if hash1 == hash3 {
		t.Errorf("Content hash should change when config changes")
	}
}
