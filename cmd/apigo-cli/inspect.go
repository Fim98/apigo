package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"apigo/internal/collection"
)

const inspectUsage = `apigo inspect - print a collection tree

Usage:
  apigo inspect [--json] <collection-dir>

Flags:
  --json   print a machine-readable description
`

// inspectOptions holds the flags of the inspect command.
type inspectOptions struct {
	json bool
}

// runInspect prints a collection's tree.
//
// Exit codes: 0 on success, 2 for usage or load errors.
func runInspect(args []string, stdout, stderr io.Writer) int {
	var opts inspectOptions
	var root string
	for _, arg := range args {
		switch arg {
		case "--json":
			opts.json = true
		case "--help", "-h":
			fmt.Fprint(stdout, inspectUsage)
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "apigo inspect: unknown flag %q\n\n%s", arg, inspectUsage)
				return 2
			}
			if root != "" {
				fmt.Fprintf(stderr, "apigo inspect: one collection at a time\n\n%s", inspectUsage)
				return 2
			}
			root = arg
		}
	}
	if root == "" {
		fmt.Fprintf(stderr, "apigo inspect: missing collection directory\n\n%s", inspectUsage)
		return 2
	}

	c, err := collection.Load(context.Background(), root)
	if err != nil {
		fmt.Fprintf(stderr, "apigo inspect: %v\n", err)
		return 2
	}

	if opts.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(describe(c)); err != nil {
			fmt.Fprintf(stderr, "apigo inspect: %v\n", err)
			return 2
		}
		return 0
	}

	name := c.Config.Name
	if name == "" {
		name = c.Root
	}
	fmt.Fprintf(stdout, "%s\n", name)
	writeTree(stdout, c.Tree, "")

	requests := c.Requests()
	fmt.Fprintf(stdout, "\n%d request(s), %d environment(s)\n", len(requests), len(c.Environments))
	if c.DotEnv != nil {
		fmt.Fprintf(stdout, ".env: %d variable(s)\n", len(c.DotEnv.Vars))
	}
	for _, warning := range c.Warnings {
		fmt.Fprintf(stderr, "warning: %v\n", warning)
	}
	return 0
}

// writeTree renders nodes with box-drawing prefixes.
func writeTree(w io.Writer, nodes []*collection.Node, prefix string) {
	for i, node := range nodes {
		last := i == len(nodes)-1
		connector := "├── "
		next := prefix + "│   "
		if last {
			connector = "└── "
			next = prefix + "    "
		}
		if node.IsFolder() {
			fmt.Fprintf(w, "%s%s%s/\n", prefix, connector, node.Name)
			writeTree(w, node.Children, next)
			continue
		}
		fmt.Fprintf(w, "%s%s%-6s %s\n", prefix, connector, node.Request.Method, node.Name)
	}
}

// requestSummary, nodeSummary, environmentSummary and collectionSummary are the
// JSON shape of inspect --json.
type collectionSummary struct {
	Root         string               `json:"root"`
	Name         string               `json:"name"`
	Requests     []requestSummary     `json:"requests"`
	Environments []environmentSummary `json:"environments"`
	DotEnv       *dotenvSummary       `json:"dotenv,omitempty"`
	Warnings     []string             `json:"warnings,omitempty"`
}

type requestSummary struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	URL    string `json:"url"`
	Path   string `json:"path"`
	Type   string `json:"type"`
	Seq    int    `json:"seq"`
}

// dotenvSummary lists the names in `.env`. Values are omitted, because inspect
// output is meant to be pasteable into an issue or a CI log.
type dotenvSummary struct {
	Path string   `json:"path"`
	Vars []string `json:"vars"`
}

type environmentSummary struct {
	Name    string   `json:"name"`
	Extends []string `json:"extends,omitempty"`
	Secrets int      `json:"secrets"`
	Vars    int      `json:"vars"`
}

func describe(c *collection.Collection) collectionSummary {
	summary := collectionSummary{Root: c.Root, Name: c.Config.Name, Requests: []requestSummary{}}
	for _, req := range c.Requests() {
		summary.Requests = append(summary.Requests, requestSummary{
			Name:   req.Name(),
			Method: req.Method,
			URL:    req.URL,
			Path:   req.RelPath,
			Type:   req.Meta.Type,
			Seq:    req.Meta.Seq,
		})
	}
	for _, env := range c.Environments {
		summary.Environments = append(summary.Environments, environmentSummary{
			Name:    env.Name,
			Extends: env.Extends,
			Secrets: len(env.SecretNames),
			Vars:    len(env.Vars),
		})
	}
	if c.DotEnv != nil {
		names := make([]string, 0, len(c.DotEnv.Vars))
		for _, v := range c.DotEnv.Vars {
			names = append(names, v.Name)
		}
		summary.DotEnv = &dotenvSummary{Path: c.DotEnv.Path, Vars: names}
	}
	for _, warning := range c.Warnings {
		summary.Warnings = append(summary.Warnings, warning.Error())
	}
	return summary
}
