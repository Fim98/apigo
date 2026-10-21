package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"apigo/internal/bru"
	"apigo/internal/collection"
)

// fmtOptions holds the flags of the fmt command.
type fmtOptions struct {
	check  bool
	stdout bool
}

// runFmt rewrites .bru files in canonical form.
//
// Exit codes: 0 when everything is already canonical or was rewritten, 1 when
// --check found a file that would change, 2 for usage or I/O errors.
func runFmt(args []string, stdout, stderr io.Writer) int {
	var opts fmtOptions
	var paths []string
	for _, arg := range args {
		switch arg {
		case "--check", "-c":
			opts.check = true
		case "--stdout", "-s":
			opts.stdout = true
		case "--help", "-h":
			fmt.Fprint(stdout, fmtUsage)
			return 0
		default:
			if strings.HasPrefix(arg, "-") {
				fmt.Fprintf(stderr, "apigo fmt: unknown flag %q\n\n%s", arg, fmtUsage)
				return 2
			}
			paths = append(paths, arg)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintf(stderr, "apigo fmt: missing path\n\n%s", fmtUsage)
		return 2
	}
	if opts.stdout && len(paths) > 1 {
		fmt.Fprintf(stderr, "apigo fmt: --stdout accepts one file\n")
		return 2
	}

	files, code := collectFiles(paths, stderr)
	if code != 0 {
		return code
	}

	changed := 0
	for _, path := range files {
		formatted, err := formatFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "apigo fmt: %v\n", err)
			return 2
		}
		original, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintf(stderr, "apigo fmt: %v\n", err)
			return 2
		}
		if string(original) == formatted {
			continue
		}
		changed++

		switch {
		case opts.stdout:
			fmt.Fprint(stdout, formatted)
		case opts.check:
			fmt.Fprintln(stdout, path)
		default:
			if err := writeFileAtomic(path, []byte(formatted)); err != nil {
				fmt.Fprintf(stderr, "apigo fmt: %v\n", err)
				return 2
			}
			fmt.Fprintln(stdout, path)
		}
	}

	if opts.check && changed > 0 {
		return 1
	}
	return 0
}

// formatFile parses a file and returns its canonical form. Environment files,
// recognised by living in an environments directory, use the env grammar; `.yml`
// and `.yaml` files use the OpenCollection writer.
func formatFile(path string) (string, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	yamlFile := strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml")

	if isEnvFile(path) {
		if yamlFile {
			env, err := collection.ParseYAMLEnvironment(path, envName(path), src)
			if err != nil {
				return "", err
			}
			out, err := collection.MarshalYAMLEnvironment(env)
			return string(out), err
		}
		env, err := bru.ParseEnv(path, src)
		if err != nil {
			return "", err
		}
		return string(bru.FormatEnv(env)), nil
	}
	if yamlFile {
		if !isOpenCollectionYAML(path) {
			return "", fmt.Errorf("%s: not an OpenCollection file (no %s in its directory tree)", path, collection.OpenCollectionFile)
		}
		return formatYAMLFile(path, src)
	}
	file, err := bru.Parse(path, src)
	if err != nil {
		return "", err
	}
	return bru.FormatString(file), nil
}

// formatYAMLFile canonicalises an OpenCollection file by kind.
func formatYAMLFile(path string, src []byte) (string, error) {
	switch filepath.Base(path) {
	case collection.OpenCollectionFile, collection.OpenCollectionAltFile:
		cfg, defaults, err := collection.ParseYAMLCollection(path, src)
		if err != nil {
			return "", err
		}
		c := &collection.Collection{Config: cfg, Defaults: defaults}
		if defaults != nil {
			c.Source = defaults.Source
		}
		out, err := collection.MarshalYAMLCollection(c)
		return string(out), err
	case collection.YAMLFolderFile, collection.YAMLFolderAltFile:
		folder, err := collection.ParseYAMLFolder(path, filepath.Base(path), src)
		if err != nil {
			return "", err
		}
		out, err := collection.MarshalYAMLFolder(folder)
		return string(out), err
	default:
		req, err := collection.ParseYAMLRequest(path, filepath.Base(path), src)
		if err != nil {
			return "", err
		}
		out, err := collection.MarshalYAMLRequest(req)
		return string(out), err
	}
}

// envName is the environment's display name, derived from the file name.
func envName(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".bru")
	base = strings.TrimSuffix(base, ".yml")
	base = strings.TrimSuffix(base, ".yaml")
	return base
}

// isEnvFile reports whether path is an environment file.
func isEnvFile(path string) bool {
	return filepath.Base(filepath.Dir(path)) == "environments"
}

// collectFiles expands files and directories into the .bru files to format.
func collectFiles(paths []string, stderr io.Writer) ([]string, int) {
	var files []string
	for _, root := range paths {
		info, err := os.Stat(root)
		if err != nil {
			fmt.Fprintf(stderr, "apigo fmt: %v\n", err)
			return nil, 2
		}
		if !info.IsDir() {
			files = append(files, root)
			continue
		}
		err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			name := entry.Name()
			if entry.IsDir() {
				if path != root && (strings.HasPrefix(name, ".") || name == "node_modules") {
					return filepath.SkipDir
				}
				return nil
			}
			switch {
			case strings.HasSuffix(name, ".bru"):
				files = append(files, path)
			case (strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")) && isOpenCollectionYAML(path):
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(stderr, "apigo fmt: %v\n", err)
			return nil, 2
		}
	}
	if len(files) == 0 {
		fmt.Fprintf(stderr, "apigo fmt: no .bru files or OpenCollection YAML found\n")
		return nil, 2
	}
	return files, 0
}

// isOpenCollectionYAML reports whether a YAML file belongs to an OpenCollection:
// it is the collection file itself, or a file with opencollection.yml somewhere
// above it. Anything else (compose files, CI config) is left alone.
func isOpenCollectionYAML(path string) bool {
	base := filepath.Base(path)
	if base == collection.OpenCollectionFile || base == collection.OpenCollectionAltFile {
		return true
	}
	dir := filepath.Dir(path)
	for range 64 {
		for _, name := range []string{collection.OpenCollectionFile, collection.OpenCollectionAltFile} {
			if info, err := os.Stat(filepath.Join(dir, name)); err == nil && !info.IsDir() {
				return true
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return false
		}
		dir = parent
	}
	return false
}

// writeFileAtomic replaces path with data without ever leaving a partial file.
func writeFileAtomic(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".apigo-fmt-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer func() {
		// Best effort: CreateTemp files are removed by rename on success.
		_ = os.Remove(tmpName)
	}()

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, info.Mode().Perm()); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		return errors.Join(err, os.Remove(tmpName))
	}
	return nil
}

const fmtUsage = `apigo fmt - rewrite .bru files in canonical form

Usage:
  apigo fmt [--check] [--stdout] <path>...

Flags:
  -c, --check    list files that would change; exit 1 if any
  -s, --stdout   print the formatted file instead of writing it

Paths may be .bru, .yml or .yaml files, or directories (searched
recursively, skipping hidden directories and node_modules). OpenCollection
files are canonicalised by the YAML writer.
`
