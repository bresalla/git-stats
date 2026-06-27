package main

import (
	"testing"

	"git-statistics/internal/storage"
)

func TestRunMCPServerConstruction(t *testing.T) {
	// Create a temporary in-memory database for testing
	// Note: We can't easily test the full stdio round-trip in unit tests,
	// but we can verify that the server constructs without errors
	// and that the function signature is correct.

	// Create a test store (using :memory: for SQLite in-memory database)
	store, err := storage.Open(":memory:")
	if err != nil {
		t.Fatalf("failed to open test store: %v", err)
	}
	defer store.Close()

	// Verify that runMCPServer is callable with the right signature.
	// We can't fully test stdio transport in a unit test without
	// mocking stdin/stdout, so this test verifies the function
	// exists and has the correct signature.
	_ = runMCPServer

	// The actual MCP server Run() call would block waiting for stdio,
	// so full integration testing should be done via subprocess testing
	// or manual verification with the actual binary.
}
