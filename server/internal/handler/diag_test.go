package handler

import (
	"context"
	"fmt"
	"runtime"
	"testing"
)

func TestDiagnostic(t *testing.T) {
	fmt.Printf("DIAG: Go version: %s\n", runtime.Version())
	fmt.Printf("DIAG: testHandler nil: %v\n", testHandler == nil)
	fmt.Printf("DIAG: testPool nil: %v\n", testPool == nil)
	fmt.Printf("DIAG: testUserID: %q\n", testUserID)
	fmt.Printf("DIAG: testWorkspaceID: %q\n", testWorkspaceID)

	if testHandler == nil {
		t.Skip("handler not initialized (no DB)")
		return
	}

	ctx := context.Background()

	// Try a simple query
	var count int
	err := testPool.QueryRow(ctx, `SELECT count(*) FROM workspace WHERE id = $1`, testWorkspaceID).Scan(&count)
	if err != nil {
		t.Fatalf("DIAG: workspace query failed: %v", err)
	}
	fmt.Printf("DIAG: workspace count: %d\n", count)

	// Check workspace settings
	var settings []byte
	err = testPool.QueryRow(ctx, `SELECT settings FROM workspace WHERE id = $1`, testWorkspaceID).Scan(&settings)
	if err != nil {
		t.Fatalf("DIAG: settings query failed: %v", err)
	}
	fmt.Printf("DIAG: workspace settings: %s\n", string(settings))
}
