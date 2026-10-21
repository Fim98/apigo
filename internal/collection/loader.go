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

// File names the loader treats specially.
const (
	ConfigFile     = "bruno.json"
	CollectionFile = "collection.bru"
	FolderFile     = "folder.bru"
	EnvDir         = "environments"
)

// Format identifies how a collection is stored on disk.
type Format uint8

// Collection formats.
const (
	// FormatBRU is `.bru` files with bruno.json.
	FormatBRU Format = iota
	// FormatYAML is OpenCollection YAML.
	FormatYAML
)

// String returns the format name.
func (f Format) String() string {
	if f == FormatYAML {
		return "yaml"
	}
	return "bru"
}

// skippedDirs are never walked. Hidden directories are skipped as well.
var skippedDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	EnvDir:         true,
}

// Load reads a collection from a directory.
//
// Broken or oversized files do not fail the load: they are recorded in
// Collection.Warnings and skipped, so one bad file cannot make a collection
// unopenable.
func Load(ctx context.Context, root string) (*Collection, error) {
	info, err := os.Stat(root)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: %s", ErrNotFound, root)
		}
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%w: %s is not a directory", ErrNotFound, root)
	}

	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	c := &Collection{Root: abs, Format: DetectFormat(abs)}
	if err := c.loadConfig(ctx, abs); err != nil {
		return nil, err
	}

	ignore := map[string]bool{}
	for _, name := range c.Config.Ignore {
		ignore[name] = true
	}

	tree, err := loadDir(ctx, abs, "", nil, ignore, c)
	if err != nil {
		return nil, err
	}
	// A synthetic root carries collection.bru so inheritance walks reach the
	// collection defaults.
	rootNode := &Node{Path: abs, Folder: c.Defaults}
	if c.Defaults != nil {
		rootNode.Name = c.Defaults.Meta.Name
	}
	if rootNode.Name == "" {
		rootNode.Name = c.Config.Name
	}
	for _, node := range tree {
		node.Parent = rootNode
	}
	rootNode.Children = tree
	c.root = rootNode
	c.Tree = tree

	envs, err := loadEnvironments(ctx, abs, c.Format)
	if err != nil {
		c.Warnings = append(c.Warnings, err)
	}
	c.Environments = envs

	return c, nil
}

// loadConfig reads the collection-level files for the collection's format:
// opencollection.yml, or bruno.json plus collection.bru.
func (c *Collection) loadConfig(ctx context.Context, abs string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if c.Format == FormatYAML {
		cfgPath, ok := openCollectionPath(abs)
		if !ok {
			return fmt.Errorf("%w: %s has no %s", ErrNotFound, abs, OpenCollectionFile)
		}
		src, err := os.ReadFile(cfgPath)
		if err != nil {
			return err
		}
		cfg, defaults, err := ParseYAMLCollection(cfgPath, src)
		if err != nil {
			c.Warnings = append(c.Warnings, err)
			return nil
		}
		c.Config = cfg
		c.Defaults = defaults
		c.Source = defaults.Source
		if _, statErr := os.Stat(filepath.Join(abs, ConfigFile)); statErr == nil {
			c.Warnings = append(c.Warnings, fmt.Errorf("%s: %s and %s both exist; reading the OpenCollection YAML format", abs, OpenCollectionFile, ConfigFile))
		}
		return nil
	}

	path := filepath.Join(abs, ConfigFile)
	src, err := os.ReadFile(path)
	switch {
	case err == nil:
		cfg, cfgErr := ParseConfig(path, src)
		if cfgErr != nil {
			c.Warnings = append(c.Warnings, cfgErr)
		} else {
			c.Config = cfg
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}

	collectionPath := filepath.Join(abs, CollectionFile)
	src, err = os.ReadFile(collectionPath)
	switch {
	case err == nil:
		f, parseErr := bru.Parse(collectionPath, src)
		if parseErr != nil {
			c.Warnings = append(c.Warnings, parseErr)
		}
		if f != nil {
			defaults := FolderFromFile(f)
			defaults.Path = collectionPath
			c.Defaults = defaults
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	return nil
}

// loadDir reads one directory's entries, recursively.
func loadDir(ctx context.Context, root, rel string, parent *Node, ignore map[string]bool, c *Collection) ([]*Node, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, filepath.FromSlash(rel))
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var nodes []*Node
	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() && (skippedDirs[name] || ignore[name]) {
			continue
		}
		childRel := name
		if rel != "" {
			childRel = rel + "/" + name
		}

		if entry.IsDir() {
			folder := &Folder{Meta: Meta{Name: name, Type: string(TypeHTTP), Seq: 1}}
			folder.Path = filepath.Join(root, filepath.FromSlash(childRel))
			folder.RelPath = childRel
			if parsed, perr := loadFolderMeta(ctx, folder, c.Format); perr != nil {
				c.Warnings = append(c.Warnings, perr)
			} else if parsed != nil {
				parsed.Meta.Name = orDefault(parsed.Meta.Name, name)
				folder = parsed
			}

			node := &Node{Name: folder.Meta.Name, Path: folder.Path, RelPath: childRel, Folder: folder, Parent: parent}
			children, err := loadDir(ctx, root, childRel, node, ignore, c)
			if err != nil {
				return nil, err
			}
			node.Children = children
			nodes = append(nodes, node)
			continue
		}

		if !isRequestFile(name, c.Format) {
			continue
		}
		req, err := loadRequest(ctx, filepath.Join(root, filepath.FromSlash(childRel)), childRel, c.Format)
		if err != nil {
			c.Warnings = append(c.Warnings, err)
			continue
		}
		if c.Format == FormatYAML && !IsYAMLRequestType(req.Meta.Type) {
			// `script`, `app` and other item types are preserved on disk but are
			// not part of the request tree.
			c.Warnings = append(c.Warnings, fmt.Errorf("%s: %q items are not shown as requests yet", req.RelPath, req.Meta.Type))
			continue
		}
		nodes = append(nodes, &Node{
			Name:    req.Name(),
			Path:    req.Path,
			RelPath: childRel,
			Request: req,
			Parent:  parent,
		})
	}

	sortNodes(nodes)
	return nodes, nil
}

// loadRequest reads one request file, enforcing the size cap.
func loadRequest(ctx context.Context, path, rel string, format Format) (*Request, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > MaxFileSize {
		return nil, fmt.Errorf("%w: %s", ErrTooLarge, path)
	}
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if format == FormatYAML {
		return ParseYAMLRequest(path, rel, src)
	}
	return ParseRequest(path, rel, src)
}

// loadFolderMeta reads folder.bru, folder.yml or nothing.
func loadFolderMeta(ctx context.Context, base *Folder, format Format) (*Folder, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if format == FormatYAML {
		path, ok := yamlFolderPath(base.Path)
		if !ok {
			return nil, nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return ParseYAMLFolder(path, base.RelPath, src)
	}

	path := filepath.Join(base.Path, FolderFile)
	src, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	f, err := bru.Parse(path, src)
	if err != nil {
		return nil, err
	}
	folder := FolderFromFile(f)
	folder.Path = base.Path
	folder.RelPath = base.RelPath
	return folder, nil
}

// isRequestFile reports whether a directory entry is a request in the collection
// format.
func isRequestFile(name string, format Format) bool {
	if format == FormatYAML {
		switch name {
		case OpenCollectionFile, OpenCollectionAltFile, YAMLFolderFile, YAMLFolderAltFile:
			return false
		}
		return strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml")
	}
	return strings.HasSuffix(name, ".bru") && name != CollectionFile && name != FolderFile
}

// orDefault returns value when non-empty, otherwise def.
func orDefault(value, def string) string {
	if value != "" {
		return value
	}
	return def
}

// sortNodes orders entries by seq then name, folders and requests together, the
// way Bruno's sidebar does.
func sortNodes(nodes []*Node) {
	sort.SliceStable(nodes, func(i, j int) bool {
		si, sj := nodeSeq(nodes[i]), nodeSeq(nodes[j])
		if si != sj {
			return si < sj
		}
		return strings.ToLower(nodes[i].Name) < strings.ToLower(nodes[j].Name)
	})
}

func nodeSeq(n *Node) int {
	switch {
	case n.Folder != nil && n.Folder.Meta.Seq != 0:
		return n.Folder.Meta.Seq
	case n.Request != nil && n.Request.Meta.Seq != 0:
		return n.Request.Meta.Seq
	default:
		return 1
	}
}

// LoadRequestFile loads a single request file outside a collection.
func LoadRequestFile(ctx context.Context, path string) (*Request, error) {
	format := FormatBRU
	if strings.HasSuffix(path, ".yml") || strings.HasSuffix(path, ".yaml") {
		format = FormatYAML
	}
	return loadRequest(ctx, path, filepath.Base(path), format)
}
