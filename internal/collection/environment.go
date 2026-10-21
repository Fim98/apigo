package collection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"apigo/internal/bru"
)

// Environment is one file under environments/.
//
// Secret variables carry their names only; the values live in the OS keychain
// (desktop) or are injected at run time (CLI), so they are never read from or
// written to the file.
type Environment struct {
	Name string
	Path string

	Vars        []Variable
	SecretNames []string
	Extends     []string
	Color       string

	// External holds vars:externalsecrets:<provider> blocks. They are preserved
	// but not resolved until a provider is implemented.
	External []ExternalSecretProvider
}

// ExternalSecretProvider is a preserved external secret block.
type ExternalSecretProvider struct {
	Provider string
	Pairs    []KeyValue
}

// ParseEnvironment parses an environment file.
func ParseEnvironment(path, name string, src []byte) (*Environment, error) {
	env, err := bru.ParseEnv(path, src)
	if err != nil {
		return nil, err
	}
	e := &Environment{
		Name:        name,
		Path:        path,
		Vars:        variablesFromPairs(env.Variables),
		SecretNames: append([]string(nil), env.Secrets...),
		Extends:     append([]string(nil), env.Extends...),
		Color:       env.Color,
	}
	for _, ext := range env.External {
		e.External = append(e.External, ExternalSecretProvider{
			Provider: ext.Provider,
			Pairs:    keyValuesFromPairs(ext.Pairs),
		})
	}
	e.ApplySecrets()
	return e, nil
}

// ApplySecrets marks variables named in vars:secret as secret and clears their
// values, which must never live in the file.
func (e *Environment) ApplySecrets() {
	if len(e.SecretNames) == 0 {
		return
	}
	names := make(map[string]bool, len(e.SecretNames))
	for _, name := range e.SecretNames {
		names[name] = true
	}
	for i := range e.Vars {
		if names[e.Vars[i].Name] {
			e.Vars[i].Secret = true
			e.Vars[i].Value = ""
		}
	}
	for _, name := range e.SecretNames {
		found := false
		for _, v := range e.Vars {
			if v.Name == name {
				found = true
				break
			}
		}
		if !found {
			// A secret declared without a value row still exists.
			e.Vars = append(e.Vars, Variable{Name: name, Enabled: true, Secret: true, Kind: VarString})
		}
	}
}

// LoadEnvironments reads every environment file in root/environments.
func LoadEnvironments(ctx context.Context, root string) ([]*Environment, error) {
	dir := filepath.Join(root, EnvDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var envs []*Environment
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".bru") || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		env, err := ParseEnvironment(path, strings.TrimSuffix(name, ".bru"), src)
		if err != nil {
			return nil, err
		}
		envs = append(envs, env)
	}
	sort.SliceStable(envs, func(i, j int) bool { return envs[i].Name < envs[j].Name })
	return envs, nil
}

// ResolveEnvironment flattens an environment and its extends chain into a single
// variable list. Parents are applied before the environment itself, cycles are
// reported instead of hanging, and a secret declaration wins over a non-secret
// one with the same name.
func ResolveEnvironment(env *Environment, byName map[string]*Environment) ([]Variable, error) {
	var merged []Variable
	index := map[string]int{}
	visiting := map[string]bool{}
	visited := map[string]bool{}

	var apply func(e *Environment, path []string) error
	apply = func(e *Environment, path []string) error {
		if visited[e.Name] {
			return nil
		}
		if visiting[e.Name] {
			return fmt.Errorf("environment %q extends itself: %s", e.Name, strings.Join(append(path, e.Name), " -> "))
		}
		visiting[e.Name] = true
		defer func() {
			visiting[e.Name] = false
			visited[e.Name] = true
		}()

		for _, parentName := range e.Extends {
			parent, ok := byName[parentName]
			if !ok {
				continue
			}
			if err := apply(parent, append(path, e.Name)); err != nil {
				return err
			}
		}

		for _, v := range e.Vars {
			if i, ok := index[v.Name]; ok {
				if merged[i].Secret && !v.Secret {
					// A secret declaration wins over a plain variable.
					continue
				}
				merged[i] = v
				continue
			}
			index[v.Name] = len(merged)
			merged = append(merged, v)
		}
		return nil
	}

	if err := apply(env, nil); err != nil {
		return nil, err
	}
	return merged, nil
}
