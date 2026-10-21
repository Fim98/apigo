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

	c := &Collection{Root: abs}

	if src, err := os.ReadFile(filepath.Join(abs, ConfigFile)); err == nil {
		cfg, err := ParseConfig(filepath.Join(abs, ConfigFile), src)
		if err != nil {
			c.Warnings = append(c.Warnings, err)
		} else {
			c.Config = cfg
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if src, err := os.ReadFile(filepath.Join(abs, CollectionFile)); err == nil {
		f, err := bru.Parse(filepath.Join(abs, CollectionFile), src)
		if err != nil {
			c.Warnings = append(c.Warnings, err)
		}
		if f != nil {
			defaults := FolderFromFile(f)
			defaults.Path = filepath.Join(abs, CollectionFile)
			c.Defaults = defaults
		}
	} else if !errors.Is(err, os.ErrNotExist) {
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

	envs, err := LoadEnvironments(ctx, abs)
	if err != nil {
		c.Warnings = append(c.Warnings, err)
	}
	c.Environments = envs

	return c, nil
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
			if src, err := os.ReadFile(filepath.Join(folder.Path, FolderFile)); err == nil {
				if f, perr := bru.Parse(filepath.Join(folder.Path, FolderFile), src); perr == nil {
					parsed := FolderFromFile(f)
					parsed.Path = folder.Path
					parsed.RelPath = childRel
					if parsed.Meta.Name == "" {
						parsed.Meta.Name = name
					}
					folder = parsed
				} else {
					c.Warnings = append(c.Warnings, perr)
				}
			} else if !errors.Is(err, os.ErrNotExist) {
				c.Warnings = append(c.Warnings, err)
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

		if !strings.HasSuffix(name, ".bru") || name == CollectionFile || name == FolderFile {
			continue
		}
		req, err := loadRequest(filepath.Join(root, filepath.FromSlash(childRel)), childRel)
		if err != nil {
			c.Warnings = append(c.Warnings, err)
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
func loadRequest(path, rel string) (*Request, error) {
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
	req, err := ParseRequest(path, rel, src)
	if err != nil {
		return nil, err
	}
	return req, nil
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
func LoadRequestFile(path string) (*Request, error) {
	req, err := loadRequest(path, filepath.Base(path))
	if err != nil {
		return nil, err
	}
	return req, nil
}
