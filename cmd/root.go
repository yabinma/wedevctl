// Package cmd provides CLI command definitions for wedevctl.
package cmd

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/wedevctl/util"
	"github.com/wedevctl/wedev"
)

var dbPath string
var storage *wedev.StorageManager
var vnManager *wedev.VirtualNetworkManager
var validator util.IPValidator

// NewRootCommand creates the root CLI command
func NewRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "wedevctl",
		Short: "WeDev resource management CLI tool",
		Long:  "wedevctl is a CLI tool for managing WeDev virtual networks and WireGuard configurations",
		PersistentPreRunE: func(_cmd *cobra.Command, _args []string) error {
			// Initialize database
			// Check environment variable first
			dbDir := os.Getenv("WEDEVCTL_DB_PATH")

			// If not set, use default ~/.wedevctl
			if dbDir == "" {
				homeDir, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("failed to get home directory: %w", err)
				}
				dbDir = filepath.Join(homeDir, ".wedevctl")
			}

			// Expand relative paths to absolute
			if !filepath.IsAbs(dbDir) {
				absDir, err := filepath.Abs(dbDir)
				if err != nil {
					return fmt.Errorf("failed to resolve db path: %w", err)
				}
				dbDir = absDir
			}

			// Create directory with secure permissions
			if err := os.MkdirAll(dbDir, 0o700); err != nil {
				return fmt.Errorf("failed to create db directory: %w", err)
			}

			dbPath = filepath.Join(dbDir, "wedevctl.db")

			var sErr error
			storage, sErr = wedev.NewStorageManager(dbPath)
			if sErr != nil {
				return fmt.Errorf("failed to initialize storage: %w", sErr)
			}

			validator = util.NewDefaultIPValidator()
			var mnErr error
			vnManager, mnErr = wedev.NewVirtualNetworkManager(storage, validator)
			if mnErr != nil {
				return fmt.Errorf("failed to initialize virtual network manager: %w", mnErr)
			}
			return nil
		},
		PersistentPostRunE: func(_cmd *cobra.Command, _args []string) error {
			if storage != nil {
				return storage.Close()
			}
			return nil
		},
	}

	// Add subcommands
	root.AddCommand(NewVirtualNetworkCommand())

	return root
}

// NewVirtualNetworkCommand creates the 'vn' command group
func NewVirtualNetworkCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vn [network-name]",
		Short: "Manage virtual networks",
		Long: `Create, list, and manage virtual networks.

Without arguments: shows available commands
With network-name: manage specific network resources

Examples:
  wedevctl vn add prod-net 10.0.0.0/24
  wedevctl vn list
  wedevctl vn prod-net server add server1 example.com
  wedevctl vn prod-net node add node1 192.168.1.1 51821 peer
  wedevctl vn prod-net config generate`,
		// Disable flag parsing for this command since we handle routing manually
		DisableFlagParsing: true,
		RunE: func(c *cobra.Command, args []string) error {
			// If no args, show help
			if len(args) == 0 {
				return c.Help()
			}

			networkName := args[0]

			// Check if this is a direct subcommand (add, list, delete)
			switch networkName {
			case "add", "list", "delete":
				// Re-enable normal command processing for these
				for _, cmd := range c.Commands() {
					if cmd.Name() == networkName {
						cmd.SetArgs(args[1:])
						return cmd.Execute()
					}
				}
				return c.Help()
			}

			// Validate network exists
			_, err := storage.GetNetworkByName(networkName)
			if err != nil {
				return fmt.Errorf("network '%s' not found. Use 'wedevctl vn list' to see available networks", networkName)
			}

			// Create dynamic subcommand for this network
			networkCmd := &cobra.Command{
				Use:   networkName,
				Short: fmt.Sprintf("Manage network '%s'", networkName),
				Long:  fmt.Sprintf("Manage servers, nodes, and configurations for virtual network '%s'", networkName),
			}

			// Disable default completion command on network-scoped commands
			networkCmd.CompletionOptions.DisableDefaultCmd = true

			// Add server/node/config subcommands with network context
			networkCmd.AddCommand(makeServerCommand(networkName))
			networkCmd.AddCommand(makeNodeCommand(networkName))
			networkCmd.AddCommand(makeConfigCommand(networkName))

			// Execute with remaining args
			if len(args) > 1 {
				networkCmd.SetArgs(args[1:])
			}
			return networkCmd.Execute()
		},
	}

	cmd.AddCommand(NewVNAddCommand())
	cmd.AddCommand(NewVNListCommand())
	cmd.AddCommand(NewVNDeleteCommand())

	return cmd
}

// ========== Virtual Network Commands ==========

// NewVNAddCommand creates the 'vn add' command
func NewVNAddCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "add <network-name> <network-cidr>",
		Short: "Create a new virtual network",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			cidr := args[1]

			// Ask for confirmation
			if !confirmAction(fmt.Sprintf("Create virtual network '%s' with CIDR %s?", name, cidr)) {
				fmt.Println("Cancelled")
				return nil
			}

			net, err := vnManager.CreateVirtualNetwork(name, cidr)
			if err != nil {
				return fmt.Errorf("failed to create network: %w", err)
			}

			fmt.Printf("Virtual network '%s' created successfully (ID: %s)\n", net.Name, net.ID)
			return nil
		},
	}
}

// NewVNListCommand creates the 'vn list' command
func NewVNListCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all virtual networks",
		RunE: func(_cmd *cobra.Command, _args []string) error {
			networks, err := vnManager.ListVirtualNetworks()
			if err != nil {
				return fmt.Errorf("failed to list networks: %w", err)
			}

			if len(networks) == 0 {
				fmt.Println("No virtual networks found")
				return nil
			}

			fmt.Printf("%-20s %-20s\n", "Name", "CIDR")
			fmt.Println("----------------------------------------------")
			for _, net := range networks {
				fmt.Printf("%-20s %-20s\n", net.Name, net.CIDR)
			}

			return nil
		},
	}
}

// NewVNDeleteCommand creates the 'vn delete' command
func NewVNDeleteCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <network-name>",
		Short: "Delete a virtual network",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]

			// Warn about cascade deletion
			if !confirmAction(fmt.Sprintf("Delete network '%s'? This will also delete the server, all nodes, and all configuration history.", name)) {
				fmt.Println("Cancelled")
				return nil
			}

			err := vnManager.DeleteVirtualNetwork(name)
			if err != nil {
				return fmt.Errorf("failed to delete network: %w", err)
			}

			fmt.Printf("Virtual network '%s' deleted successfully\n", name)
			return nil
		},
	}
}

// ========== Server Commands ==========

// makeServerCommand creates the 'server' command group for a specific network
func makeServerCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "server",
		Short: "Manage servers",
		Long:  fmt.Sprintf("Manage servers in virtual network '%s'", networkName),
	}

	cmd.AddCommand(makeServerAddCommand(networkName))
	cmd.AddCommand(makeServerInfoCommand(networkName))
	cmd.AddCommand(makeServerEditCommand(networkName))
	cmd.AddCommand(makeServerDeleteCommand(networkName))

	return cmd
}

// makeServerAddCommand creates the 'server add' command for a specific network
func makeServerAddCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <server-name> <public-address> [port]",
		Short: "Create a new server",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			serverName := args[0]
			publicAddress := args[1]

			port := 51820
			if len(args) == 3 {
				_, err := fmt.Sscanf(args[2], "%d", &port)
				if err != nil {
					return fmt.Errorf("invalid port number: %w", err)
				}
			}

			server, err := vnManager.CreateServer(networkName, serverName, publicAddress, port)
			if err != nil {
				return fmt.Errorf("failed to create server: %w", err)
			}

			fmt.Printf("Server '%s' created successfully\n", server.Name)
			fmt.Printf("Virtual IP: %s\n", server.VirtualIP)
			fmt.Printf("Public Address: %s:%d\n", server.PublicAddress, server.Port)

			return nil
		},
	}

	return cmd
}

// makeServerInfoCommand creates the 'server info' command for a specific network
func makeServerInfoCommand(networkName string) *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Show server information",
		Args:  cobra.NoArgs,
		RunE: func(_cmd *cobra.Command, _args []string) error {
			server, err := vnManager.GetServer(networkName)
			if err != nil {
				return fmt.Errorf("failed to get server: %w", err)
			}

			fmt.Printf("Server: %s\n", server.Name)
			fmt.Printf("Virtual IP: %s\n", server.VirtualIP)
			fmt.Printf("Public Address: %s:%d\n", server.PublicAddress, server.Port)
			fmt.Printf("ID: %s\n", server.ID)

			return nil
		},
	}
}

// makeServerEditCommand creates the 'server edit' command for a specific network
func makeServerEditCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit --public-address <addr> --port <port>",
		Short: "Edit server information",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			publicAddress, err := cmd.Flags().GetString("public-address")
			if err != nil {
				return fmt.Errorf("failed to get public-address flag: %w", err)
			}
			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return fmt.Errorf("failed to get port flag: %w", err)
			}

			if publicAddress == "" && port == 0 {
				return fmt.Errorf("must specify at least --public-address or --port")
			}

			server, err := vnManager.GetServer(networkName)
			if err != nil {
				return fmt.Errorf("failed to get server: %w", err)
			}

			// Use current values if not specified
			if publicAddress == "" {
				publicAddress = server.PublicAddress
			}
			if port == 0 {
				port = server.Port
			}

			updated, err := vnManager.UpdateServer(networkName, publicAddress, port)
			if err != nil {
				return fmt.Errorf("failed to update server: %w", err)
			}

			fmt.Printf("Server '%s' updated successfully\n", updated.Name)
			fmt.Printf("Public Address: %s:%d\n", updated.PublicAddress, updated.Port)

			return nil
		},
	}

	cmd.Flags().String("public-address", "", "Public address or domain")
	cmd.Flags().Int("port", 0, "Port number")

	return cmd
}

// makeServerDeleteCommand creates the 'server delete' command for a specific network
func makeServerDeleteCommand(networkName string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete",
		Short: "Delete the server",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {

			if !confirmAction(fmt.Sprintf("Delete server in network '%s'?", networkName)) {
				fmt.Println("Cancelled")
				return nil
			}

			err := vnManager.DeleteServer(networkName)
			if err != nil {
				return fmt.Errorf("failed to delete server: %w", err)
			}

			fmt.Println("Server deleted successfully")
			return nil
		},
	}
}

// ========== Node Commands ==========

// makeNodeCommand creates the 'node' command group for a specific network
func makeNodeCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "node",
		Short: "Manage nodes",
		Long:  fmt.Sprintf("Manage nodes in virtual network '%s'", networkName),
	}

	cmd.AddCommand(makeNodeAddCommand(networkName))
	cmd.AddCommand(makeNodeListCommand(networkName))
	cmd.AddCommand(makeNodeEditCommand(networkName))
	cmd.AddCommand(makeNodeDeleteCommand(networkName))

	return cmd
}

// makeNodeAddCommand creates the 'node add' command for a specific network
func makeNodeAddCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <node-name> <type> [public-address] [port]",
		Short: "Create a new node",
		Long: `Create a new node in the virtual network.

Type can be 'peer' or 'route':
  - peer: requires public-address, participates in peer-to-peer connections
  - route: public-address is optional, only connects to server

Examples:
  # Peer node (public-address required)
  wedevctl vn mynet node add node1 peer 192.168.1.100
  wedevctl vn mynet node add node1 peer 192.168.1.100 51821

  # Route node (public-address optional)
  wedevctl vn mynet node add node2 route
  wedevctl vn mynet node add node2 route 192.168.1.200 51822`,
		Args: cobra.RangeArgs(2, 4),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]
			nodeTypeStr := args[1]

			// Validate and parse node type
			var nodeType wedev.NodeType
			if nodeTypeStr == "route" {
				nodeType = wedev.NodeTypeRoute
			} else if nodeTypeStr == "peer" {
				nodeType = wedev.NodeTypePeer
			} else {
				return fmt.Errorf("invalid node type: %s (must be 'peer' or 'route')", nodeTypeStr)
			}

			// Parse public address
			publicAddress := ""
			if len(args) >= 3 {
				publicAddress = args[2]
			}

			// Validate: peer type requires public address
			if nodeType == wedev.NodeTypePeer && publicAddress == "" {
				return fmt.Errorf("peer type nodes require a public address")
			}

			// Parse port (default 51820)
			port := 51820
			if len(args) >= 4 {
				_, err := fmt.Sscanf(args[3], "%d", &port)
				if err != nil {
					return fmt.Errorf("invalid port number: %w", err)
				}
			}

			node, err := vnManager.CreateNode(networkName, nodeName, publicAddress, port, nodeType)
			if err != nil {
				return fmt.Errorf("failed to create node: %w", err)
			}

			fmt.Printf("Node '%s' created successfully\n", node.Name)
			fmt.Printf("Virtual IP: %s\n", node.VirtualIP)
			fmt.Printf("Type: %s\n", node.Type)
			if publicAddress != "" {
				fmt.Printf("Public Address: %s:%d\n", node.PublicAddress, node.Port)
			}

			return nil
		},
	}

	return cmd
}

// makeNodeListCommand creates the 'node list' command for a specific network
func makeNodeListCommand(networkName string) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all nodes",
		Args:  cobra.NoArgs,
		RunE: func(_cmd *cobra.Command, _args []string) error {

			nodes, err := vnManager.ListNodes(networkName)
			if err != nil {
				return fmt.Errorf("failed to list nodes: %w", err)
			}

			if len(nodes) == 0 {
				fmt.Println("No nodes found")
				return nil
			}

			fmt.Printf("%-15s %-15s %-20s %-10s\n", "Name", "Virtual IP", "Public Address", "Type")
			fmt.Println("--------------------------------------------------------------")
			for _, node := range nodes {
				endpoint := fmt.Sprintf("%s:%d", node.PublicAddress, node.Port)
				fmt.Printf("%-15s %-15s %-20s %-10s\n", node.Name, node.VirtualIP, endpoint, node.Type)
			}

			return nil
		},
	}
}

// makeNodeEditCommand creates the 'node edit' command for a specific network.
func makeNodeEditCommand(_ string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "edit <node-name> [--type <type>] [--public-address <addr>] [--port <port>]",
		Short: "Edit node information",
		Long: `Edit node information including type, public address, and port.

Validation rules:
  - When changing type to 'peer': public-address is required
  - When changing type to 'route': public-address is optional
  - Peer type nodes must always have a public-address

Examples:
  # Change node type to route (can clear public address)
  wedevctl vn mynet node edit node1 --type route --public-address ""

  # Change node type to peer (must provide public address)
  wedevctl vn mynet node edit node1 --type peer --public-address 192.168.1.100

  # Update only port
  wedevctl vn mynet node edit node1 --port 51821`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]

			publicAddress, err := cmd.Flags().GetString("public-address")
			if err != nil {
				return fmt.Errorf("failed to get public-address flag: %w", err)
			}
			port, err := cmd.Flags().GetInt("port")
			if err != nil {
				return fmt.Errorf("failed to get port flag: %w", err)
			}
			nodeTypeStr, err := cmd.Flags().GetString("type")
			if err != nil {
				return fmt.Errorf("failed to get type flag: %w", err)
			}

			node, err := vnManager.GetNode(nodeName)
			if err != nil {
				return fmt.Errorf("failed to get node: %w", err)
			}

			// Determine the final node type
			nodeType := node.Type
			typeChanged := false
			if nodeTypeStr != "" {
				typeChanged = true
				switch nodeTypeStr {
				case "route":
					nodeType = wedev.NodeTypeRoute
				case "peer":
					nodeType = wedev.NodeTypePeer
				default:
					return fmt.Errorf("invalid node type: %s (must be 'peer' or 'route')", nodeTypeStr)
				}
			}

			// Determine the final public address
			publicAddressProvided := cmd.Flags().Changed("public-address")
			if !publicAddressProvided {
				publicAddress = node.PublicAddress
			}

			// Validate type and public address combination
			if typeChanged && nodeType == wedev.NodeTypePeer && publicAddress == "" {
				return fmt.Errorf("peer type nodes require a public address (use --public-address)")
			}

			// If already peer type and trying to clear public address
			if node.Type == wedev.NodeTypePeer && publicAddressProvided && publicAddress == "" && nodeType == wedev.NodeTypePeer {
				return fmt.Errorf("cannot clear public address for peer type nodes (change type to route first)")
			}

			// Use current port if not specified
			if port == 0 {
				port = node.Port
			}

			updated, err := vnManager.UpdateNode(nodeName, publicAddress, port, nodeType)
			if err != nil {
				return fmt.Errorf("failed to update node: %w", err)
			}

			fmt.Printf("Node '%s' updated successfully\n", updated.Name)
			fmt.Printf("Type: %s\n", updated.Type)
			if updated.PublicAddress != "" {
				fmt.Printf("Public Address: %s:%d\n", updated.PublicAddress, updated.Port)
			} else {
				fmt.Printf("Public Address: (none)\n")
			}

			return nil
		},
	}

	cmd.Flags().String("public-address", "", "Public address or domain (empty string to clear for route type)")
	cmd.Flags().Int("port", 0, "Port number")
	cmd.Flags().String("type", "", "Node type (peer or route)")

	return cmd
}

// makeNodeDeleteCommand creates the 'node delete' command for a specific network.
func makeNodeDeleteCommand(_ string) *cobra.Command {
	return &cobra.Command{
		Use:   "delete <node-name>",
		Short: "Delete a node",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			nodeName := args[0]

			if !confirmAction(fmt.Sprintf("Delete node '%s'?", nodeName)) {
				fmt.Println("Cancelled")
				return nil
			}

			err := vnManager.DeleteNode(nodeName)
			if err != nil {
				return fmt.Errorf("failed to delete node: %w", err)
			}

			fmt.Println("Node deleted successfully")
			return nil
		},
	}
}

// ========== Config Commands ==========

// makeConfigCommand creates the 'config' command group for a specific network
func makeConfigCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage WireGuard configurations",
		Long:  fmt.Sprintf("Manage WireGuard configurations for virtual network '%s'", networkName),
	}

	cmd.AddCommand(makeConfigGenerateCommand(networkName))
	cmd.AddCommand(makeConfigInfoCommand(networkName))
	cmd.AddCommand(makeConfigHistoryCommand(networkName))

	return cmd
}

// makeConfigGenerateCommand creates the 'config generate' command for a specific network
func makeConfigGenerateCommand(networkName string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate WireGuard configuration files",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			outputDir, err := cmd.Flags().GetString("output-dir")
			if err != nil {
				return fmt.Errorf("failed to get output-dir flag: %w", err)
			}
			force, err := cmd.Flags().GetBool("force")
			if err != nil {
				return fmt.Errorf("failed to get force flag: %w", err)
			}

			if outputDir == "" {
				var getWdErr error
				outputDir, getWdErr = os.Getwd()
				if getWdErr != nil {
					return fmt.Errorf("failed to get current directory: %w", getWdErr)
				}
			}

			// Ensure directory exists
			mkdirErr := os.MkdirAll(outputDir, 0o700)
			if mkdirErr != nil {
				return fmt.Errorf("failed to create output directory: %w", mkdirErr)
			}

			generator := wedev.NewWireGuardConfigGenerator(storage)
			configs, _, err := generator.GenerateConfigs(networkName, storage)
			if err != nil {
				return fmt.Errorf("failed to generate configs: %w", err)
			}

			// Check for existing files
			var existingFiles []string
			for name := range configs {
				filePath := filepath.Join(outputDir, name+".conf")
				if _, statErr := os.Stat(filePath); statErr == nil {
					existingFiles = append(existingFiles, filePath)
				}
			}

			// Ask for overwrite confirmation
			if len(existingFiles) > 0 && !force {
				fmt.Println("The following files already exist:")
				for _, f := range existingFiles {
					fmt.Printf("  %s\n", f)
				}
				if !confirmAction("Overwrite existing files?") {
					fmt.Println("Cancelled")
					return nil
				}
			}

			// Write files
			for name, config := range configs {
				filePath := filepath.Join(outputDir, name+".conf")
				if writeErr := os.WriteFile(filePath, []byte(config), 0o600); writeErr != nil {
					return fmt.Errorf("failed to write config file %s: %w", filePath, writeErr)
				}
				fmt.Printf("Generated: %s\n", filePath)
			}

			// Save version
			version, created, err := generator.SaveConfigVersion(networkName)
			if err != nil {
				return fmt.Errorf("failed to save config version: %w", err)
			}

			if created {
				fmt.Printf("\nConfiguration version %d saved\n", version.Version)
			} else {
				fmt.Println("\nNo changes detected, version not updated")
			}

			return nil
		},
	}

	cmd.Flags().String("output-dir", "", "Output directory (default: current directory)")
	cmd.Flags().Bool("force", false, "Skip all interactive confirmations")

	return cmd
}

// makeConfigInfoCommand creates the 'config info' command for a specific network
func makeConfigInfoCommand(networkName string) *cobra.Command {
	return &cobra.Command{
		Use:   "info [version]",
		Short: "View configuration information",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {

			generator := wedev.NewWireGuardConfigGenerator(storage)

			var version *wedev.ConfigVersion
			var err error

			if len(args) == 1 {
				var ver int
				_, parseErr := fmt.Sscanf(args[0], "%d", &ver)
				if parseErr != nil {
					return fmt.Errorf("invalid version number: %w", parseErr)
				}
				version, err = generator.GetConfig(networkName, ver)
			} else {
				history, histErr := generator.GetConfigHistory(networkName)
				if histErr != nil || len(history) == 0 {
					return fmt.Errorf("no configuration versions found")
				}
				version = history[len(history)-1]
			}

			if err != nil {
				return fmt.Errorf("failed to get configuration: %w", err)
			}

			fmt.Printf("Configuration Version: %d\n", version.Version)
			fmt.Printf("Content Hash: %s\n", version.ContentHash)
			fmt.Printf("Created At: %s\n", version.CreatedAt)
			fmt.Printf("Files:\n")
			for name := range version.Configs {
				fmt.Printf("  - %s.conf\n", name)
			}

			return nil
		},
	}
}

// makeConfigHistoryCommand creates the 'config history' command for a specific network
func makeConfigHistoryCommand(networkName string) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "View configuration history",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {

			generator := wedev.NewWireGuardConfigGenerator(storage)
			history, err := generator.GetConfigHistory(networkName)
			if err != nil {
				return fmt.Errorf("failed to get config history: %w", err)
			}

			if len(history) == 0 {
				fmt.Println("No configuration versions found")
				return nil
			}

			fmt.Printf("%-8s %-35s %-20s\n", "Version", "Hash", "Created")
			fmt.Println("--------------------------------------------------------------")
			for _, cfg := range history {
				fmt.Printf("%-8d %-35s %-20s\n", cfg.Version, cfg.ContentHash, cfg.CreatedAt.Format("2006-01-02 15:04:05"))
			}

			return nil
		},
	}
}

// confirmAction prompts user for confirmation.
func confirmAction(prompt string) bool {
	// For testing, we may redirect stdin
	fmt.Printf("%s (y/n): ", prompt)
	var response string
	_, err := fmt.Scanln(&response)
	if err != nil && err != io.EOF {
		return false
	}
	return response == "y" || response == "Y" || response == "yes" || response == "YES"
}
