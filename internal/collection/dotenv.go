package collection

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DotEnvFile is the name of the dotenv file a collection may carry.
//
// Only this exact name is read. Bruno treats `.env.*` files (`.env.local`,
// `.env.production`) as inert, so a collection cannot accidentally pick up a
// developer's local overrides.
const DotEnvFile = ".env"

// DotEnvVar is one KEY=VALUE entry from a dotenv file.
type DotEnvVar struct {
	Name  string
	Value string
}

// DotEnv is the content of a collection's `.env` file.
//
// The variables are kept in file order and are never merged into the normal
// collection/folder/request scopes: they are only reachable through the
// `process.env` namespace.
type DotEnv struct {
	// Path is the file the variables were read from.
	Path string
	// Vars are the entries in file order, duplicates collapsed.
	Vars []DotEnvVar
	// Warnings records lines that could not be parsed. A dotenv file is
	// advisory, so a malformed line is skipped instead of failing the load;
	// Load copies these into Collection.Warnings so a consumer reads one list.
	Warnings []error
}

// Map returns the variables as a name-to-value map.
func (d *DotEnv) Map() map[string]string {
	if d == nil || len(d.Vars) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(d.Vars))
	for _, v := range d.Vars {
		out[v.Name] = v.Value
	}
	return out
}

// Lookup returns a value by name. A nil DotEnv has no variables.
func (d *DotEnv) Lookup(name string) (string, bool) {
	if d == nil {
		return "", false
	}
	for _, v := range d.Vars {
		if v.Name == name {
			return v.Value, true
		}
	}
	return "", false
}

// LoadDotEnv reads root/.env. A missing file is not an error and yields nil.
//
// The file is read whole, like Bruno does: the 2.5 MiB request-file cap in
// ARCHITECTURE §7 exists to keep the watcher from parsing huge request files and
// does not apply to a dotenv file.
func LoadDotEnv(ctx context.Context, root string) (*DotEnv, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	path := filepath.Join(root, DotEnvFile)
	src, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	return ParseDotEnv(path, src), nil
}

// ParseDotEnv parses dotenv text the way Bruno does (npm `dotenv` v16):
//
//   - `KEY=value`, `KEY: value` and `export KEY=value` are accepted;
//   - keys may contain ASCII letters, digits, `_`, `.` and `-`;
//   - blank lines and lines whose first non-space character is `#` are ignored;
//   - an unquoted value ends at `#` and is trimmed of whitespace;
//   - a value in `'`, `"` or backticks keeps its whitespace, and a quoted value
//     may span lines. A closing quote must be the last thing on its line apart
//     from spaces and a comment, which is what dotenv's pattern requires;
//   - `\n` and `\r` are expanded inside double quotes only;
//   - `KEY=` sets an empty string, and a repeated key keeps its first position
//     with the last value.
//
// Two behaviours are apigo extensions: a line that does not parse becomes a
// warning instead of being dropped silently, and so does an unterminated quote.
// In both cases the line keeps dotenv's fallback value, so the text is not lost.
func ParseDotEnv(path string, src []byte) *DotEnv {
	text := strings.TrimPrefix(string(src), "\ufeff")
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	s := &dotenvScanner{path: path, src: text, line: 1, env: &DotEnv{Path: path}}
	s.scan()
	return s.env
}

// dotenvScanner walks dotenv text once. Values may contain newlines, so a
// line-based split would need lookahead; a cursor is simpler.
type dotenvScanner struct {
	path  string
	src   string
	off   int
	line  int
	env   *DotEnv
	index map[string]int
}

func (s *dotenvScanner) scan() {
	for s.off < len(s.src) {
		s.skipSpaces()
		if s.off >= len(s.src) {
			return
		}
		switch s.cur() {
		case '\n':
			s.newline()
			continue
		case '#':
			s.skipLine()
			continue
		}

		startLine := s.line
		key, ok := s.readKey()
		if !ok {
			s.warn(startLine, "expected KEY=VALUE")
			s.skipLine()
			continue
		}

		separated := s.consumeSeparator()
		if !separated {
			s.warn(startLine, fmt.Sprintf("%q has no '=' or ': ' after it", key))
			s.skipLine()
			continue
		}
		s.skipSpaces()

		value, closed := s.readValue()
		if !closed {
			s.warn(startLine, fmt.Sprintf("%q has no closing quote; kept the rest of the line", key))
		}
		s.skipLine()
		s.set(key, value)
	}
}

// set records a variable. A duplicate key keeps its first position and takes the
// last value, matching how Bruno's parser collapses the file into an object.
func (s *dotenvScanner) set(key, value string) {
	if s.index == nil {
		s.index = make(map[string]int, len(s.env.Vars))
	}
	if i, ok := s.index[key]; ok {
		s.env.Vars[i].Value = value
		return
	}
	s.index[key] = len(s.env.Vars)
	s.env.Vars = append(s.env.Vars, DotEnvVar{Name: key, Value: value})
}

func (s *dotenvScanner) warn(line int, msg string) {
	s.env.Warnings = append(s.env.Warnings, fmt.Errorf("%s:%d: dotenv: %s", s.path, line, msg))
}

func (s *dotenvScanner) cur() byte { return s.src[s.off] }

// newline consumes one line break and advances the line counter.
func (s *dotenvScanner) newline() {
	s.off++
	s.line++
}

func (s *dotenvScanner) skipSpaces() {
	for s.off < len(s.src) && (s.src[s.off] == ' ' || s.src[s.off] == '\t') {
		s.off++
	}
}

// skipLine consumes the rest of the current line, including its newline.
func (s *dotenvScanner) skipLine() {
	for s.off < len(s.src) {
		if s.src[s.off] == '\n' {
			s.newline()
			return
		}
		s.off++
	}
}

// readKey reads an optional `export ` prefix and the key itself.
func (s *dotenvScanner) readKey() (string, bool) {
	s.skipExport()
	start := s.off
	for s.off < len(s.src) && isDotEnvKeyByte(s.src[s.off]) {
		s.off++
	}
	if s.off == start {
		return "", false
	}
	return s.src[start:s.off], true
}

// skipExport consumes an `export` keyword and the whitespace after it. The
// keyword must be followed by whitespace, so a key called `exported` is not
// mistaken for it.
func (s *dotenvScanner) skipExport() {
	const keyword = "export"
	if !strings.HasPrefix(s.src[s.off:], keyword) {
		return
	}
	end := s.off + len(keyword)
	if end >= len(s.src) || (s.src[end] != ' ' && s.src[end] != '\t') {
		return
	}
	for end < len(s.src) && (s.src[end] == ' ' || s.src[end] == '\t') {
		end++
	}
	s.off = end
}

// consumeSeparator consumes `=` or `: ` and reports whether one was there.
//
// dotenv allows spaces before `=`, but a colon must follow the key directly and
// be followed by whitespace: `KEY: value` is an assignment, `KEY : value` and
// `KEY:value` are not.
func (s *dotenvScanner) consumeSeparator() bool {
	switch {
	case s.off < len(s.src) && s.cur() == '=':
		s.off++
		return true
	case strings.HasPrefix(s.src[s.off:], ":"):
		if s.off+1 < len(s.src) && (s.src[s.off+1] == ' ' || s.src[s.off+1] == '\t') {
			s.off++
			return true
		}
	}
	// `KEY = value`: the space was already consumed, so look for `=` here.
	s.skipSpaces()
	if s.off < len(s.src) && s.cur() == '=' {
		s.off++
		return true
	}
	return false
}

// readValue reads one value and reports whether a quoted value was closed.
//
// A quoted value that is not closed, or that is followed by text other than a
// comment, falls back to dotenv's unquoted rule: the rest of the line, quotes
// included. That keeps the text instead of silently dropping the file's tail.
func (s *dotenvScanner) readValue() (value string, closed bool) {
	if s.off >= len(s.src) || s.cur() == '\n' {
		return "", true
	}

	start := s.off
	quote := s.cur()
	if quote != '\'' && quote != '"' && quote != '`' {
		return strings.TrimSpace(s.takeToValueEnd()), true
	}

	end := s.closingQuote(quote, start+1)
	if end < 0 {
		return strings.TrimSpace(s.takeToValueEnd()), false
	}
	if !s.endsAfterQuote(end + 1) {
		return strings.TrimSpace(s.takeToValueEnd()), false
	}

	value = s.src[start+1 : end]
	if quote == '"' {
		value = expandDotEnvEscapes(value)
	}
	s.line += strings.Count(s.src[start:end], "\n")
	s.off = end + 1
	return value, true
}

// takeToValueEnd consumes an unquoted value, stopping at `#` or the line end.
func (s *dotenvScanner) takeToValueEnd() string {
	start := s.off
	for s.off < len(s.src) && s.cur() != '\n' && s.cur() != '#' {
		s.off++
	}
	return s.src[start:s.off]
}

// closingQuote finds the quote that ends a value opened at from.
//
// The rule is the one dotenv's pattern enforces after backtracking: a closing
// quote is followed by nothing but spaces and an optional comment. That is why
// `'C:\'` closes on its final quote (a trailing backslash is literal), while
// `'a\'b'` skips the escaped quote and closes on the last one.
func (s *dotenvScanner) closingQuote(quote byte, from int) int {
	for i := from; i < len(s.src); i++ {
		if s.src[i] == quote && s.endsAfterQuote(i+1) {
			return i
		}
	}
	return -1
}

// endsAfterQuote reports whether only spaces and an optional comment follow a
// closing quote on its line, which dotenv requires.
func (s *dotenvScanner) endsAfterQuote(from int) bool {
	rest := s.src[from:]
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[:i]
	}
	rest = strings.TrimSpace(rest)
	return rest == "" || strings.HasPrefix(rest, "#")
}

// expandDotEnvEscapes applies the two escapes npm dotenv expands inside double
// quotes. Everything else (`\\`, `\"`, `\t`) stays literal, as it does there.
func expandDotEnvEscapes(text string) string {
	if !strings.Contains(text, "\\") {
		return text
	}
	text = strings.ReplaceAll(text, `\n`, "\n")
	return strings.ReplaceAll(text, `\r`, "\r")
}

func isDotEnvKeyByte(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z', c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		return true
	case c == '_' || c == '.' || c == '-':
		return true
	}
	return false
}

// ParseOSEnviron turns os.Environ-style "K=V" entries into a map.
func ParseOSEnviron(environ []string) map[string]string {
	out := make(map[string]string, len(environ))
	for _, entry := range environ {
		key, value, ok := strings.Cut(entry, "=")
		if !ok || key == "" {
			continue
		}
		out[key] = value
	}
	return out
}

// BuildProcessEnv merges the `process.env` layers. Later layers win, matching
// Bruno's priority: OS environment, then workspace `.env`, then collection
// `.env`.
func BuildProcessEnv(osEnviron []string, layers ...*DotEnv) map[string]string {
	out := ParseOSEnviron(osEnviron)
	for _, layer := range layers {
		if layer == nil {
			continue
		}
		for _, v := range layer.Vars {
			out[v.Name] = v.Value
		}
	}
	return out
}

// ProcessEnv returns the `process.env` namespace for this collection. Pass the
// workspace `.env` as workspace when the desktop app opened the collection
// inside one; it has lower priority than the collection's own file.
//
// This is the seam the request engine uses in M2; nothing else calls it yet.
func (c *Collection) ProcessEnv(osEnviron []string, workspace *DotEnv) map[string]string {
	return BuildProcessEnv(osEnviron, workspace, c.DotEnv)
}
