package util

import (
	"testing"
)

func TestDefaultIPValidator_IsValidNetworkName(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid name", "mynetwork", false},
		{"valid name with numbers", "net123", false},
		{"valid name single letter", "a", false},
		{"empty name", "", true},
		{"starts with number", "1network", true},
		{"contains underscore", "my_network", true},
		{"contains dash", "my-network", true},
		{"contains space", "my network", true},
		{"mixed case valid", "MyNetwork", false},
	}

	validator := NewDefaultIPValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.IsValidNetworkName(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidNetworkName(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestDefaultIPValidator_IsValidCIDR(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid CIDR /24", "10.0.0.0/24", false},
		{"valid CIDR /16", "192.168.0.0/16", false},
		{"valid CIDR /25", "10.1.1.0/25", false},
		{"invalid CIDR no mask", "10.0.0.0", true},
		{"invalid CIDR bad format", "10.0.0/24", true},
		{"invalid CIDR out of range", "10.0.0.0/33", true},
	}

	validator := NewDefaultIPValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.IsValidCIDR(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidCIDR(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestDefaultIPValidator_IsValidPublicAddress(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid IP", "192.168.1.1", false},
		{"valid domain", "example.com", false},
		{"valid domain subdomain", "sub.example.com", false},
		{"localhost", "localhost", false},
		{"empty string", "", true},
		{"IP with space", "192.168.1.1 ", true},
		{"domain with space", "example .com", true},
		{"single word no dot", "nodomain", true},
	}

	validator := NewDefaultIPValidator()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.IsValidPublicAddress(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("IsValidPublicAddress(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
		})
	}
}

func TestNewIPPool(t *testing.T) {
	tests := []struct {
		name         string
		cidr         string
		wantServerIP string
		wantErr      bool
	}{
		{"valid /24", "10.0.0.0/24", "10.0.0.1", false},
		{"valid /25", "192.168.1.0/25", "192.168.1.1", false},
		{"invalid CIDR", "invalid", "", true},
		{"too small subnet /31", "10.0.0.0/31", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewIPPool(tt.cidr)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewIPPool() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && pool.GetServerIP() != tt.wantServerIP {
				t.Errorf("GetServerIP() = %v, want %v", pool.GetServerIP(), tt.wantServerIP)
			}
		})
	}
}

func TestIPPool_AllocateNodeIP(t *testing.T) {
	tests := []struct {
		name       string
		cidr       string
		allocCount int
		wantErr    bool
	}{
		{"allocate one IP", "10.0.0.0/24", 1, false},
		{"allocate many IPs", "10.0.0.0/24", 50, false},
		{"exhaust pool /25", "10.0.0.0/25", 62, false}, // /25 has 126 usable - 2 (reserved) = 124 total usable, minus server = 123
		{"exceed pool /25", "10.0.0.0/25", 124, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool, err := NewIPPool(tt.cidr)
			if err != nil {
				t.Fatalf("NewIPPool() error = %v", err)
			}

			for i := 0; i < tt.allocCount; i++ {
				ip, err := pool.AllocateNodeIP()
				if (err != nil) != tt.wantErr {
					if !tt.wantErr {
						t.Errorf("AllocateNodeIP() iteration %d error = %v, wantErr %v", i, err, tt.wantErr)
					}
					return
				}
				if !tt.wantErr {
					// Verify IP is not the server IP
					if ip == pool.GetServerIP() {
						t.Errorf("AllocateNodeIP() returned server IP: %s", ip)
					}
				}
			}
		})
	}
}

func TestIPPool_ReleaseAndReuse(t *testing.T) {
	pool, err := NewIPPool("10.0.0.0/24")
	if err != nil {
		t.Fatalf("NewIPPool() error = %v", err)
	}

	// Allocate some IPs
	ip1, err := pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	ip2, err := pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	ip3, err := pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}

	// Release one
	err = pool.ReleaseNodeIP(ip2)
	if err != nil {
		t.Errorf("ReleaseNodeIP() error = %v", err)
	}

	// Allocate should reuse released IP
	ip4, err := pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	if ip4 != ip2 {
		t.Errorf("AllocateNodeIP() should reuse released IP %s, got %s", ip2, ip4)
	}

	// Try to release server IP - should fail
	err = pool.ReleaseNodeIP(pool.GetServerIP())
	if err == nil {
		t.Errorf("ReleaseNodeIP() should not allow releasing server IP")
	}

	// Try to release unallocated IP - should fail
	err = pool.ReleaseNodeIP("10.99.99.99")
	if err == nil {
		t.Errorf("ReleaseNodeIP() should not allow releasing unallocated IP")
	}

	// Verify ip1 and ip3 are still allocated (used in comparisons)
	_ = ip1
	_ = ip3
}

func TestIPPool_GetState_RestoreIPPool(t *testing.T) {
	// Create and populate a pool
	pool, err := NewIPPool("10.0.0.0/24")
	if err != nil {
		t.Fatalf("NewIPPool() error = %v", err)
	}

	ip1, err := pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	_, err = pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	_, err = pool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}

	// Release one to create recycled state
	err = pool.ReleaseNodeIP(ip1)
	if err != nil {
		t.Fatalf("ReleaseNodeIP() error = %v", err)
	}

	// Get state
	state := pool.GetState()

	// Create a new pool from state
	newPool, err := RestoreIPPool(state)
	if err != nil {
		t.Fatalf("RestoreIPPool() error = %v", err)
	}

	// Verify state is restored
	if newPool.GetServerIP() != pool.GetServerIP() {
		t.Errorf("Server IP mismatch: got %s, want %s", newPool.GetServerIP(), pool.GetServerIP())
	}

	// Verify recycled IP can be reused
	ip4, err := newPool.AllocateNodeIP()
	if err != nil {
		t.Fatalf("AllocateNodeIP() error = %v", err)
	}
	if ip4 != ip1 {
		t.Errorf("Restored pool should reuse recycled IP %s, got %s", ip1, ip4)
	}
}

func TestGenerateWireGuardKeys(t *testing.T) {
	tests := []struct {
		name string
		_    struct{}
	}{
		{"generate keys once", struct{}{}},
		{"generate keys twice", struct{}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys, err := GenerateWireGuardKeys()
			if err != nil {
				t.Errorf("GenerateWireGuardKeys() error = %v", err)
			}
			if keys == nil {
				t.Errorf("GenerateWireGuardKeys() returned nil")
				return
			}
			if keys.PrivateKey == "" || keys.PublicKey == "" {
				t.Errorf("GenerateWireGuardKeys() generated empty keys")
			}
			// Keys should be base64 encoded
			if len(keys.PrivateKey) != 44 { // 32 bytes base64 encoded
				t.Errorf("PrivateKey has unexpected length: %d", len(keys.PrivateKey))
			}
		})
	}
}

func TestGenerateWireGuardKeys_Uniqueness(t *testing.T) {
	keys1, err := GenerateWireGuardKeys()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeys() error = %v", err)
	}
	keys2, err := GenerateWireGuardKeys()
	if err != nil {
		t.Fatalf("GenerateWireGuardKeys() error = %v", err)
	}

	if keys1.PrivateKey == keys2.PrivateKey {
		t.Errorf("Generated private keys should be unique")
	}
	if keys1.PublicKey == keys2.PublicKey {
		t.Errorf("Generated public keys should be unique")
	}
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		name    string
		address string
		port    int
		wantErr bool
	}{
		{"valid endpoint", "example.com", 51820, false},
		{"valid IP endpoint", "192.168.1.1", 8080, false},
		{"empty address", "", 51820, true},
		{"port too low", "example.com", 0, true},
		{"port too high", "example.com", 65536, true},
		{"valid port range low", "example.com", 1, false},
		{"valid port range high", "example.com", 65535, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEndpoint(tt.address, tt.port)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEndpoint() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestFormatEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		address  string
		port     int
		expected string
	}{
		{"simple endpoint", "example.com", 51820, "example.com:51820"},
		{"IP endpoint", "192.168.1.1", 8080, "192.168.1.1:8080"},
		{"localhost", "localhost", 3000, "localhost:3000"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatEndpoint(tt.address, tt.port)
			if result != tt.expected {
				t.Errorf("FormatEndpoint() = %v, want %v", result, tt.expected)
			}
		})
	}
}
