package cmd

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test VN Add Command - Can be created
func TestVNAddCommand(t *testing.T) {
	cmd := NewVNAddCommand()
	if cmd == nil {
		t.Errorf("NewVNAddCommand() returned nil")
	}
}

// Test VN List Command - Can be created
func TestVNListCommand(t *testing.T) {
	cmd := NewVNListCommand()
	if cmd == nil {
		t.Errorf("NewVNListCommand() returned nil")
	}
}

// Test Server Add Command - Can be created
func TestServerAddCommand(t *testing.T) {
	cmd := makeServerAddCommand("test-network")
	if cmd == nil {
		t.Errorf("makeServerAddCommand() returned nil")
	}
}

// Test Server Info Command - Can be created
func TestServerInfoCommand(t *testing.T) {
	cmd := makeServerInfoCommand("test-network")
	if cmd == nil {
		t.Errorf("makeServerInfoCommand() returned nil")
	}
}

// Test Server Edit Command - Can be created
func TestServerEditCommand(t *testing.T) {
	cmd := makeServerEditCommand("test-network")
	if cmd == nil {
		t.Errorf("makeServerEditCommand() returned nil")
	}
}

// Test Server Delete Command - Can be created
func TestServerDeleteCommand(t *testing.T) {
	cmd := makeServerDeleteCommand("test-network")
	if cmd == nil {
		t.Errorf("makeServerDeleteCommand() returned nil")
	}
}

// Test Node Add Command - Can be created
func TestNodeAddCommand(t *testing.T) {
	cmd := makeNodeAddCommand("test-network")
	if cmd == nil {
		t.Errorf("makeNodeAddCommand() returned nil")
	}
}

// Test Node List Command - Can be created
func TestNodeListCommand(t *testing.T) {
	cmd := makeNodeListCommand("test-network")
	if cmd == nil {
		t.Errorf("makeNodeListCommand() returned nil")
	}
}

// Test Node Edit Command - Can be created
func TestNodeEditCommand(t *testing.T) {
	cmd := makeNodeEditCommand("test-network")
	if cmd == nil {
		t.Errorf("makeNodeEditCommand() returned nil")
	}
}

// Test Node Delete Command - Can be created
func TestNodeDeleteCommand(t *testing.T) {
	cmd := makeNodeDeleteCommand("test-network")
	if cmd == nil {
		t.Errorf("makeNodeDeleteCommand() returned nil")
	}
}

// Test Config Generate Command - Can be created
func TestConfigGenerateCommand(t *testing.T) {
	cmd := makeConfigGenerateCommand("test-network")
	if cmd == nil {
		t.Errorf("makeConfigGenerateCommand() returned nil")
	}
}

// Test Config History Command - Can be created
func TestConfigHistoryCommand(t *testing.T) {
	cmd := makeConfigHistoryCommand("test-network")
	if cmd == nil {
		t.Errorf("makeConfigHistoryCommand() returned nil")
	}
}

// Test Config Info Command - Can be created
func TestConfigInfoCommand(t *testing.T) {
	cmd := makeConfigInfoCommand("test-network")
	if cmd == nil {
		t.Errorf("makeConfigInfoCommand() returned nil")
	}
}

// Test VN Delete Command - Can be created
func TestVNDeleteCommand(t *testing.T) {
	cmd := NewVNDeleteCommand()
	if cmd == nil {
		t.Errorf("NewVNDeleteCommand() returned nil")
	}
}

// TestCustomDBPath tests using custom database path via environment variable
func TestCustomDBPath(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()
	customDBDir := filepath.Join(tmpDir, "custom-wedevctl")

	// Set environment variable
	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Setenv("WEDEVCTL_DB_PATH", customDBDir)
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	// Execute a simple command to trigger database initialization
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"vn", "list"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err != nil {
		t.Logf("Command execution error (expected in test): %v", err)
	}

	// Verify database was created in custom location
	expectedDBPath := filepath.Join(customDBDir, "wedevctl.db")
	_, err = os.Stat(expectedDBPath)
	if err != nil {
		t.Errorf("Database should exist at custom path %s, got error: %v", expectedDBPath, err)
	}

	// Verify directory permissions
	info, err := os.Stat(customDBDir)
	if err != nil {
		t.Errorf("Failed to stat directory: %v", err)
	} else if info.Mode().Perm() != 0o700 {
		t.Errorf("Directory permissions = %v, want 0o700", info.Mode().Perm())
	}
}

// TestRelativeDBPath tests using relative path for database
func TestRelativeDBPath(t *testing.T) {
	// Create temporary directory and change to it
	tmpDir := t.TempDir()
	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current working directory: %v", err)
	}
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	// Set relative path
	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Setenv("WEDEVCTL_DB_PATH", "./data")
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	// Execute command
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"vn", "list"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err = rootCmd.Execute()
	if err != nil {
		t.Logf("Command execution error (expected in test): %v", err)
	}

	// Verify database exists at absolute path
	expectedPath := filepath.Join(tmpDir, "data", "wedevctl.db")
	_, err = os.Stat(expectedPath)
	if err != nil {
		t.Errorf("Database should exist at resolved absolute path %s, got error: %v", expectedPath, err)
	}
}

// TestDefaultDBPath tests default database path behavior
func TestDefaultDBPath(t *testing.T) {
	// Ensure no custom path is set
	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Unsetenv("WEDEVCTL_DB_PATH")
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	// We can't easily test the actual home directory in unit tests,
	// but we can verify the command doesn't error during initialization
	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"vn", "list"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	// Just verify it doesn't panic or fail initialization
	err := rootCmd.Execute()
	if err != nil {
		t.Logf("Command execution error (expected in test with empty db): %v", err)
	}
	// Not checking for specific error since we're just testing initialization
}

// TestDBPathPrecedence tests that environment variable takes precedence
func TestDBPathPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	customPath := filepath.Join(tmpDir, "custom-db")

	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Setenv("WEDEVCTL_DB_PATH", customPath)
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"vn", "list"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	_ = rootCmd.Execute()

	// Verify database created in custom location
	dbFile := filepath.Join(customPath, "wedevctl.db")
	if _, err := os.Stat(dbFile); err != nil {
		t.Errorf("Database should exist at %s", dbFile)
	}
}

// TestDBDirectoryCreation tests that nested directories are created
func TestDBDirectoryCreation(t *testing.T) {
	tmpDir := t.TempDir()
	nestedPath := filepath.Join(tmpDir, "level1", "level2", "level3")

	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Setenv("WEDEVCTL_DB_PATH", nestedPath)
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"vn", "list"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	_ = rootCmd.Execute()

	// Verify directory was created with correct permissions
	info, err := os.Stat(nestedPath)
	if err != nil {
		t.Fatalf("Directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("Path should be a directory")
	}
	if info.Mode().Perm() != 0o700 {
		t.Errorf("Directory permissions = %v, want 0o700", info.Mode().Perm())
	}
}

// TestConfirmActionYes tests confirmation with yes response
func TestConfirmActionYes(t *testing.T) {
	// Mock stdin with "y"
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	// Write input and close writer in goroutine
	go func() {
		defer w.Close()
		if _, err := w.WriteString("y\n"); err != nil {
			t.Errorf("failed to write to pipe: %v", err)
		}
	}()

	result := confirmAction("Test prompt")
	if !result {
		t.Error("Expected true for 'y' input")
	}
}

// TestConfirmActionNo tests confirmation with no response
func TestConfirmActionNo(t *testing.T) {
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdin = r
	defer func() {
		os.Stdin = oldStdin
		r.Close()
	}()

	// Write input and close writer in goroutine
	go func() {
		defer w.Close()
		if _, err := w.WriteString("n\n"); err != nil {
			t.Errorf("failed to write to pipe: %v", err)
		}
	}()

	result := confirmAction("Test prompt")
	if result {
		t.Error("Expected false for 'n' input")
	}
}

// TestVirtualNetworkCommandHelp tests vn command help
func TestVirtualNetworkCommandHelp(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Setenv("WEDEVCTL_DB_PATH", tmpDir)
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	cmd := NewVirtualNetworkCommand()
	cmd.SetArgs([]string{})
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)

	// Should show help without error
	err := cmd.Execute()
	if err != nil {
		t.Errorf("Help should not error: %v", err)
	}
}

// TestVirtualNetworkCommandNonExistentNetwork tests accessing non-existent network
func TestVirtualNetworkCommandNonExistentNetwork(t *testing.T) {
	tmpDir := t.TempDir()
	oldPath := os.Getenv("WEDEVCTL_DB_PATH")
	os.Setenv("WEDEVCTL_DB_PATH", tmpDir)
	defer os.Setenv("WEDEVCTL_DB_PATH", oldPath)

	rootCmd := NewRootCommand()
	rootCmd.SetArgs([]string{"vn", "nonexistent", "server", "info"})
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)

	err := rootCmd.Execute()
	if err == nil {
		t.Error("Expected error for non-existent network")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestMakeServerCommand tests server command group creation
func TestMakeServerCommand(t *testing.T) {
	cmd := makeServerCommand("test-net")
	if cmd == nil {
		t.Error("makeServerCommand returned nil")
	}
	if len(cmd.Commands()) != 4 {
		t.Errorf("Expected 4 subcommands, got %d", len(cmd.Commands()))
	}
}

// TestMakeNodeCommand tests node command group creation
func TestMakeNodeCommand(t *testing.T) {
	cmd := makeNodeCommand("test-net")
	if cmd == nil {
		t.Error("makeNodeCommand returned nil")
	}
	if len(cmd.Commands()) != 4 {
		t.Errorf("Expected 4 subcommands, got %d", len(cmd.Commands()))
	}
}

// TestMakeConfigCommand tests config command group creation
func TestMakeConfigCommand(t *testing.T) {
	cmd := makeConfigCommand("test-net")
	if cmd == nil {
		t.Error("makeConfigCommand returned nil")
	}
	if len(cmd.Commands()) != 3 {
		t.Errorf("Expected 3 subcommands, got %d", len(cmd.Commands()))
	}
}

// TestRootCommandCreation tests root command creation
func TestRootCommandCreation(t *testing.T) {
	cmd := NewRootCommand()
	if cmd == nil {
		t.Error("NewRootCommand returned nil")
		return
	}
	if cmd.Use != "wedevctl" {
		t.Errorf("Expected 'wedevctl', got '%s'", cmd.Use)
	}
}
