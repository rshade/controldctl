package main

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/mcp"

	"github.com/rshade/controldctl/internal/controld"
)

// version is set via -ldflags "-X main.version=..." at build time (see
// .goreleaser.yaml). ax.ResolveVersion falls back to Go build metadata when
// this is empty, e.g. for `go install` or unreleased builds.
var version string

func main() {
	os.Exit(run(context.Background(), rewriteMCPFlag(os.Args[1:]), os.Stdin, os.Stdout, os.Stderr))
}

// rewriteMCPFlag turns a leading "--mcp" into the "mcp-server" subcommand
// name, so "controldctl --mcp <rest>" behaves exactly like
// "controldctl mcp-server <rest>". This is a pure argument rewrite: there is
// only one code path underneath (ax-go's mcp.NewCommand), so --help and
// __schema output never mention a flag that mcp-server's own Cobra
// definition doesn't already have.
func rewriteMCPFlag(args []string) []string {
	if len(args) == 0 || args[0] != "--mcp" {
		return args
	}
	rewritten := make([]string, 0, len(args))
	rewritten = append(rewritten, "mcp-server")
	rewritten = append(rewritten, args[1:]...)
	return rewritten
}

// run takes stdin/stdout/stderr as the io.Reader/io.Writer interfaces ax.WithStdin
// et al. actually accept (not the concrete *os.File main() passes) — this is the
// same test seam ax-go's own examples/integration/main.go uses, and it's required
// for later tests to substitute a bytes.Buffer for stdout.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	resolved := ax.ResolveVersion(version)

	factory := func(cmd *cobra.Command) (*controld.API, error) {
		apiToken, _ := cmd.Flags().GetString("api-token")
		configPath, _ := cmd.Flags().GetString("config")
		return controld.NewClient(cmd.Context(), apiToken, configPath, "", os.Getenv)
	}

	root := newRootCommand(factory)
	root.AddCommand(mcp.NewCommand(root, mcp.WithVersion(resolved)))
	root.SetArgs(args)

	return ax.Execute(
		ctx,
		root,
		ax.WithStdin(stdin),
		ax.WithStdout(stdout),
		ax.WithStderr(stderr),
		ax.WithEnv(os.Getenv),
		ax.WithVersion(resolved),
	)
}
