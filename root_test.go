package main

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/rshade/controldctl/internal/controld"
)

func noopFactory(*cobra.Command) (*controld.API, error) { return nil, nil }

func TestNewRootCommandHasExpectedUse(t *testing.T) {
	root := newRootCommand(noopFactory)
	if root.Use != "controldctl" {
		t.Fatalf("Use = %q, want %q", root.Use, "controldctl")
	}
}
