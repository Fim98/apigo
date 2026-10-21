// Command apigo-cli runs API collections without the desktop interface.
//
// It shares the collection, request and runner packages with the app, so a
// collection behaves the same in a terminal and in CI. The command surface is
// intentionally small while the engine is being built: see docs/BACKLOG.md
// (milestone M6) for the planned Bruno-compatible commands and flags.
package main

import (
	"fmt"
	"io"
	"os"

	"apigo/internal/version"
)

const usage = `apigo - run API collections from the command line

Usage:
  apigo <command> [flags]

Commands:
  version    print the version
  fmt        rewrite .bru files in canonical form
  inspect    print a collection tree
  help       print this help

The run, import and docs commands arrive with milestone M6; see docs/BACKLOG.md.
`

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(stderr, usage)
		return 2
	}

	switch args[0] {
	case "version", "--version", "-v":
		fmt.Fprintf(stdout, "apigo %s\n", version.Value)
		return 0
	case "help", "--help", "-h":
		fmt.Fprint(stdout, usage)
		return 0
	case "fmt":
		return runFmt(args[1:], stdout, stderr)
	case "inspect":
		return runInspect(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "apigo: unknown command %q\n\n%s", args[0], usage)
		return 2
	}
}
