package bru

import "strings"

// lexer scans a .bru source buffer on demand. The format is line oriented, so
// the parser sometimes asks for structural tokens (names, braces) and sometimes
// for raw line ranges (text blocks, list bodies). Both go through the same
// position bookkeeping, which keeps spans and error positions exact.
type lexer struct {
	path string
	src  []byte
	off  int
	line int
	col  int
}

func newLexer(path string, src []byte) *lexer {
	return &lexer{path: path, src: src, off: 0, line: 1, col: 1}
}

func (l *lexer) pos() Pos { return Pos{Offset: l.off, Line: l.line, Column: l.col} }

func (l *lexer) span(start Pos) Span { return Span{Start: start, End: l.pos()} }

func (l *lexer) eof() bool { return l.off >= len(l.src) }

// cur returns the current byte, or 0 at end of input.
func (l *lexer) cur() byte {
	if l.eof() {
		return 0
	}
	return l.src[l.off]
}

// peekByte returns the byte n positions ahead, or 0 past the end.
func (l *lexer) peekByte(n int) byte {
	if l.off+n >= len(l.src) {
		return 0
	}
	return l.src[l.off+n]
}

// hasPrefix reports whether the remaining input starts with s.
func (l *lexer) hasPrefix(s string) bool {
	if len(l.src)-l.off < len(s) {
		return false
	}
	for i := range len(s) {
		if l.src[l.off+i] != s[i] {
			return false
		}
	}
	return true
}

// advance moves forward n bytes, tracking line and column. A newline resets the
// column to one.
func (l *lexer) advance(n int) {
	for range n {
		if l.eof() {
			return
		}
		if l.src[l.off] == '\n' {
			l.line++
			l.col = 1
		} else {
			l.col++
		}
		l.off++
	}
}

func (l *lexer) mark() mark   { return mark{off: l.off, line: l.line, col: l.col} }
func (l *lexer) reset(m mark) { l.off, l.line, l.col = m.off, m.line, m.col }

// atNewline reports whether the scanner is on a line break.
func (l *lexer) atNewline() bool {
	return l.cur() == '\n' || (l.cur() == '\r' && l.peekByte(1) == '\n')
}

func (l *lexer) skipSpaces() { l.skipHorizontal() }

func (l *lexer) skipHorizontal() {
	for !l.eof() {
		switch {
		case l.cur() == ' ' || l.cur() == '\t':
			l.advance(1)
		case l.cur() == '\r' && l.peekByte(1) != '\n':
			l.advance(1)
		default:
			return
		}
	}
}

// skipNewline consumes one CRLF, LF or CR line break.
func (l *lexer) skipNewline() bool {
	switch {
	case l.cur() == '\r' && l.peekByte(1) == '\n':
		l.advance(2)
		return true
	case l.cur() == '\n' || l.cur() == '\r':
		l.advance(1)
		return true
	default:
		return false
	}
}

// skipBlankLines consumes blank and whitespace-only lines and leaves the
// scanner on the first non-blank line, past its indentation.
func (l *lexer) skipBlankLines() {
	for {
		l.skipHorizontal()
		if !l.skipNewline() {
			return
		}
	}
}

// next returns the next structural token.
func (l *lexer) next() Token {
	l.skipHorizontal()
	start := l.pos()
	if l.eof() {
		return Token{Kind: EOF, Span: Span{Start: start, End: start}}
	}

	switch c := l.cur(); c {
	case '\r', '\n':
		l.skipNewline()
		return Token{Kind: Newline, Text: "\n", Span: l.span(start)}
	case '{':
		l.advance(1)
		return Token{Kind: LBrace, Text: "{", Span: l.span(start)}
	case '}':
		l.advance(1)
		return Token{Kind: RBrace, Text: "}", Span: l.span(start)}
	case '[':
		l.advance(1)
		return Token{Kind: LBracket, Text: "[", Span: l.span(start)}
	case ']':
		l.advance(1)
		return Token{Kind: RBracket, Text: "]", Span: l.span(start)}
	case ':':
		l.advance(1)
		return Token{Kind: Colon, Text: ":", Span: l.span(start)}
	case ',':
		l.advance(1)
		return Token{Kind: Comma, Text: ",", Span: l.span(start)}
	case '"':
		text, _ := l.readQuoted()
		return Token{Kind: Quoted, Text: text, Span: l.span(start)}
	case '@':
		l.advance(1)
		return Token{Kind: At, Text: "@", Span: l.span(start)}
	case '\'':
		if l.hasPrefix("'''") {
			l.advance(3)
			return Token{Kind: TextBlockStart, Text: "'''", Span: l.span(start)}
		}
	}

	text := l.readBare()
	return Token{Kind: Name, Text: text, Span: l.span(start)}
}

// readBare reads a bare word: a block name, key or list item. It stops at
// whitespace, a line break, or one of the structural punctuation bytes.
func (l *lexer) readBare() string {
	start := l.off
	for !l.eof() {
		switch l.cur() {
		case ' ', '\t', '\r', '\n', '{', '}', '[', ']', ':', ',', '"':
			return string(l.src[start:l.off])
		}
		l.advance(1)
	}
	return string(l.src[start:l.off])
}

// readQuoted reads a "..." string and decodes \" escapes. A backslash before any
// other byte is kept verbatim, matching Bruno's quoted_key grammar.
func (l *lexer) readQuoted() (string, Span) {
	start := l.pos()
	l.advance(1) // opening quote
	var b strings.Builder
	for !l.eof() {
		c := l.cur()
		switch c {
		case '\\':
			if l.peekByte(1) == '"' {
				b.WriteByte('"')
				l.advance(2)
				continue
			}
			b.WriteByte(c)
			l.advance(1)
		case '"':
			l.advance(1)
			return b.String(), l.span(start)
		default:
			b.WriteByte(c)
			l.advance(1)
		}
	}
	return b.String(), l.span(start)
}

// readKeyRaw reads a key up to the next colon and returns it trimmed. Quoted keys
// are unquoted with escapes decoded; the returned disabled flag reports a
// leading '~' outside the quotes.
func (l *lexer) readKeyRaw() (key string, disabled bool, span Span) {
	l.skipHorizontal()
	start := l.pos()
	if l.cur() == '~' {
		disabled = true
		l.advance(1)
		l.skipHorizontal()
	}
	if l.cur() == '"' {
		key, _ = l.readQuoted()
		return key, disabled, l.span(start)
	}
	raw := l.readUntilColon()
	return strings.TrimSpace(raw), disabled, l.span(start)
}

// readUntilColon reads up to (not including) a colon or line break.
func (l *lexer) readUntilColon() string {
	start := l.off
	for !l.eof() {
		switch l.cur() {
		case ':', '\n':
			return string(l.src[start:l.off])
		case '\r':
			if l.peekByte(1) == '\n' {
				return string(l.src[start:l.off])
			}
			l.advance(1)
		default:
			l.advance(1)
		}
	}
	return string(l.src[start:l.off])
}

// readLine reads the rest of the current line without the line break and leaves
// the scanner on the break.
func (l *lexer) readLine() (string, Span) {
	start := l.pos()
	from := l.off
	for !l.eof() && !l.atNewline() {
		l.advance(1)
	}
	return string(l.src[from:l.off]), l.span(start)
}

// readValueLine reads a single-line value and trims it, matching Bruno's
// singlelinevalue semantics.
func (l *lexer) readValueLine() (string, Span) {
	text, span := l.readLine()
	return strings.TrimSpace(text), span
}

// readMultilineValue reads a ”' ... ”' value, removes four leading spaces from
// every line as Bruno does, and keeps a trailing @contentType(...) annotation.
func (l *lexer) readMultilineValue() (string, Span, error) {
	start := l.pos()
	if !l.hasPrefix("'''") {
		return "", Span{Start: start, End: start}, l.error(start, "expected '''")
	}
	l.advance(3)
	from := l.off
	for {
		if l.eof() {
			return "", l.span(start), l.error(start, "unterminated text block")
		}
		if l.hasPrefix("'''") {
			break
		}
		l.advance(1)
	}
	content := string(l.src[from:l.off])
	l.advance(3)
	text := stripFourSpaces(content)

	// A trailing @contentType(...) is part of the value in the file grammar.
	l.skipHorizontal()
	if l.hasPrefix("@contentType(") {
		l.advance(len("@contentType("))
		argFrom := l.off
		for !l.eof() && l.cur() != ')' && !l.atNewline() {
			l.advance(1)
		}
		arg := strings.TrimSpace(string(l.src[argFrom:l.off]))
		if l.cur() == ')' {
			l.advance(1)
		}
		if arg != "" {
			text += " @contentType(" + arg + ")"
		}
	}
	return strings.TrimSpace(text), l.span(start), nil
}

// stripFourSpaces removes up to four leading spaces from each line, which is the
// indentation Bruno's multiline value reader assumes.
func stripFourSpaces(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		line = strings.TrimSuffix(line, "\r")
		if len(line) >= 4 {
			line = line[4:]
		} else {
			line = ""
		}
		lines[i] = line
	}
	return strings.Join(lines, "\n")
}

// readTextBlock reads a text block body after the opening brace. It leaves the
// scanner on the closing brace and returns the content without the newline that
// precedes it, matching Bruno's textblock semantics.
func (l *lexer) readTextBlock() (string, Span, error) {
	l.skipHorizontal()
	if l.cur() == '}' {
		start := l.pos()
		return "", Span{Start: start, End: start}, nil
	}
	if !l.atNewline() {
		return "", l.span(l.pos()), l.error(l.pos(), "unexpected text after opening brace")
	}
	l.skipNewline()

	start := l.pos()
	var b strings.Builder
	for {
		if l.eof() {
			return "", l.span(start), l.error(start, "unterminated block")
		}
		// A line that starts with '}' closes the block.
		if l.cur() == '}' {
			return b.String(), l.span(start), nil
		}
		lineStart := l.off
		for !l.eof() && !l.atNewline() {
			l.advance(1)
		}
		line := strings.TrimSuffix(string(l.src[lineStart:l.off]), "\r")
		if l.eof() {
			return "", l.span(start), l.error(start, "unterminated block")
		}
		b.WriteString(line)
		if l.peekAfterNewline() == '}' {
			l.skipNewline()
			return b.String(), l.span(start), nil
		}
		b.WriteString("\n")
		l.skipNewline()
	}
}

// peekAfterNewline returns the byte following the current line break.
func (l *lexer) peekAfterNewline() byte {
	if l.cur() == '\r' && l.peekByte(1) == '\n' {
		return l.peekByte(2)
	}
	if l.cur() == '\n' {
		return l.peekByte(1)
	}
	return l.cur()
}

// readListBody reads a list body after the opening bracket and returns its items.
// Items are separated by whitespace or commas; quotes are stripped. The grammar
// only allows simple words, so this is lossless for valid files and forgiving for
// hand-edited ones. The line break after the closing bracket is left for the
// caller.
func (l *lexer) readListBody() ([]string, Span, error) {
	start := l.pos()
	var items []string
	var cur strings.Builder

	flush := func() {
		item := strings.TrimSpace(cur.String())
		cur.Reset()
		if item != "" {
			items = append(items, item)
		}
	}

	for {
		if l.eof() {
			return items, l.span(start), l.error(start, "unterminated list")
		}
		switch c := l.cur(); c {
		case ']':
			l.advance(1)
			flush()
			return items, l.span(start), nil
		case '"':
			text, _ := l.readQuoted()
			cur.WriteString(text)
		case ',', ' ', '\t', '\r', '\n':
			flush()
			l.advance(1)
		default:
			cur.WriteByte(c)
			l.advance(1)
		}
	}
}

// readAnnotation reads a line-initial annotation such as @number or
// @description('Page size'). It reports ok=false when the line is not an
// annotation.
func (l *lexer) readAnnotation() (Annotation, bool, Span) {
	m := l.mark()
	start := l.pos()
	if l.cur() != '@' {
		return Annotation{}, false, l.span(start)
	}
	l.advance(1)

	name := l.readAnnotationName()
	if name == "" {
		l.reset(m)
		return Annotation{}, false, l.span(start)
	}
	ann := Annotation{Name: name}

	l.skipHorizontal()
	if l.cur() == '(' {
		l.advance(1)
		arg, err := l.readAnnotationArg()
		if err != nil {
			l.reset(m)
			return Annotation{}, false, l.span(start)
		}
		ann.Arg = arg
		ann.HasArg = true
		l.skipHorizontal()
		if l.cur() != ')' {
			l.reset(m)
			return Annotation{}, false, l.span(start)
		}
		l.advance(1)
	}

	// An annotation line has no colon before its line break; otherwise it is a
	// key such as @localVar: value.
	l.skipHorizontal()
	if !l.atNewline() && !l.eof() {
		l.reset(m)
		return Annotation{}, false, l.span(start)
	}
	ann.Span = l.span(start)
	return ann, true, ann.Span
}

// readAnnotationName reads an annotation name, which stops at the argument
// delimiter and at whitespace.
func (l *lexer) readAnnotationName() string {
	start := l.off
	for !l.eof() {
		switch l.cur() {
		case '(', ')', ' ', '\t', '\r', '\n', ':':
			return string(l.src[start:l.off])
		}
		l.advance(1)
	}
	return string(l.src[start:l.off])
}

// readAnnotationArg reads one annotation argument: a quoted string, a ”' block,
// or an unquoted run up to ')'.
func (l *lexer) readAnnotationArg() (string, error) {
	switch l.cur() {
	case '\'':
		if l.hasPrefix("'''") {
			l.advance(3)
			from := l.off
			for {
				if l.eof() {
					return "", l.error(l.pos(), "unterminated annotation block")
				}
				if l.hasPrefix("'''") {
					break
				}
				l.advance(1)
			}
			content := string(l.src[from:l.off])
			l.advance(3)
			return outdentBlock(content), nil
		}
		return l.readQuotedSingle()
	case '"':
		text, _ := l.readQuoted()
		return unescapeDoubleQuoted(text), nil
	default:
		from := l.off
		for !l.eof() && l.cur() != ')' && !l.atNewline() {
			l.advance(1)
		}
		return string(l.src[from:l.off]), nil
	}
}

// readQuotedSingle reads a '...' string with no escape sequences.
func (l *lexer) readQuotedSingle() (string, error) {
	start := l.pos()
	l.advance(1)
	from := l.off
	for {
		if l.eof() || l.atNewline() {
			return "", l.error(start, "unterminated annotation argument")
		}
		if l.cur() == '\'' {
			text := string(l.src[from:l.off])
			l.advance(1)
			return text, nil
		}
		l.advance(1)
	}
}

// unescapeDoubleQuoted decodes the escape sequences Bruno supports inside a
// double-quoted annotation argument.
func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'r':
				b.WriteByte('\r')
				i++
				continue
			case 'n':
				b.WriteByte('\n')
				i++
				continue
			case 't':
				b.WriteByte('\t')
				i++
				continue
			case '"':
				b.WriteByte('"')
				i++
				continue
			case '\\':
				b.WriteByte('\\')
				i++
				continue
			}
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

// outdentBlock removes the shared indentation from a ”' block, dropping a
// leading empty line and a trailing blank line, as Bruno does for annotations.
func outdentBlock(content string) string {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSuffix(line, "\r")
	}
	if len(lines) > 0 && lines[0] == "" {
		lines = lines[1:]
	}
	if len(lines) > 0 && strings.TrimSpace(lines[len(lines)-1]) == "" {
		lines = lines[:len(lines)-1]
	}
	minIndent := -1
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if minIndent == -1 || indent < minIndent {
			minIndent = indent
		}
	}
	if minIndent < 0 {
		minIndent = 0
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			lines[i] = ""
			continue
		}
		if len(line) >= minIndent {
			lines[i] = line[minIndent:]
		}
	}
	return strings.Join(lines, "\n")
}

func (l *lexer) error(pos Pos, msg string) *Error {
	return &Error{Path: l.path, Pos: pos, Msg: msg}
}
