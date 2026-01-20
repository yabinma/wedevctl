// Package util provides utility functions and validators for wedevctl.
package util

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net"
	"regexp"
	"strings"
)

// IPValidator validates network names and IP addresses
type IPValidator interface {
	IsValidNetworkName(name string) error
	IsValidCIDR(cidr string) error
	IsValidPublicAddress(addr string) error
}

// DefaultIPValidator provides default validation logic
type DefaultIPValidator struct{}

// IsValidNetworkName validates network names according to design:
// - Only alphanumeric characters
// - First character must be a letter
func (v *DefaultIPValidator) IsValidNetworkName(name string) error {
	if name == "" {
		return fmt.Errorf("network name cannot be empty")
	}
	if !regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9]*$`).MatchString(name) {
		return fmt.Errorf("network name must start with a letter and contain only alphanumeric characters")
	}
	return nil
}

// IsValidCIDR validates CIDR notation
func (v *DefaultIPValidator) IsValidCIDR(cidr string) error {
	_, ipnet, err := net.ParseCIDR(cidr)
	if err != nil {
		return fmt.Errorf("invalid CIDR notation: %w", err)
	}
	if ipnet == nil {
		return fmt.Errorf("invalid CIDR: network is nil")
	}
	return nil
}

// IsValidPublicAddress validates public address (domain or IP)
func (v *DefaultIPValidator) IsValidPublicAddress(addr string) error {
	if addr == "" {
		return fmt.Errorf("public address cannot be empty")
	}
	// Try to parse as IP first
	if ip := net.ParseIP(addr); ip != nil {
		return nil
	}
	// Otherwise, it should be a valid domain name
	// Basic validation: no spaces, contains at least one dot or is localhost
	if strings.Contains(addr, " ") {
		return fmt.Errorf("public address cannot contain spaces")
	}
	if !strings.Contains(addr, ".") && addr != "localhost" {
		return fmt.Errorf("public address must be a valid IP or domain name")
	}
	return nil
}

// NewDefaultIPValidator creates a new DefaultIPValidator
func NewDefaultIPValidator() *DefaultIPValidator {
	return &DefaultIPValidator{}
}

// IPPool manages IP allocation for a virtual network
type IPPool struct {
	networkCIDR string          // Network CIDR (e.g., "10.0.0.0/24")
	serverIP    string          // Reserved server IP (first usable)
	allocated   map[string]bool // Current allocated IPs: ip -> true
	recycled    []string        // Recycled IPs (for reuse)
	nextIndex   int             // Next index to allocate from
	firstUsable string
	lastUsable  string
	totalUsable int
}

// NewIPPool creates a new IP pool
func NewIPPool(networkCIDR string) (*IPPool, error) {
	// Parse CIDR
	_, ipnet, err := net.ParseCIDR(networkCIDR)
	if err != nil {
		return nil, fmt.Errorf("invalid CIDR: %w", err)
	}

	// Calculate usable IPs (all IPs except network address and broadcast)
	ones, bits := ipnet.Mask.Size()
	totalIPs := 1 << uint(bits-ones)

	// For IPv4 subnets with more than 2 IPs, first IP is network, last is broadcast
	// For /31 and /32, all are usable
	if totalIPs > 2 {
		// Get first usable IP (network + 1)
		firstIP := net.ParseIP(ipnet.IP.String())
		firstIP = increment(firstIP)
		firstUsable := firstIP.String()

		// Last usable IP (broadcast - 1)
		lastIP := net.ParseIP(ipnet.IP.String())
		for i := 0; i < totalIPs-1; i++ {
			lastIP = increment(lastIP)
		}
		lastUsable := lastIP.String()

		return &IPPool{
			networkCIDR: networkCIDR,
			serverIP:    firstUsable,
			allocated:   make(map[string]bool),
			recycled:    []string{},
			nextIndex:   1, // Start after server IP (index 0)
			firstUsable: firstUsable,
			lastUsable:  lastUsable,
			totalUsable: totalIPs - 2, // Exclude network and broadcast
		}, nil
	}

	return nil, fmt.Errorf("network must have at least 3 usable IPs")
}

// increment increments an IP address by 1
func increment(ip net.IP) net.IP {
	ip = ip.To4()
	if ip == nil {
		return nil
	}
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] > 0 {
			return ip
		}
	}
	return ip
}

// GetServerIP returns the reserved server IP (first usable IP)
func (p *IPPool) GetServerIP() string {
	return p.serverIP
}

// AllocateNodeIP allocates the next available IP for a node
// Returns the IP or an error if no IPs are available
func (p *IPPool) AllocateNodeIP() (string, error) {
	// Try to reuse recycled IP first
	if len(p.recycled) > 0 {
		ip := p.recycled[0]
		p.recycled = p.recycled[1:]
		p.allocated[ip] = true
		return ip, nil
	}

	// Allocate new IP if index doesn't exceed total
	if p.nextIndex >= p.totalUsable {
		return "", fmt.Errorf("no available IPs in pool (total usable: %d, allocated: %d)",
			p.totalUsable, len(p.allocated))
	}

	// Calculate the IP at nextIndex
	baseIP := net.ParseIP(p.firstUsable)
	for i := 0; i < p.nextIndex; i++ {
		baseIP = increment(baseIP)
	}
	p.nextIndex++

	ip := baseIP.String()
	p.allocated[ip] = true
	return ip, nil
}

// MarkIPAllocated marks an existing IP as allocated in the pool.
// This is used when reconstructing the pool from existing database records.
func (p *IPPool) MarkIPAllocated(ip string) error {
	if ip == "" {
		return fmt.Errorf("IP cannot be empty")
	}
	if p.allocated[ip] {
		return fmt.Errorf("IP %s is already allocated", ip)
	}
	p.allocated[ip] = true
	return nil
}

// SyncNextIndex updates nextIndex to point past all currently allocated IPs.
// This should be called after marking existing IPs as allocated when reconstructing the pool.
func (p *IPPool) SyncNextIndex() {
	maxIndex := 0

	for allocatedIP := range p.allocated {
		if allocatedIP == p.serverIP {
			continue // Server IP is at index 0, skip it
		}

		// Calculate the index of this IP
		currentIP := net.ParseIP(p.firstUsable)
		index := 0
		for currentIP.String() != allocatedIP && index < p.totalUsable {
			currentIP = increment(currentIP)
			index++
		}

		if index > maxIndex {
			maxIndex = index
		}
	}

	// Set nextIndex to one past the highest allocated index
	p.nextIndex = maxIndex + 1
}

// ReleaseNodeIP returns an IP to the pool for recycling
func (p *IPPool) ReleaseNodeIP(ip string) error {
	if ip == p.serverIP {
		return fmt.Errorf("cannot release server IP")
	}
	if !p.allocated[ip] {
		return fmt.Errorf("IP %s is not allocated", ip)
	}

	delete(p.allocated, ip)
	p.recycled = append(p.recycled, ip)
	return nil
}

// GetAllocatedIPs returns a copy of all allocated IPs
func (p *IPPool) GetAllocatedIPs() map[string]bool {
	result := make(map[string]bool)
	for ip := range p.allocated {
		result[ip] = true
	}
	return result
}

// GetState returns current state for persistence
func (p *IPPool) GetState() *IPPoolState {
	allocated := make([]string, 0, len(p.allocated))
	for ip := range p.allocated {
		if ip != p.serverIP { // Don't include server IP in the state
			allocated = append(allocated, ip)
		}
	}
	return &IPPoolState{
		NetworkCIDR: p.networkCIDR,
		ServerIP:    p.serverIP,
		Allocated:   allocated,
		Recycled:    p.recycled,
		NextIndex:   p.nextIndex,
	}
}

// IPPoolState represents the persistent state of an IP pool
type IPPoolState struct {
	NetworkCIDR string   `json:"network_cidr"`
	ServerIP    string   `json:"server_ip"`
	Allocated   []string `json:"allocated"`
	Recycled    []string `json:"recycled"`
	NextIndex   int      `json:"next_index"`
}

// RestoreIPPool creates an IP pool from saved state.
func RestoreIPPool(state *IPPoolState) (*IPPool, error) {
	pool, err := NewIPPool(state.NetworkCIDR)
	if err != nil {
		return nil, err
	}

	// Restore allocated IPs
	for _, ip := range state.Allocated {
		pool.allocated[ip] = true
	}

	// Restore recycled IPs
	pool.recycled = state.Recycled

	// Restore next index
	pool.nextIndex = state.NextIndex

	return pool, nil
}

// WireGuardKeyPair represents a WireGuard key pair
type WireGuardKeyPair struct {
	PrivateKey string
	PublicKey  string
}

// GenerateWireGuardKeys generates a WireGuard key pair
func GenerateWireGuardKeys() (*WireGuardKeyPair, error) {
	// Generate 32 random bytes for private key
	privateKeyBytes := make([]byte, 32)
	_, err := rand.Read(privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Encode to base64
	privateKey := base64.StdEncoding.EncodeToString(privateKeyBytes)

	// For public key, we'll use a simple hash-based approach
	// In real WireGuard, public key is derived from private using Curve25519
	// For this mock, we generate a different random key
	publicKeyBytes := make([]byte, 32)
	_, err = rand.Read(publicKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to generate public key: %w", err)
	}
	publicKey := base64.StdEncoding.EncodeToString(publicKeyBytes)

	return &WireGuardKeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// ValidateEndpoint validates endpoint format: address:port
func ValidateEndpoint(address string, port int) error {
	if address == "" {
		return fmt.Errorf("endpoint address cannot be empty")
	}
	if port < 1 || port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", port)
	}
	return nil
}

// FormatEndpoint formats endpoint as address:port
func FormatEndpoint(address string, port int) string {
	return fmt.Sprintf("%s:%d", address, port)
}
