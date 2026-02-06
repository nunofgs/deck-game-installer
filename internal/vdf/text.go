package vdf

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

type TokenType int

const (
	tokenString TokenType = iota
	tokenLBrace
	tokenRBrace
	tokenEOF
)

type Token struct {
	Type  TokenType
	Value string
}

type lexer struct {
	data []rune
	pos  int
}

func newLexer(input string) *lexer {
	return &lexer{data: []rune(input)}
}

func (l *lexer) nextToken() (Token, error) {
	for l.pos < len(l.data) {
		ch := l.data[l.pos]
		if isWhitespace(ch) {
			l.pos++
			continue
		}
		if ch == '/' && l.pos+1 < len(l.data) && l.data[l.pos+1] == '/' {
			// Skip line comment
			for l.pos < len(l.data) && l.data[l.pos] != '\n' {
				l.pos++
			}
			continue
		}
		if ch == '{' {
			l.pos++
			return Token{
				Type:  tokenLBrace,
				Value: "{",
			}, nil
		}
		if ch == '}' {
			l.pos++
			return Token{Type: tokenRBrace, Value: "}"}, nil
		}
		if ch == '"' {
			l.pos++
			var b strings.Builder
			for l.pos < len(l.data) {
				c := l.data[l.pos]
				if c == '\\' && l.pos+1 < len(l.data) {
					l.pos++
					b.WriteRune(l.data[l.pos])
					l.pos++
					continue
				}
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

func isWhitespace(ch rune) bool {
	return ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r'
}

func Parse(input string) (map[string]any, error) {
	l := newLexer(input)
	return parseObject(l)
}

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
			tok, err = l.nextToken()
			if err != nil {
				return nil, err
			}
			if tok.Type == tokenLBrace {
				child, err := parseObject(l)
				if err != nil {
					return nil, err
				}
				obj[key] = child
				continue
			}
			if tok.Type != tokenString {
				return nil, fmt.Errorf("expected value or { after key")
			}
			obj[key] = tok.Value
		default:
			return nil, fmt.Errorf("unexpected token")
		}
	}
}

func Dump(data map[string]any) string {
	var b bytes.Buffer
	dumpObject(&b, data, 0)
	return b.String()
}

func dumpObject(b *bytes.Buffer, data map[string]any, indent int) {
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
			b.WriteString(prefix)
			b.WriteString(fmt.Sprintf("\"%s\"\n", k))
			b.WriteString(prefix)
			b.WriteString("{\n")
			dumpObject(b, val, indent+1)
			b.WriteString(prefix)
			b.WriteString("}\n")
		default:
			b.WriteString(prefix)
			b.WriteString(fmt.Sprintf("\"%s\"\t\t\"%v\"\n", k, val))
		}
	}
}
