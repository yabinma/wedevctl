package wedev

import (
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
	"go.etcd.io/bbolt"
)

const (
	// BucketNetworks is the BoltDB bucket for network data.
	BucketNetworks = "networks"
	// BucketNetworksByName is the index bucket for networks by name.
	BucketNetworksByName = "networks_by_name"
	// BucketServers is the BoltDB bucket for server data.
	BucketServers = "servers"
	// BucketServersByName is the index bucket for servers by name.
	BucketServersByName = "servers_by_name"
	// BucketNodes is the BoltDB bucket for node data.
	BucketNodes = "nodes"
	// BucketNodesByName is the index bucket for nodes by name.
	BucketNodesByName = "nodes_by_name"
	// BucketConfigs is the BoltDB bucket for config data.
	BucketConfigs = "configs"
	// BucketConfigsByVer is the index bucket for configs by version.
	BucketConfigsByVer = "configs_by_version"
	// BucketIPPools is the BoltDB bucket for IP pool data.
	BucketIPPools = "ip_pools"
)

// VirtualNetwork represents a virtual network
type VirtualNetwork struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CIDR      string    `json:"cidr"`
	CreatedAt time.Time `json:"created_at"`
}

// Server represents a WireGuard server
type Server struct {
	ID            string    `json:"id"`
	NetworkID     string    `json:"network_id"`
	Name          string    `json:"name"`
	PublicAddress string    `json:"public_address"`
	Port          int       `json:"port"`
	VirtualIP     string    `json:"virtual_ip"`
	PrivateKey    string    `json:"private_key"`
	PublicKey     string    `json:"public_key"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// NodeType represents the type of node
type NodeType string

const (
	// NodeTypePeer represents a peer node.
	NodeTypePeer NodeType = "peer"
	// NodeTypeRoute represents a route node.
	NodeTypeRoute NodeType = "route"
)

// Node represents a node in the network
type Node struct {
	ID            string    `json:"id"`
	NetworkID     string    `json:"network_id"`
	Name          string    `json:"name"`
	PublicAddress string    `json:"public_address"`
	Port          int       `json:"port"`
	VirtualIP     string    `json:"virtual_ip"`
	Type          NodeType  `json:"type"`
	PrivateKey    string    `json:"private_key"`
	PublicKey     string    `json:"public_key"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// ConfigVersion represents a snapshot of WireGuard configurations
type ConfigVersion struct {
	ID          string            `json:"id"`
	NetworkID   string            `json:"network_id"`
	Version     int               `json:"version"`
	ContentHash string            `json:"content_hash"`
	Configs     map[string]string `json:"configs"` // name -> config content
	CreatedAt   time.Time         `json:"created_at"`
}

// StorageManager handles all BoltDB operations
type StorageManager struct {
	db *bbolt.DB
}

// NewStorageManager creates a new storage manager.
func NewStorageManager(dbPath string) (*StorageManager, error) {
	db, err := bbolt.Open(dbPath, 0o600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Initialize buckets
	if err := db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{
			BucketNetworks, BucketNetworksByName,
			BucketServers, BucketServersByName,
			BucketNodes, BucketNodesByName,
			BucketConfigs, BucketConfigsByVer,
			BucketIPPools,
		}
		for _, bucketName := range buckets {
			if _, err := tx.CreateBucketIfNotExists([]byte(bucketName)); err != nil {
				return fmt.Errorf("failed to create bucket %s: %w", bucketName, err)
			}
		}
		return nil
	}); err != nil {
		db.Close()
		return nil, err
	}

	return &StorageManager{db: db}, nil
}

// Close closes the database
func (sm *StorageManager) Close() error {
	return sm.db.Close()
}

// ========== VirtualNetwork Operations ==========

// CreateNetwork creates a new virtual network.
func (sm *StorageManager) CreateNetwork(name, cidr string) (*VirtualNetwork, error) {
	var network *VirtualNetwork

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Check if name already exists
		nameIdx := tx.Bucket([]byte(BucketNetworksByName))
		if nameIdx.Get([]byte(name)) != nil {
			return fmt.Errorf("network name %q already exists", name)
		}

		network = &VirtualNetwork{
			ID:        uuid.New().String(),
			Name:      name,
			CIDR:      cidr,
			CreatedAt: time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(network)
		if err != nil {
			return fmt.Errorf("failed to marshal network: %w", err)
		}
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if err := networksBucket.Put([]byte(network.ID), data); err != nil {
			return fmt.Errorf("failed to save network: %w", err)
		}

		// Save to secondary bucket (name -> id)
		nameIdx = tx.Bucket([]byte(BucketNetworksByName))
		if err := nameIdx.Put([]byte(name), []byte(network.ID)); err != nil {
			return fmt.Errorf("failed to save name index: %w", err)
		}

		return nil
	})

	return network, err
}

// GetNetworkByName retrieves a network by name
func (sm *StorageManager) GetNetworkByName(name string) (*VirtualNetwork, error) {
	var network *VirtualNetwork

	err := sm.db.View(func(tx *bbolt.Tx) error {
		// Get ID from name index
		nameIdx := tx.Bucket([]byte(BucketNetworksByName))
		id := nameIdx.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("network %q not found", name)
		}

		// Get network from primary bucket
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		data := networksBucket.Get(id)
		if data == nil {
			return fmt.Errorf("network data not found")
		}

		network = &VirtualNetwork{}
		if err := json.Unmarshal(data, network); err != nil {
			return fmt.Errorf("failed to unmarshal network: %w", err)
		}

		return nil
	})

	return network, err
}

// GetNetworkByID retrieves a network by ID
func (sm *StorageManager) GetNetworkByID(id string) (*VirtualNetwork, error) {
	var network *VirtualNetwork

	err := sm.db.View(func(tx *bbolt.Tx) error {
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		data := networksBucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("network %q not found", id)
		}

		network = &VirtualNetwork{}
		if err := json.Unmarshal(data, network); err != nil {
			return fmt.Errorf("failed to unmarshal network: %w", err)
		}

		return nil
	})

	return network, err
}

// ListNetworks lists all networks
func (sm *StorageManager) ListNetworks() ([]*VirtualNetwork, error) {
	var networks []*VirtualNetwork

	err := sm.db.View(func(tx *bbolt.Tx) error {
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		return networksBucket.ForEach(func(_, v []byte) error {
			network := &VirtualNetwork{}
			if err := json.Unmarshal(v, network); err != nil {
				return err
			}
			networks = append(networks, network)
			return nil
		})
	})

	return networks, err
}

// DeleteNetwork deletes a network and all its associated resources
func (sm *StorageManager) DeleteNetwork(name string) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		// Get network ID
		nameIdx := tx.Bucket([]byte(BucketNetworksByName))
		id := nameIdx.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("network %q not found", name)
		}
		idStr := string(id)

		// Delete server
		serversBucket := tx.Bucket([]byte(BucketServers))
		serversByName := tx.Bucket([]byte(BucketServersByName))
		if err := serversBucket.ForEach(func(k, v []byte) error {
			server := &Server{}
			if err := json.Unmarshal(v, server); err != nil {
				return err
			}
			if server.NetworkID == idStr {
				if err := serversBucket.Delete(k); err != nil {
					return err
				}
				// Find and delete from name index
				if err := serversByName.ForEach(func(k2, v2 []byte) error {
					if string(v2) == idStr {
						if err := serversByName.Delete(k2); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		// Delete nodes
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		if err := nodesBucket.ForEach(func(k, v []byte) error {
			node := &Node{}
			if err := json.Unmarshal(v, node); err != nil {
				return err
			}
			if node.NetworkID == idStr {
				if err := nodesBucket.Delete(k); err != nil {
					return err
				}
				// Find and delete from name index
				if err := nodesByName.ForEach(func(k2, v2 []byte) error {
					if string(v2) == idStr {
						if err := nodesByName.Delete(k2); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		// Delete configs
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		configsByVer := tx.Bucket([]byte(BucketConfigsByVer))
		if err := configsBucket.ForEach(func(k, v []byte) error {
			config := &ConfigVersion{}
			if err := json.Unmarshal(v, config); err != nil {
				return err
			}
			if config.NetworkID == idStr {
				if err := configsBucket.Delete(k); err != nil {
					return err
				}
				// Find and delete from version index
				if err := configsByVer.ForEach(func(k2, v2 []byte) error {
					if string(v2) == idStr {
						if err := configsByVer.Delete(k2); err != nil {
							return err
						}
					}
					return nil
				}); err != nil {
					return err
				}
			}
			return nil
		}); err != nil {
			return err
		}

		// Delete IP pool
		ipPoolsBucket := tx.Bucket([]byte(BucketIPPools))
		if err := ipPoolsBucket.Delete([]byte(idStr)); err != nil {
			return err
		}

		// Delete network
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if err := networksBucket.Delete([]byte(idStr)); err != nil {
			return err
		}
		if err := nameIdx.Delete([]byte(name)); err != nil {
			return err
		}

		return nil
	})
}

// ========== Server Operations ==========

// CreateServer creates a new server.
func (sm *StorageManager) CreateServer(networkID, name, publicAddress string, port int, virtualIP, privateKey, publicKey string) (*Server, error) {
	var server *Server

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Get network to verify it exists
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if networksBucket.Get([]byte(networkID)) == nil {
			return fmt.Errorf("network %q not found", networkID)
		}

		// Check if server already exists for this network
		serversBucket := tx.Bucket([]byte(BucketServers))
		found := false
		if err := serversBucket.ForEach(func(_, v []byte) error {
			s := &Server{}
			if err := json.Unmarshal(v, s); err != nil {
				return err
			}
			if s.NetworkID == networkID {
				found = true
			}
			return nil
		}); err != nil {
			return err
		}
		if found {
			return fmt.Errorf("server already exists for network %q", networkID)
		}

		// Check if name already exists in this network
		serversByName := tx.Bucket([]byte(BucketServersByName))
		if serversByName.Get([]byte(name)) != nil {
			return fmt.Errorf("server name %q already exists", name)
		}

		server = &Server{
			ID:            uuid.New().String(),
			NetworkID:     networkID,
			Name:          name,
			PublicAddress: publicAddress,
			Port:          port,
			VirtualIP:     virtualIP,
			PrivateKey:    privateKey,
			PublicKey:     publicKey,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("failed to marshal server: %w", err)
		}
		if err := serversBucket.Put([]byte(server.ID), data); err != nil {
			return fmt.Errorf("failed to save server: %w", err)
		}

		// Save to secondary bucket (name -> id)
		if err := serversByName.Put([]byte(name), []byte(server.ID)); err != nil {
			return fmt.Errorf("failed to save name index: %w", err)
		}

		return nil
	})

	return server, err
}

// GetServerByName retrieves a server by name
func (sm *StorageManager) GetServerByName(name string) (*Server, error) {
	var server *Server

	err := sm.db.View(func(tx *bbolt.Tx) error {
		serversByName := tx.Bucket([]byte(BucketServersByName))
		id := serversByName.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("server %q not found", name)
		}

		serversBucket := tx.Bucket([]byte(BucketServers))
		data := serversBucket.Get(id)
		if data == nil {
			return fmt.Errorf("server data not found")
		}

		server = &Server{}
		if err := json.Unmarshal(data, server); err != nil {
			return fmt.Errorf("failed to unmarshal server: %w", err)
		}

		return nil
	})

	return server, err
}

// GetServerByNetworkID retrieves the server for a network
func (sm *StorageManager) GetServerByNetworkID(networkID string) (*Server, error) {
	var server *Server

	err := sm.db.View(func(tx *bbolt.Tx) error {
		serversBucket := tx.Bucket([]byte(BucketServers))
		found := false
		var findErr error
		if err := serversBucket.ForEach(func(_, v []byte) error {
			s := &Server{}
			if err := json.Unmarshal(v, s); err != nil {
				findErr = err
				return err
			}
			if s.NetworkID == networkID {
				server = s
				found = true
				return nil
			}
			return nil
		}); err != nil {
			return err
		}
		if findErr != nil {
			return findErr
		}
		if !found {
			return fmt.Errorf("no server found for network %q", networkID)
		}
		return nil
	})

	return server, err
}

// UpdateServer updates server information.
func (sm *StorageManager) UpdateServer(id, publicAddress string, port int) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		serversBucket := tx.Bucket([]byte(BucketServers))
		data := serversBucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("server not found")
		}

		server := &Server{}
		if err := json.Unmarshal(data, server); err != nil {
			return err
		}

		server.PublicAddress = publicAddress
		server.Port = port
		server.UpdatedAt = time.Now()

		updated, err := json.Marshal(server)
		if err != nil {
			return fmt.Errorf("failed to marshal server: %w", err)
		}
		return serversBucket.Put([]byte(id), updated)
	})
}

// DeleteServer deletes a server
func (sm *StorageManager) DeleteServer(networkID string) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		serversBucket := tx.Bucket([]byte(BucketServers))
		serversByName := tx.Bucket([]byte(BucketServersByName))

		var serverID string
		var serverName string
		if err := serversBucket.ForEach(func(k, v []byte) error {
			server := &Server{}
			if err := json.Unmarshal(v, server); err != nil {
				return err
			}
			if server.NetworkID == networkID {
				serverID = string(k)
				serverName = server.Name
				return nil
			}
			return nil
		}); err != nil {
			return err
		}

		if serverID == "" {
			return fmt.Errorf("server not found for network")
		}

		if err := serversBucket.Delete([]byte(serverID)); err != nil {
			return err
		}
		if err := serversByName.Delete([]byte(serverName)); err != nil {
			return err
		}
		return nil
	})
}

// ========== Node Operations ==========

// CreateNode creates a new node.
func (sm *StorageManager) CreateNode(networkID, name, publicAddress string, port int, virtualIP string, nodeType NodeType, privateKey, publicKey string) (*Node, error) {
	var node *Node

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Get network to verify it exists
		networksBucket := tx.Bucket([]byte(BucketNetworks))
		if networksBucket.Get([]byte(networkID)) == nil {
			return fmt.Errorf("network %q not found", networkID)
		}

		// Check if name already exists in this network
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		if nodesByName.Get([]byte(name)) != nil {
			return fmt.Errorf("node name %q already exists", name)
		}

		node = &Node{
			ID:            uuid.New().String(),
			NetworkID:     networkID,
			Name:          name,
			PublicAddress: publicAddress,
			Port:          port,
			VirtualIP:     virtualIP,
			Type:          nodeType,
			PrivateKey:    privateKey,
			PublicKey:     publicKey,
			CreatedAt:     time.Now(),
			UpdatedAt:     time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("failed to marshal node: %w", err)
		}
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		if err := nodesBucket.Put([]byte(node.ID), data); err != nil {
			return fmt.Errorf("failed to save node: %w", err)
		}

		// Save to secondary bucket (name -> id)
		if err := nodesByName.Put([]byte(name), []byte(node.ID)); err != nil {
			return fmt.Errorf("failed to save name index: %w", err)
		}

		return nil
	})

	return node, err
}

// GetNodeByName retrieves a node by name
func (sm *StorageManager) GetNodeByName(name string) (*Node, error) {
	var node *Node

	err := sm.db.View(func(tx *bbolt.Tx) error {
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		id := nodesByName.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("node %q not found", name)
		}

		nodesBucket := tx.Bucket([]byte(BucketNodes))
		data := nodesBucket.Get(id)
		if data == nil {
			return fmt.Errorf("node data not found")
		}

		node = &Node{}
		if err := json.Unmarshal(data, node); err != nil {
			return fmt.Errorf("failed to unmarshal node: %w", err)
		}

		return nil
	})

	return node, err
}

// ListNodesByNetworkID lists all nodes in a network
func (sm *StorageManager) ListNodesByNetworkID(networkID string) ([]*Node, error) {
	var nodes []*Node

	err := sm.db.View(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		return nodesBucket.ForEach(func(k, v []byte) error {
			node := &Node{}
			if err := json.Unmarshal(v, node); err != nil {
				return err
			}
			if node.NetworkID == networkID {
				nodes = append(nodes, node)
			}
			return nil
		})
	})

	return nodes, err
}

// UpdateNode updates node information.
func (sm *StorageManager) UpdateNode(id, publicAddress string, port int, nodeType NodeType) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		nodesBucket := tx.Bucket([]byte(BucketNodes))
		data := nodesBucket.Get([]byte(id))
		if data == nil {
			return fmt.Errorf("node not found")
		}

		node := &Node{}
		if err := json.Unmarshal(data, node); err != nil {
			return err
		}

		node.PublicAddress = publicAddress
		node.Port = port
		node.Type = nodeType
		node.UpdatedAt = time.Now()

		updated, err := json.Marshal(node)
		if err != nil {
			return fmt.Errorf("failed to marshal node: %w", err)
		}
		return nodesBucket.Put([]byte(id), updated)
	})
}

// DeleteNode deletes a node
func (sm *StorageManager) DeleteNode(name string) error {
	return sm.db.Update(func(tx *bbolt.Tx) error {
		nodesByName := tx.Bucket([]byte(BucketNodesByName))
		id := nodesByName.Get([]byte(name))
		if id == nil {
			return fmt.Errorf("node %q not found", name)
		}

		nodesBucket := tx.Bucket([]byte(BucketNodes))
		if err := nodesBucket.Delete(id); err != nil {
			return err
		}
		if err := nodesByName.Delete([]byte(name)); err != nil {
			return err
		}
		return nil
	})
}

// ========== Config Operations ==========

// SaveConfigVersion saves a new config version.
func (sm *StorageManager) SaveConfigVersion(networkID, contentHash string, configs map[string]string) (*ConfigVersion, error) {
	var config *ConfigVersion

	err := sm.db.Update(func(tx *bbolt.Tx) error {
		// Get next version number by finding the maximum version for this network
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		nextVer := 1
		if err := configsBucket.ForEach(func(_, v []byte) error {
			c := &ConfigVersion{}
			if err := json.Unmarshal(v, c); err == nil {
				if c.NetworkID == networkID && c.Version >= nextVer {
					nextVer = c.Version + 1
				}
			}
			return nil
		}); err != nil {
			return err
		}

		config = &ConfigVersion{
			ID:          uuid.New().String(),
			NetworkID:   networkID,
			Version:     nextVer,
			ContentHash: contentHash,
			Configs:     configs,
			CreatedAt:   time.Now(),
		}

		// Save to primary bucket
		data, err := json.Marshal(config)
		if err != nil {
			return fmt.Errorf("failed to marshal config: %w", err)
		}
		if err := configsBucket.Put([]byte(config.ID), data); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		return nil
	})

	return config, err
}

// GetLatestConfigVersion retrieves the latest config version for a network
func (sm *StorageManager) GetLatestConfigVersion(networkID string) (*ConfigVersion, error) {
	var latestConfig *ConfigVersion

	err := sm.db.View(func(tx *bbolt.Tx) error {
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		if err := configsBucket.ForEach(func(_, v []byte) error {
			config := &ConfigVersion{}
			if err := json.Unmarshal(v, config); err != nil {
				return err
			}
			if config.NetworkID == networkID {
				if latestConfig == nil || config.Version > latestConfig.Version {
					latestConfig = config
				}
			}
			return nil
		}); err != nil {
			return err
		}
		if latestConfig == nil {
			return fmt.Errorf("no config version found for network %q", networkID)
		}
		return nil
	})

	return latestConfig, err
}

// GetConfigVersion retrieves a specific config version
func (sm *StorageManager) GetConfigVersion(networkID string, version int) (*ConfigVersion, error) {
	var config *ConfigVersion

	err := sm.db.View(func(tx *bbolt.Tx) error {
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		found := false
		if err := configsBucket.ForEach(func(_, v []byte) error {
			c := &ConfigVersion{}
			if err := json.Unmarshal(v, c); err != nil {
				return err
			}
			if c.NetworkID == networkID && c.Version == version {
				config = c
				found = true
				return nil
			}
			return nil
		}); err != nil {
			return err
		}
		if !found {
			return fmt.Errorf("config version %d not found for network %q", version, networkID)
		}
		return nil
	})

	return config, err
}

// ListConfigVersions lists all versions for a network
func (sm *StorageManager) ListConfigVersions(networkID string) ([]*ConfigVersion, error) {
	var versions []*ConfigVersion

	err := sm.db.View(func(tx *bbolt.Tx) error {
		configsBucket := tx.Bucket([]byte(BucketConfigs))
		return configsBucket.ForEach(func(_, v []byte) error {
			config := &ConfigVersion{}
			if err := json.Unmarshal(v, config); err != nil {
				return err
			}
			if config.NetworkID == networkID {
				versions = append(versions, config)
			}
			return nil
		})
	})

	if err != nil {
		return nil, err
	}

	// Sort by version in ascending order
	slices.SortFunc(versions, func(a, b *ConfigVersion) int {
		if a.Version < b.Version {
			return -1
		} else if a.Version > b.Version {
			return 1
		}
		return 0
	})

	return versions, err
}

// GetConfigHashByVersion retrieves the hash of a specific version
func (sm *StorageManager) GetConfigHashByVersion(networkID string, version int) (string, error) {
	config, err := sm.GetConfigVersion(networkID, version)
	if err != nil {
		return "", err
	}
	return config.ContentHash, nil
}
