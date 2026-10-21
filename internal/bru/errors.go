package bru

import (
	"fmt"
	"strconv"
	"strings"
)

// Error is a parse or formatting error with a source position.
type Error struct {
	// Path is the file the error refers to; it may be empty for in-memory input.
	Path string
	// Pos is the position of the error.
	Pos Pos
	// Msg is a short description of the problem.
	Msg string
}

// Error implements error.
func (e *Error) Error() string {
	var b strings.Builder
	if e.Path != "" {
		b.WriteString(e.Path)
		b.WriteByte(':')
	}
	b.WriteString(strconv.Itoa(e.Pos.Line))
	b.WriteByte(':')
	b.WriteString(strconv.Itoa(e.Pos.Column))
	b.WriteString(": ")
	b.WriteString(e.Msg)
	return b.String()
}

// ErrorList collects recoverable parse errors so a caller can report them all at
// once. It implements error.
type ErrorList []*Error

// Error implements error.
func (l ErrorList) Error() string {
	msgs := make([]string, len(l))
	for i, e := range l {
		msgs[i] = e.Error()
	}
	return strings.Join(msgs, "\n")
}

// Unwrap lets errors.As reach the individual errors.
func (l ErrorList) Unwrap() []error {
	out := make([]error, len(l))
	for i, e := range l {
		out[i] = e
	}
	return out
}

// add appends a recoverable warning to the file.
func (f *File) add(path string, pos Pos, format string, args ...any) {
	f.Warnings = append(f.Warnings, &Error{Path: path, Pos: pos, Msg: fmt.Sprintf(format, args...)})
}
