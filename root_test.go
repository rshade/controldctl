package main

import "testing"

func TestNewRootCommandHasExpectedUse(t *testing.T) {
	root := newRootCommand()
	if root.Use != "controldctl" {
		t.Fatalf("Use = %q, want %q", root.Use, "controldctl")
	}
}
