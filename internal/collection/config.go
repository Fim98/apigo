package collection

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Config is a collection's bruno.json.
//
// Unknown keys are preserved as raw JSON so a load/save cycle does not drop
// configuration written by a newer Bruno. Marshalled output is deterministic:
// known keys first in Bruno's order, then unknown keys sorted by name.
type Config struct {
	Version string
	Name    string
	Type    string
	Ignore  []string
	// Raw holds every key of the source document, including the known ones.
	Raw map[string]json.RawMessage
}

// ParseConfig decodes bruno.json.
func ParseConfig(path string, src []byte) (Config, error) {
	cfg := Config{Raw: map[string]json.RawMessage{}}
	if len(bytes.TrimSpace(src)) == 0 {
		return cfg, nil
	}
	if err := json.Unmarshal(src, &cfg.Raw); err != nil {
		return cfg, &ConfigError{Path: path, Err: err}
	}
	if raw, ok := cfg.Raw["version"]; ok {
		_ = json.Unmarshal(raw, &cfg.Version)
	}
	if raw, ok := cfg.Raw["name"]; ok {
		_ = json.Unmarshal(raw, &cfg.Name)
	}
	if raw, ok := cfg.Raw["type"]; ok {
		_ = json.Unmarshal(raw, &cfg.Type)
	}
	if raw, ok := cfg.Raw["ignore"]; ok {
		_ = json.Unmarshal(raw, &cfg.Ignore)
	}
	return cfg, nil
}

// Marshal renders bruno.json.
func (c Config) Marshal() ([]byte, error) {
	doc := map[string]any{}
	for key, raw := range c.Raw {
		doc[key] = raw
	}
	set := func(key string, value any) {
		if raw, err := json.Marshal(value); err == nil {
			doc[key] = json.RawMessage(raw)
		}
	}
	if c.Version != "" {
		set("version", c.Version)
	}
	if c.Name != "" {
		set("name", c.Name)
	}
	if c.Type != "" {
		set("type", c.Type)
	}
	if c.Ignore != nil {
		set("ignore", c.Ignore)
	}

	// Emit known keys in a stable order, then the rest alphabetically.
	order := []string{"version", "name", "type", "ignore"}
	var rest []string
	for key := range doc {
		known := false
		for _, k := range order {
			if k == key {
				known = true
				break
			}
		}
		if !known {
			rest = append(rest, key)
		}
	}
	sort.Strings(rest)
	order = append(order, rest...)

	var buf bytes.Buffer
	buf.WriteString("{\n")
	first := true
	for _, key := range order {
		raw, ok := doc[key]
		if !ok {
			continue
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, raw.(json.RawMessage)); err != nil {
			continue
		}
		if !first {
			buf.WriteString(",\n")
		}
		first = false
		keyJSON, _ := json.Marshal(key)
		buf.WriteString("  ")
		buf.Write(keyJSON)
		buf.WriteString(": ")
		buf.Write(compact.Bytes())
	}
	buf.WriteString("\n}\n")
	return buf.Bytes(), nil
}
