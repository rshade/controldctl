package main

import (
	"context"
	"io"
	"os"

	"github.com/rshade/ax-go"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run takes stdin/stdout/stderr as the io.Reader/io.Writer interfaces ax.WithStdin
// et al. actually accept (not the concrete *os.File main() passes) — this is the
// same test seam ax-go's own examples/integration/main.go uses, and it's required
// for later tests to substitute a bytes.Buffer for stdout.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root := newRootCommand()
	root.SetArgs(args)

	return ax.Execute(
		ctx,
		root,
		ax.WithStdin(stdin),
		ax.WithStdout(stdout),
		ax.WithStderr(stderr),
		ax.WithEnv(os.Getenv),
		ax.WithVersion(ax.ResolveVersion("")),
	)
}
