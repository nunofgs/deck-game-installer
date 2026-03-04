// Package vdf provides parsing and serialization for Valve's text-based VDF format.
// This is used for config.vdf and other Steam configuration files.
package vdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// TokenType represents the type of a lexer token.
type TokenType int

const (
	tokenString TokenType = iota
	tokenLBrace
	tokenRBrace
	tokenEOF
)

// Token represents a single lexer token.
type Token struct {
	Type  TokenType
	Value string
}

// lexer tokenizes VDF text input.
type lexer struct {
	data []rune
	pos  int
}

// newLexer creates a new lexer for the given input string.
func newLexer(input string) *lexer {
	return &lexer{data: []rune(input)}
}

// nextToken returns the next token from the input.
func (l *lexer) nextToken() (Token, error) {
	for l.pos < len(l.data) {
		ch := l.data[l.pos]

		// Skip whitespace
		if isWhitespace(ch) {
			l.pos++
			continue
		}

		// Skip line comments
		if ch == '/' && l.pos+1 < len(l.data) && l.data[l.pos+1] == '/' {
			for l.pos < len(l.data) && l.data[l.pos] != '\n' {
				l.pos++
			}
			continue
		}

		// Handle braces
		if ch == '{' {
			l.pos++
			return Token{Type: tokenLBrace, Value: "{"}, nil
		}
		if ch == '}' {
			l.pos++
			return Token{Type: tokenRBrace, Value: "}"}, nil
		}

		// Handle quoted strings
		if ch == '"' {
			l.pos++
			var b strings.Builder
			for l.pos < len(l.data) {
				c := l.data[l.pos]
				// Handle escape sequences
				if c == '\\' && l.pos+1 < len(l.data) {
					l.pos++
					b.WriteRune(l.data[l.pos])
					l.pos++
					continue
				}
				// End of string
				if c == '"' {
					l.pos++
					return Token{Type: tokenString, Value: b.String()}, nil
				}
				b.WriteRune(c)
				l.pos++
			}
			return Token{}, fmt.Errorf("unterminated string")
		}

		return Token{}, fmt.Errorf("unexpected character: %q", ch)
	}
	return Token{Type: tokenEOF}, nil
}

// isWhitespace returns true if the character is whitespace.
func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

// Parse parses VDF text into a nested map structure.
func Parse(input string) (map[string]any, error) {
	l := newLexer(input)
	return parseObject(l)
}

// parseObject parses a VDF object (key-value pairs within braces).
func parseObject(l *lexer) (map[string]any, error) {
	obj := map[string]any{}

	for {
		tok, err := l.nextToken()
		if err != nil {
			return nil, err
		}

		switch tok.Type {
		case tokenEOF, tokenRBrace:
			return obj, nil

		case tokenString:
			key := tok.Value

			// Get the value (either another object or a string)
			tok, err = l.nextToken()
			if err != nil {
				return nil, err
			}

			if tok.Type == tokenLBrace {
				// Nested object
				child, err := parseObject(l)
				if err != nil {
					return nil, err
				}
				obj[key] = child
				continue
			}

			if tok.Type != tokenString {
				return nil, fmt.Errorf("expected value or { after key %q", key)
			}
			obj[key] = tok.Value

		default:
			return nil, fmt.Errorf("unexpected token: %v", tok)
		}
	}
}

// Dump serializes a map structure back to VDF text format.
func Dump(data map[string]any) string {
	var b bytes.Buffer
	dumpObject(&b, data, 0)
	return b.String()
}

// dumpObject recursively writes a map to the buffer in VDF format.
func dumpObject(b *bytes.Buffer, data map[string]any, indent int) {
	// Sort keys for consistent output
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	prefix := strings.Repeat("\t", indent)

	for _, k := range keys {
		v := data[k]
		switch val := v.(type) {
		case map[string]any:
			// Nested object
			b.WriteString(prefix)
			b.WriteString(fmt.Sprintf("\"%s\"\n", escapeVDFString(k)))
			b.WriteString(prefix)
			b.WriteString("{\n")
			dumpObject(b, val, indent+1)
			b.WriteString(prefix)
			b.WriteString("}\n")
		default:
			// String value — escape backslashes and quotes so the file round-trips cleanly.
			b.WriteString(prefix)
			b.WriteString(fmt.Sprintf("\"%s\"\t\t\"%s\"\n", k, escapeVDFString(fmt.Sprintf("%v", val))))
		}
	}
}

// escapeVDFString escapes backslashes and double-quotes inside a VDF string value
// so that the file can be re-parsed correctly after being written.
func escapeVDFString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// GetNestedMap retrieves a nested map by a path of keys.
// Returns nil if the path doesn't exist or isn't a map.
func GetNestedMap(data map[string]any, keys ...string) map[string]any {
	current := data
	for _, key := range keys {
		if val, ok := current[key]; ok {
			if nested, ok := val.(map[string]any); ok {
				current = nested
			} else {
				return nil
			}
		} else {
			return nil
		}
	}
	return current
}


