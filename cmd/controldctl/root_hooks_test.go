package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go/mcp"
)

// productionRoot assembles the tree run() builds, including the mcp-server
// subcommand main() mounts after newRootCommand returns, so the walk below
// covers every command a real invocation can reach.
func productionRoot() *cobra.Command {
	root := newRootCommand(noopFactory)
	root.AddCommand(mcp.NewCommand(root))
	return root
}

// walkSubcommands visits every command below root, depth first. The root is
// deliberately excluded: it owns the one persistent hook this CLI is allowed
// to declare.
func walkSubcommands(root *cobra.Command, visit func(*cobra.Command)) {
	for _, sub := range root.Commands() {
		visit(sub)
		walkSubcommands(sub, visit)
	}
}

// TestNoSubcommandDeclaresPersistentPreRun pins the one refactor that would
// silently disarm every agent-safety guarantee this CLI makes. Resolving
// shared state once in a command group's PersistentPreRunE is the obvious
// cleanup — and root.go already models it — but only the root may do it.
func TestNoSubcommandDeclaresPersistentPreRun(t *testing.T) {
	var offenders []string
	walkSubcommands(productionRoot(), func(cmd *cobra.Command) {
		if cmd.PersistentPreRun != nil || cmd.PersistentPreRunE != nil {
			offenders = append(offenders, cmd.CommandPath())
		}
	})

	if len(offenders) > 0 {
		t.Fatalf("commands below the root declare their own persistent hook: %s\n\n"+
			"ax.Execute wraps only root.PersistentPreRunE, and Cobra runs solely the "+
			"nearest persistent hook in the ancestor chain, so a hook here replaces "+
			"ax-go's wrapper outright and the agent-safety context is never installed. "+
			"ax.Guard then performs the real side effect under --dry-run, "+
			"orgIDFromContext returns \"\" so requireConfirmation drops the --org-id "+
			"gate on non-destructive mutations, and the run still exits 0.\n\n"+
			"Resolve per-subtree state in RunE, or in a non-persistent PreRunE, instead.",
			strings.Join(offenders, ", "))
	}
}

// TestRootDeclaresPersistentPreRun is the other half of that invariant: the
// wrapper ax.Execute installs chains the root hook rather than supplying one,
// so deleting the root hook drops org-ID resolution and with it the
// cross-tenant confirmation gate requireConfirmation reads.
func TestRootDeclaresPersistentPreRun(t *testing.T) {
	if productionRoot().PersistentPreRunE == nil {
		t.Fatal("root declares no PersistentPreRunE; --org-id resolution and the " +
			"confirmation gate that depends on it are gone")
	}
}
