// Package wedev provides virtual network and WireGuard configuration management.
package wedev

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/wedevctl/util"
)

// VirtualNetworkManager manages virtual networks and their resources
type VirtualNetworkManager struct {
	storage   *StorageManager
	ipPools   map[string]*util.IPPool // networkID -> IPPool
	validator util.IPValidator
}

// NewVirtualNetworkManager creates a new VirtualNetworkManager
func NewVirtualNetworkManager(storage *StorageManager, validator util.IPValidator) (*VirtualNetworkManager, error) {
	return &VirtualNetworkManager{
		storage:   storage,
		ipPools:   make(map[string]*util.IPPool),
		validator: validator,
	}, nil
}

// ensureIPPool ensures an IP pool exists for the network and is properly initialized
// with all existing IP allocations from the database
func (vnm *VirtualNetworkManager) ensureIPPool(networkID, networkCIDR string) error {
	if _, exists := vnm.ipPools[networkID]; exists {
		return nil
	}

	// Create new IP pool
	ipPool, err := util.NewIPPool(networkCIDR)
	if err != nil {
		return fmt.Errorf("failed to create IP pool: %w", err)
	}

	// Load existing server and mark its IP as allocated
	server, err := vnm.storage.GetServerByNetworkID(networkID)
	if err == nil && server != nil {
		// Server IP is already reserved by GetServerIP(), but we need to mark it as allocated
		if markErr := ipPool.MarkIPAllocated(server.VirtualIP); markErr != nil {
			return fmt.Errorf("failed to mark server IP as allocated: %w", markErr)
		}
	}

	// Load existing nodes and mark their IPs as allocated
	nodes, err := vnm.storage.ListNodesByNetworkID(networkID)
	if err != nil {
		return fmt.Errorf("failed to load existing nodes: %w", err)
	}

	// Mark all existing node IPs as allocated
	for _, node := range nodes {
		if markErr := ipPool.MarkIPAllocated(node.VirtualIP); markErr != nil {
			// If IP is already allocated, it means we have duplicate IPs in the database
			// This is a data integrity issue, but we'll log it and continue
			// rather than failing completely
			fmt.Fprintf(os.Stderr, "Warning: duplicate IP detected for node %s: %s\n", node.Name, node.VirtualIP)
		}
	}

	// Sync nextIndex to ensure new allocations don't conflict with existing ones
	ipPool.SyncNextIndex()

	vnm.ipPools[networkID] = ipPool
	return nil
}

// CreateVirtualNetwork creates a new virtual network.
func (vnm *VirtualNetworkManager) CreateVirtualNetwork(name, cidr string) (*VirtualNetwork, error) {
	// Validate input
	if err := vnm.validator.IsValidNetworkName(name); err != nil {
		return nil, err
	}
	if err := vnm.validator.IsValidCIDR(cidr); err != nil {
		return nil, err
	}

	// Create IP pool
	ipPool, err := util.NewIPPool(cidr)
	if err != nil {
		return nil, fmt.Errorf("failed to create IP pool: %w", err)
	}

	// Create network in storage
	network, err := vnm.storage.CreateNetwork(name, cidr)
	if err != nil {
		return nil, err
	}

	// Store IP pool
	vnm.ipPools[network.ID] = ipPool

	return network, nil
}

// GetVirtualNetwork retrieves a virtual network by name
func (vnm *VirtualNetworkManager) GetVirtualNetwork(name string) (*VirtualNetwork, error) {
	return vnm.storage.GetNetworkByName(name)
}

// ListVirtualNetworks lists all virtual networks
func (vnm *VirtualNetworkManager) ListVirtualNetworks() ([]*VirtualNetwork, error) {
	return vnm.storage.ListNetworks()
}

// DeleteVirtualNetwork deletes a virtual network
func (vnm *VirtualNetworkManager) DeleteVirtualNetwork(name string) error {
	network, err := vnm.storage.GetNetworkByName(name)
	if err != nil {
		return err
	}

	// Remove IP pool
	delete(vnm.ipPools, network.ID)

	return vnm.storage.DeleteNetwork(name)
}

// CreateServer creates a new server in the network.
func (vnm *VirtualNetworkManager) CreateServer(networkName, serverName, publicAddress string, port int) (*Server, error) {
	// Get network
	network, err := vnm.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	// Validate input
	if valErr := vnm.validator.IsValidPublicAddress(publicAddress); valErr != nil {
		return nil, valErr
	}

	// Ensure IP pool exists and is properly initialized
	if err := vnm.ensureIPPool(network.ID, network.CIDR); err != nil {
		return nil, err
	}

	// Server always gets the first usable IP
	serverIP := vnm.ipPools[network.ID].GetServerIP()

	// Generate keys
	keys, err := util.GenerateWireGuardKeys()
	if err != nil {
		return nil, err
	}

	// Set default port if not specified
	if port == 0 {
		port = 51820
	}

	// Create server in storage
	server, err := vnm.storage.CreateServer(network.ID, serverName, publicAddress, port, serverIP, keys.PrivateKey, keys.PublicKey)
	if err != nil {
		return nil, err
	}

	return server, nil
}

// GetServer retrieves a server by name
func (vnm *VirtualNetworkManager) GetServer(networkName string) (*Server, error) {
	network, err := vnm.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	return vnm.storage.GetServerByNetworkID(network.ID)
}

// UpdateServer updates server information.
func (vnm *VirtualNetworkManager) UpdateServer(networkName, publicAddress string, port int) (*Server, error) {
	// Get network and server
	network, err := vnm.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	server, err := vnm.storage.GetServerByNetworkID(network.ID)
	if err != nil {
		return nil, err
	}

	// Validate new public address
	if valErr := vnm.validator.IsValidPublicAddress(publicAddress); valErr != nil {
		return nil, valErr
	}

	// Update in storage
	if updateErr := vnm.storage.UpdateServer(server.ID, publicAddress, port); updateErr != nil {
		return nil, updateErr
	}

	// Retrieve updated server
	return vnm.storage.GetServerByName(server.Name)
}

// DeleteServer deletes the server from a network
func (vnm *VirtualNetworkManager) DeleteServer(networkName string) error {
	network, err := vnm.storage.GetNetworkByName(networkName)
	if err != nil {
		return err
	}

	return vnm.storage.DeleteServer(network.ID)
}

// CreateNode creates a new node in the network.
func (vnm *VirtualNetworkManager) CreateNode(networkName, nodeName, publicAddress string, port int, nodeType NodeType) (*Node, error) {
	// Get network
	network, err := vnm.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	// Validate input: peer type requires public address, route type is optional
	if nodeType == NodeTypePeer && publicAddress == "" {
		return nil, fmt.Errorf("peer type nodes require a public address")
	}
	if publicAddress != "" {
		if valErr := vnm.validator.IsValidPublicAddress(publicAddress); valErr != nil {
			return nil, valErr
		}
	}

	// Ensure IP pool exists and is properly initialized
	if err := vnm.ensureIPPool(network.ID, network.CIDR); err != nil {
		return nil, err
	}

	// Allocate IP for node
	nodeIP, err := vnm.ipPools[network.ID].AllocateNodeIP()
	if err != nil {
		return nil, err
	}

	// Generate keys
	keys, err := util.GenerateWireGuardKeys()
	if err != nil {
		// Free the IP if key generation fails
		//nolint:errcheck // Acceptable to ignore in error cleanup path
		_ = vnm.ipPools[network.ID].ReleaseNodeIP(nodeIP)
		return nil, err
	}

	// Set default port if not specified
	if port == 0 {
		port = 51820
	}

	// Default type is peer
	if nodeType == "" {
		nodeType = NodeTypePeer
	}

	// Create node in storage
	node, err := vnm.storage.CreateNode(network.ID, nodeName, publicAddress, port, nodeIP, nodeType, keys.PrivateKey, keys.PublicKey)
	if err != nil {
		// Free the IP if node creation fails
		//nolint:errcheck // Acceptable to ignore in error cleanup path
		_ = vnm.ipPools[network.ID].ReleaseNodeIP(nodeIP)
		return nil, err
	}

	return node, nil
}

// GetNode retrieves a node by name
func (vnm *VirtualNetworkManager) GetNode(nodeName string) (*Node, error) {
	return vnm.storage.GetNodeByName(nodeName)
}

// ListNodes lists all nodes in a network
func (vnm *VirtualNetworkManager) ListNodes(networkName string) ([]*Node, error) {
	network, err := vnm.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	return vnm.storage.ListNodesByNetworkID(network.ID)
}

// UpdateNode updates node information.
func (vnm *VirtualNetworkManager) UpdateNode(nodeName, publicAddress string, port int, nodeType NodeType) (*Node, error) {
	// Get node
	node, err := vnm.storage.GetNodeByName(nodeName)
	if err != nil {
		return nil, err
	}

	// Validate: peer type requires public address, route type is optional
	if nodeType == NodeTypePeer && publicAddress == "" {
		return nil, fmt.Errorf("peer type nodes require a public address")
	}
	if publicAddress != "" {
		if valErr := vnm.validator.IsValidPublicAddress(publicAddress); valErr != nil {
			return nil, valErr
		}
	}

	// Update in storage
	if err := vnm.storage.UpdateNode(node.ID, publicAddress, port, nodeType); err != nil {
		return nil, err
	}

	// Retrieve updated node
	return vnm.storage.GetNodeByName(nodeName)
}

// DeleteNode deletes a node
func (vnm *VirtualNetworkManager) DeleteNode(nodeName string) error {
	// Get node
	node, err := vnm.storage.GetNodeByName(nodeName)
	if err != nil {
		return err
	}

	// Free the IP
	if _, exists := vnm.ipPools[node.NetworkID]; exists {
		//nolint:errcheck // Acceptable to ignore in cleanup path
		_ = vnm.ipPools[node.NetworkID].ReleaseNodeIP(node.VirtualIP)
	}

	return vnm.storage.DeleteNode(nodeName)
}

// ========== WireGuard Configuration Generation ==========

// WireGuardConfigGenerator generates WireGuard configurations
type WireGuardConfigGenerator struct {
	storage *StorageManager
}

// NewWireGuardConfigGenerator creates a new WireGuardConfigGenerator
func NewWireGuardConfigGenerator(storage *StorageManager) *WireGuardConfigGenerator {
	return &WireGuardConfigGenerator{storage: storage}
}

// GenerateConfigs generates WireGuard configurations for all entities in a network.
func (wcg *WireGuardConfigGenerator) GenerateConfigs(networkName string, storage *StorageManager) (configs map[string]string, hash string, err error) {
	// Get network
	network, err := storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, "", err
	}

	// Get server
	server, sErr := storage.GetServerByNetworkID(network.ID)
	if sErr != nil {
		return nil, "", fmt.Errorf("no server found in network")
	}

	// Get all nodes
	nodes, nErr := storage.ListNodesByNetworkID(network.ID)
	if nErr != nil {
		return nil, "", nErr
	}

	// Generate server config
	serverConfig := wcg.generateServerConfig(network, server, nodes)

	// Generate node configs
	nodeConfigs := make(map[string]string)
	for _, node := range nodes {
		nodeConfigs[node.Name] = wcg.generateNodeConfig(network, server, node, nodes)
	}

	// Combine all configs
	allConfigs := make(map[string]string)
	allConfigs[server.Name] = serverConfig
	for name, config := range nodeConfigs {
		allConfigs[name] = config
	}

	// Calculate content hash
	contentHash := wcg.calculateConfigHash(allConfigs)

	return allConfigs, contentHash, nil
}

// generateServerConfig generates the server configuration.
func (wcg *WireGuardConfigGenerator) generateServerConfig(_ *VirtualNetwork, server *Server, nodes []*Node) string {
	var config strings.Builder

	config.WriteString("[Interface]\n")
	config.WriteString(fmt.Sprintf("PrivateKey = %s\n", server.PrivateKey))
	config.WriteString(fmt.Sprintf("Address = %s/32\n", server.VirtualIP))
	config.WriteString(fmt.Sprintf("ListenPort = %d\n", server.Port))
	config.WriteString("PostUp = sysctl -w net.ipv4.ip_forward=1\n")
	config.WriteString("PostDown = sysctl -w net.ipv4.ip_forward=0\n")

	// Add peer for each node
	for _, node := range nodes {
		config.WriteString("\n[Peer]\n")
		config.WriteString(fmt.Sprintf("PublicKey = %s\n", node.PublicKey))
		config.WriteString(fmt.Sprintf("AllowedIPs = %s/32\n", node.VirtualIP))
		// Only add Endpoint for peer type nodes (route nodes connect to server, not vice versa)
		if node.Type == NodeTypePeer && node.PublicAddress != "" {
			endpoint := util.FormatEndpoint(node.PublicAddress, node.Port)
			config.WriteString(fmt.Sprintf("Endpoint = %s\n", endpoint))
		}
	}

	return config.String()
}

// generateNodeConfig generates a configuration for a specific node
func (wcg *WireGuardConfigGenerator) generateNodeConfig(network *VirtualNetwork, server *Server, node *Node, allNodes []*Node) string {
	var config strings.Builder

	config.WriteString("[Interface]\n")
	config.WriteString(fmt.Sprintf("PrivateKey = %s\n", node.PrivateKey))
	config.WriteString(fmt.Sprintf("Address = %s/32\n", node.VirtualIP))
	config.WriteString(fmt.Sprintf("ListenPort = %d\n", node.Port))

	// Add server peer
	config.WriteString("\n[Peer]\n")
	config.WriteString(fmt.Sprintf("PublicKey = %s\n", server.PublicKey))
	config.WriteString(fmt.Sprintf("AllowedIPs = %s\n", network.CIDR))
	if server.PublicAddress != "" {
		endpoint := util.FormatEndpoint(server.PublicAddress, server.Port)
		config.WriteString(fmt.Sprintf("Endpoint = %s\n", endpoint))
	}

	// For peer type nodes, add peer connections to other peer nodes
	if node.Type == NodeTypePeer {
		for _, otherNode := range allNodes {
			if otherNode.ID != node.ID && otherNode.Type == NodeTypePeer {
				config.WriteString("\n[Peer]\n")
				config.WriteString(fmt.Sprintf("PublicKey = %s\n", otherNode.PublicKey))
				config.WriteString(fmt.Sprintf("AllowedIPs = %s/32\n", otherNode.VirtualIP))
				if otherNode.PublicAddress != "" {
					endpoint := util.FormatEndpoint(otherNode.PublicAddress, otherNode.Port)
					config.WriteString(fmt.Sprintf("Endpoint = %s\n", endpoint))
				}
			}
		}
	}

	return config.String()
}

// calculateConfigHash calculates the hash of all configurations
func (wcg *WireGuardConfigGenerator) calculateConfigHash(configs map[string]string) string {
	// Sort config names for consistent hashing
	names := make([]string, 0, len(configs))
	for name := range configs {
		names = append(names, name)
	}
	sort.Strings(names)

	// Concatenate all configs in sorted order
	var combined strings.Builder
	for _, name := range names {
		combined.WriteString(name)
		combined.WriteString(":")
		combined.WriteString(configs[name])
	}

	// Calculate SHA256 hash
	hash := sha256.Sum256([]byte(combined.String()))
	return hex.EncodeToString(hash[:])
}

// SaveConfigVersion saves a configuration version if content has changed
func (wcg *WireGuardConfigGenerator) SaveConfigVersion(networkName string) (*ConfigVersion, bool, error) {
	// Generate current configs
	configs, currentHash, err := wcg.GenerateConfigs(networkName, wcg.storage)
	if err != nil {
		return nil, false, err
	}

	// Get network
	network, err := wcg.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, false, err
	}

	// Check if latest version has same hash
	latest, err := wcg.storage.GetLatestConfigVersion(network.ID)
	if err == nil && latest.ContentHash == currentHash {
		// No change
		return latest, false, nil
	}

	// Save new version
	version, err := wcg.storage.SaveConfigVersion(network.ID, currentHash, configs)
	if err != nil {
		return nil, false, err
	}

	return version, true, nil
}

// GetConfigHistory retrieves the configuration history for a network
func (wcg *WireGuardConfigGenerator) GetConfigHistory(networkName string) ([]*ConfigVersion, error) {
	network, err := wcg.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	return wcg.storage.ListConfigVersions(network.ID)
}

// GetConfig retrieves a specific configuration version
func (wcg *WireGuardConfigGenerator) GetConfig(networkName string, version int) (*ConfigVersion, error) {
	network, err := wcg.storage.GetNetworkByName(networkName)
	if err != nil {
		return nil, err
	}

	return wcg.storage.GetConfigVersion(network.ID, version)
}
