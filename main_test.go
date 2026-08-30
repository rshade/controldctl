package main

import (
	"reflect"
	"testing"
)

func TestRewriteMCPFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"no flag, unchanged", []string{"devices", "list"}, []string{"devices", "list"}},
		{"bare flag becomes subcommand", []string{"--mcp"}, []string{"mcp-server"}},
		{"flag with trailing args", []string{"--mcp", "--transport=http"}, []string{"mcp-server", "--transport=http"}},
		{"empty args", []string{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteMCPFlag(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rewriteMCPFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
