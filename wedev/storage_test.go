package wedev

import (
	"fmt"
	"path/filepath"
	"testing"

	"go.etcd.io/bbolt"
)

func TestCreateNetwork(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net1, err := sm.CreateNetwork("network1", "10.0.0.0/24")
	if err != nil {
		t.Errorf("CreateNetwork() error = %v", err)
		return
	}
	if net1.Name != "network1" || net1.CIDR != "10.0.0.0/24" {
		t.Errorf("CreateNetwork() returned unexpected values")
	}

	// Try to create duplicate - should fail
	_, err = sm.CreateNetwork("network1", "10.1.0.0/24")
	if err == nil {
		t.Errorf("CreateNetwork() should fail with duplicate name")
	}
}

func TestGetNetworkByName(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create a network
	expected, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	// Retrieve it
	got, err := sm.GetNetworkByName("testnet")
	if err != nil {
		t.Errorf("GetNetworkByName() error = %v", err)
		return
	}
	if got.ID != expected.ID {
		t.Errorf("GetNetworkByName() returned wrong network")
	}

	// Try to get non-existent network
	_, err = sm.GetNetworkByName("nonexistent")
	if err == nil {
		t.Errorf("GetNetworkByName() should return error for non-existent network")
	}
}

func TestListNetworks(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create multiple networks
	sm.CreateNetwork("net1", "10.0.0.0/24")
	sm.CreateNetwork("net2", "10.1.0.0/24")
	sm.CreateNetwork("net3", "10.2.0.0/24")

	// List them
	networks, err := sm.ListNetworks()
	if err != nil {
		t.Errorf("ListNetworks() error = %v", err)
		return
	}
	if len(networks) != 3 {
		t.Errorf("ListNetworks() returned %d networks, want 3", len(networks))
	}
}

func TestDeleteNetwork_CascadeDelete(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	// Create network with server and node
	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk1", "pub1")
	sm.CreateNode(net.ID, "node1", "192.168.1.2", 51821, "10.0.0.2", NodeTypePeer, "pk2", "pub2")

	// Delete network
	err = sm.DeleteNetwork("testnet")
	if err != nil {
		t.Errorf("DeleteNetwork() error = %v", err)
		return
	}

	// Verify network is gone
	_, err = sm.GetNetworkByName("testnet")
	if err == nil {
		t.Errorf("DeleteNetwork() should remove the network")
	}

	// Verify server is gone
	_, err = sm.GetServerByName("server1")
	if err == nil {
		t.Errorf("DeleteNetwork() should cascade delete server")
	}

	// Verify node is gone
	_, err = sm.GetNodeByName("node1")
	if err == nil {
		t.Errorf("DeleteNetwork() should cascade delete node")
	}
}

func TestCreateServer(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	tests := []struct {
		name    string
		srvName string
		wantErr bool
	}{
		{"create valid server", "server1", false},
		{"create duplicate server", "server1", true},
		{"create second server same network", "server2", true},
	}

	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.CreateServer(net.ID, tt.srvName, "192.168.1.1", 51820, "10.0.0.1", "pk", "pub")
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateServer() iteration %d error = %v, wantErr %v", i, err, tt.wantErr)
			}
		})
	}
}

func TestGetServerByNetworkID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	expected, _ := sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk", "pub")

	got, err := sm.GetServerByNetworkID(net.ID)
	if err != nil {
		t.Errorf("GetServerByNetworkID() error = %v", err)
		return
	}
	if got.ID != expected.ID {
		t.Errorf("GetServerByNetworkID() returned wrong server")
	}

	// Try to get server for network with no server
	net2, _ := sm.CreateNetwork("testnet2", "10.1.0.0/24")
	_, err = sm.GetServerByNetworkID(net2.ID)
	if err == nil {
		t.Errorf("GetServerByNetworkID() should return error when no server exists")
	}
}

func TestCreateNode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	tests := []struct {
		name     string
		nodeName string
		nodeType NodeType
		wantErr  bool
	}{
		{"create peer node", "node1", NodeTypePeer, false},
		{"create route node", "node2", NodeTypeRoute, false},
		{"duplicate name", "node1", NodeTypePeer, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sm.CreateNode(net.ID, tt.nodeName, "192.168.1.1", 51821, "10.0.0.2", tt.nodeType, "pk", "pub")
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateNode() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListNodesByNetworkID(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net1, _ := sm.CreateNetwork("net1", "10.0.0.0/24")
	net2, _ := sm.CreateNetwork("net2", "10.1.0.0/24")

	sm.CreateNode(net1.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk1", "pub1")
	sm.CreateNode(net1.ID, "node2", "192.168.1.2", 51822, "10.0.0.3", NodeTypePeer, "pk2", "pub2")
	sm.CreateNode(net2.ID, "node3", "192.168.1.3", 51823, "10.1.0.2", NodeTypePeer, "pk3", "pub3")

	nodes, err := sm.ListNodesByNetworkID(net1.ID)
	if err != nil {
		t.Errorf("ListNodesByNetworkID() error = %v", err)
		return
	}
	if len(nodes) != 2 {
		t.Errorf("ListNodesByNetworkID() returned %d nodes, want 2", len(nodes))
	}
}

func TestDeleteNode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	sm.CreateNode(net.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk", "pub")

	err = sm.DeleteNode("node1")
	if err != nil {
		t.Errorf("DeleteNode() error = %v", err)
		return
	}

	_, err = sm.GetNodeByName("node1")
	if err == nil {
		t.Errorf("DeleteNode() should remove the node")
	}
}

func TestSaveConfigVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")

	configs := map[string]string{
		"server.conf": "[Interface]\nPrivateKey = test",
		"node1.conf":  "[Interface]\nPrivateKey = test2",
	}

	config, err := sm.SaveConfigVersion(net.ID, "hash1", configs)
	if err != nil {
		t.Errorf("SaveConfigVersion() error = %v", err)
		return
	}

	if config.Version != 1 {
		t.Errorf("SaveConfigVersion() version = %v, want 1", config.Version)
	}

	// Save another version
	config2, _ := sm.SaveConfigVersion(net.ID, "hash2", configs)
	if config2.Version != 2 {
		t.Errorf("SaveConfigVersion() second version = %v, want 2", config2.Version)
	}
}

func TestGetLatestConfigVersion(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	configs := map[string]string{"server.conf": "test"}

	sm.SaveConfigVersion(net.ID, "hash1", configs)
	sm.SaveConfigVersion(net.ID, "hash2", configs)
	sm.SaveConfigVersion(net.ID, "hash3", configs)

	latest, err := sm.GetLatestConfigVersion(net.ID)
	if err != nil {
		t.Errorf("GetLatestConfigVersion() error = %v", err)
		return
	}

	if latest.Version != 3 {
		t.Errorf("GetLatestConfigVersion() version = %v, want 3", latest.Version)
	}
}

func TestListConfigVersions(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	configs := map[string]string{"server.conf": "test"}

	sm.SaveConfigVersion(net.ID, "hash1", configs)
	sm.SaveConfigVersion(net.ID, "hash2", configs)

	versions, err := sm.ListConfigVersions(net.ID)
	if err != nil {
		t.Errorf("ListConfigVersions() error = %v", err)
		return
	}

	if len(versions) != 2 {
		t.Errorf("ListConfigVersions() returned %d versions, want 2", len(versions))
	}
}

func TestTransactionConsistency(t *testing.T) {
	// Test that BoltDB transactions maintain consistency between primary and secondary buckets
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	db, err := bbolt.Open(dbPath, 0600, nil)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize buckets
	db.Update(func(tx *bbolt.Tx) error {
		tx.CreateBucketIfNotExists([]byte("items"))
		tx.CreateBucketIfNotExists([]byte("items_by_name"))
		return nil
	})

	// Test successful transaction
	err = db.Update(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		itemsByName := tx.Bucket([]byte("items_by_name"))

		data := []byte(`{"id":"1","name":"test"}`)
		items.Put([]byte("1"), data)
		itemsByName.Put([]byte("test"), []byte("1"))
		return nil
	})

	if err != nil {
		t.Errorf("Transaction failed: %v", err)
	}

	// Verify both buckets have data
	db.View(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		itemsByName := tx.Bucket([]byte("items_by_name"))

		if items.Get([]byte("1")) == nil {
			t.Errorf("Primary bucket missing data after transaction")
		}
		if itemsByName.Get([]byte("test")) == nil {
			t.Errorf("Secondary bucket missing data after transaction")
		}

		return nil
	})

	// Test failed transaction (return error)
	err = db.Update(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		itemsByName := tx.Bucket([]byte("items_by_name"))

		data := []byte(`{"id":"2","name":"test2"}`)
		items.Put([]byte("2"), data)
		itemsByName.Put([]byte("test2"), []byte("2"))
		return fmt.Errorf("simulated error")
	})

	if err == nil {
		t.Errorf("Transaction should have failed")
	}

	// Verify failed transaction didn't write anything
	db.View(func(tx *bbolt.Tx) error {
		items := tx.Bucket([]byte("items"))
		if items.Get([]byte("2")) != nil {
			t.Errorf("Failed transaction should not persist primary bucket changes")
		}
		return nil
	})
}

func TestUpdateServer(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	server, _ := sm.CreateServer(net.ID, "server1", "192.168.1.1", 51820, "10.0.0.1", "pk", "pub")

	err = sm.UpdateServer(server.ID, "192.168.1.100", 51821)
	if err != nil {
		t.Errorf("UpdateServer() error = %v", err)
		return
	}

	updated, _ := sm.GetServerByName("server1")
	if updated.PublicAddress != "192.168.1.100" || updated.Port != 51821 {
		t.Errorf("UpdateServer() did not update values correctly")
	}
}

func TestUpdateNode(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	sm, err := NewStorageManager(dbPath)
	if err != nil {
		t.Fatalf("NewStorageManager() error = %v", err)
	}
	defer sm.Close()

	net, _ := sm.CreateNetwork("testnet", "10.0.0.0/24")
	node, _ := sm.CreateNode(net.ID, "node1", "192.168.1.1", 51821, "10.0.0.2", NodeTypePeer, "pk", "pub")

	err = sm.UpdateNode(node.ID, "192.168.1.100", 51822, NodeTypeRoute)
	if err != nil {
		t.Errorf("UpdateNode() error = %v", err)
		return
	}

	updated, _ := sm.GetNodeByName("node1")
	if updated.PublicAddress != "192.168.1.100" || updated.Port != 51822 || updated.Type != NodeTypeRoute {
		t.Errorf("UpdateNode() did not update values correctly")
	}
}
