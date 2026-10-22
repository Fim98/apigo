package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"apigo/internal/collection"
	"apigo/internal/vars"
)

const varsUsage = `apigo vars - print the variables a request would see

Usage:
  apigo vars [flags] <collection-dir>

Flags:
  --env NAME       resolve the named environment
  --request REL    request path relative to the collection root
  --set NAME=VALUE set or override an environment variable (repeatable)
  --text STRING    interpolate STRING instead of listing the layers
  --json           print a machine-readable description

Without --request the collection root is used, which shows the collection and
environment layers. Secret values are masked, and .env is listed by name only;
the OS environment is never dumped. With --text the result is printed on its own,
followed by any unresolved names; the exit code stays 0 either way.
`

const maskedValue = "********"

// varsOptions holds the flags of the vars command.
type varsOptions struct {
	environment string
	request     string
	overrides   map[string]string
	text        string
	hasText     bool
	json        bool
}

// runVars prints the variable scopes of one request.
//
// Exit codes: 0 on success, 2 for usage, load or resolution errors.
func runVars(args []string, stdout, stderr io.Writer) int {
	opts := varsOptions{overrides: map[string]string{}}
	var root string

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "--help" || arg == "-h":
			fmt.Fprint(stdout, varsUsage)
			return 0
		case arg == "--env" || arg == "--request" || arg == "--set" || arg == "--text":
			if i+1 >= len(args) {
				fmt.Fprintf(stderr, "apigo vars: %s needs a value\n\n%s", arg, varsUsage)
				return 2
			}
			i++
			if code := opts.apply(arg, args[i], stderr); code != 0 {
				return code
			}
		case strings.HasPrefix(arg, "--env=") || strings.HasPrefix(arg, "--request=") ||
			strings.HasPrefix(arg, "--set=") || strings.HasPrefix(arg, "--text="):
			name, value, _ := strings.Cut(arg, "=")
			if code := opts.apply(name, value, stderr); code != 0 {
				return code
			}
		case strings.HasPrefix(arg, "-"):
			fmt.Fprintf(stderr, "apigo vars: unknown flag %q\n\n%s", arg, varsUsage)
			return 2
		default:
			if root != "" {
				fmt.Fprintf(stderr, "apigo vars: one collection at a time\n\n%s", varsUsage)
				return 2
			}
			root = arg
		}
	}
	if root == "" {
		fmt.Fprintf(stderr, "apigo vars: missing collection directory\n\n%s", varsUsage)
		return 2
	}

	c, err := collection.Load(context.Background(), root)
	if err != nil {
		fmt.Fprintf(stderr, "apigo vars: %v\n", err)
		return 2
	}

	node, err := nodeAt(c, opts.request)
	if err != nil {
		fmt.Fprintf(stderr, "apigo vars: %v\n", err)
		return 2
	}

	var environment *collection.Environment
	if opts.environment != "" {
		for _, env := range c.Environments {
			if env.Name == opts.environment {
				environment = env
				break
			}
		}
		if environment == nil {
			fmt.Fprintf(stderr, "apigo vars: no environment %q in %s\n", opts.environment, c.Root)
			return 2
		}
	}

	set, err := vars.ForRequest(c, node, vars.Options{
		Environment:          environment,
		EnvironmentOverrides: opts.overrides,
		ProcessEnv:           c.ProcessEnv(os.Environ(), nil),
	})
	if err != nil {
		fmt.Fprintf(stderr, "apigo vars: %v\n", err)
		return 2
	}

	if opts.hasText {
		return writeInterpolation(stdout, stderr, set, opts)
	}

	if opts.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(describeVars(c, node, environment, set)); err != nil {
			fmt.Fprintf(stderr, "apigo vars: %v\n", err)
			return 2
		}
		return 0
	}

	writeVars(stdout, c, node, environment, set)
	for _, warning := range c.Warnings {
		fmt.Fprintf(stderr, "warning: %v\n", warning)
	}
	return 0
}

// apply stores one flag value.
func (o *varsOptions) apply(flag, value string, stderr io.Writer) int {
	switch flag {
	case "--env":
		o.environment = value
	case "--request":
		o.request = value
	case "--set":
		name, text, ok := strings.Cut(value, "=")
		if !ok || name == "" {
			fmt.Fprintf(stderr, "apigo vars: --set wants NAME=VALUE, got %q\n", value)
			return 2
		}
		o.overrides[name] = text
	case "--text":
		o.text = value
		o.hasText = true
	}
	return 0
}

// writeInterpolation prints one interpolated string.
//
// Secrets are masked: this output is meant to be pasted into an issue as often as
// it is piped into a command. The result goes to stdout alone, so
// `apigo vars --text ... | xargs curl` works; unresolved names are diagnostics
// and go to stderr.
func writeInterpolation(stdout, stderr io.Writer, set *vars.Set, opts varsOptions) int {
	result := set.Interpolate(opts.text, vars.InterpolateOptions{MaskSecrets: true})
	if opts.json {
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		payload := struct {
			Text       string   `json:"text"`
			Kind       string   `json:"kind"`
			Secret     bool     `json:"secret,omitempty"`
			Unresolved []string `json:"unresolved,omitempty"`
		}{Text: result.Text, Kind: string(result.Kind), Secret: result.Secret, Unresolved: result.Unresolved}
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintf(stderr, "apigo vars: %v\n", err)
			return 2
		}
		return 0
	}

	fmt.Fprintln(stdout, result.Text)
	if len(result.Unresolved) > 0 {
		fmt.Fprintf(stderr, "unresolved: %s\n", strings.Join(result.Unresolved, ", "))
	}
	return 0
}

// nodeAt finds a node by its slash-separated relative path. An empty path means
// the collection root.
func nodeAt(c *collection.Collection, rel string) (*collection.Node, error) {
	// RelPath is always slash-separated; accept a Windows-style path too.
	rel = strings.ReplaceAll(rel, "\\", "/")
	rel = strings.TrimPrefix(strings.Trim(rel, "/"), "./")
	if rel == "" {
		if root := c.RootNode(); root != nil {
			return root, nil
		}
		return nil, fmt.Errorf("%s has no root node", c.Root)
	}

	var found *collection.Node
	if err := c.Walk(func(n *collection.Node) error {
		if n.RelPath == rel {
			found = n
		}
		return nil
	}); err != nil {
		return nil, err
	}
	if found == nil {
		return nil, fmt.Errorf("no request or folder %q in %s", rel, c.Root)
	}
	return found, nil
}

// scopedValue is one entry of the vars output.
type scopedValue struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Scope    string `json:"scope"`
	Kind     string `json:"kind"`
	Secret   bool   `json:"secret"`
	Shadowed bool   `json:"shadowed"`
}

type varsSummary struct {
	Root        string        `json:"root"`
	Request     string        `json:"request,omitempty"`
	Environment string        `json:"environment,omitempty"`
	Precedence  []string      `json:"precedence"`
	Variables   []scopedValue `json:"variables"`
	// DotEnv lists the collection's .env names. Values are omitted, and the OS
	// environment is never listed: it is noise, and it holds CI credentials.
	DotEnv   []string `json:"dotenv,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// describeVars builds the JSON shape: every layer's entries from lowest to
// highest priority, each marked shadowed when a higher layer wins the name.
func describeVars(c *collection.Collection, node *collection.Node, env *collection.Environment, set *vars.Set) varsSummary {
	summary := varsSummary{Root: c.Root, Request: node.RelPath, Variables: []scopedValue{}}
	if env != nil {
		summary.Environment = env.Name
	}
	for _, scope := range vars.Precedence() {
		summary.Precedence = append(summary.Precedence, scope.String())
	}

	for _, scope := range vars.Precedence() {
		for _, name := range set.Names(scope) {
			value, ok := set.Value(scope, name)
			if !ok {
				continue
			}
			winner, _ := set.Lookup(name)
			summary.Variables = append(summary.Variables, scopedValue{
				Name:     name,
				Value:    displayValue(value),
				Scope:    scope.String(),
				Kind:     string(value.Kind),
				Secret:   value.Secret,
				Shadowed: winner.Scope != scope,
			})
		}
	}

	if c.DotEnv != nil {
		for _, v := range c.DotEnv.Vars {
			summary.DotEnv = append(summary.DotEnv, v.Name)
		}
	}

	for _, warning := range c.Warnings {
		summary.Warnings = append(summary.Warnings, warning.Error())
	}
	return summary
}

// writeVars prints the layers from lowest to highest priority, which is the
// order a reader has to follow to see why a value wins.
func writeVars(w io.Writer, c *collection.Collection, node *collection.Node, env *collection.Environment, set *vars.Set) {
	title := c.Config.Name
	if title == "" {
		title = c.Root
	}
	fmt.Fprintf(w, "%s\n", title)
	if node.RelPath != "" {
		fmt.Fprintf(w, "request: %s\n", node.RelPath)
	}
	if env != nil {
		fmt.Fprintf(w, "environment: %s\n", env.Name)
	}

	for _, scope := range vars.Precedence() {
		names := set.Names(scope)
		if len(names) == 0 {
			continue
		}
		fmt.Fprintf(w, "\n%s (%d)\n", scope, len(names))
		for _, name := range names {
			value, _ := set.Value(scope, name)
			winner, _ := set.Lookup(name)
			line := fmt.Sprintf("  %s = %s", name, displayValue(value))
			if winner.Scope != scope {
				line += fmt.Sprintf(" (shadowed by %s)", winner.Scope)
			}
			fmt.Fprintln(w, line)
		}
	}

	if c.DotEnv != nil {
		fmt.Fprintf(w, "\nprocess.env (%d from .env)\n", len(c.DotEnv.Vars))
		for _, v := range c.DotEnv.Vars {
			// The value is reachable with `cat .env` when it is needed; printing
			// it here would put credentials into shell history and CI logs.
			fmt.Fprintf(w, "  %s\n", v.Name)
		}
	}
}

// displayValue masks a secret. A secret has no value on disk, so this also keeps
// an empty line from looking like an unset variable.
func displayValue(v vars.Value) string {
	if v.Secret {
		return maskedValue
	}
	return v.Text
}
