// Package lexer implements a lexer for the DOT language.
package lexer

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/anthonybishopric/gographviz/pkg/token"
)

// Lexer tokenizes DOT source code.
type Lexer struct {
	src      []byte
	ch       rune // current character
	offset   int  // current byte offset
	rdOffset int  // reading offset (position after current character)
	line     int
	column   int

	filename string
	Errors   []Error
}

// Error represents a lexer error.
type Error struct {
	Pos token.Position
	Msg string
}

func (e Error) Error() string {
	return e.Pos.String() + ": " + e.Msg
}

// New creates a new Lexer for the given source.
func New(filename string, src []byte) *Lexer {
	l := &Lexer{
		src:      src,
		filename: filename,
		line:     1,
		column:   0,
	}
	l.next() // initialize ch
	return l
}

// next reads the next character into l.ch.
func (l *Lexer) next() {
	if l.rdOffset >= len(l.src) {
		l.ch = -1 // EOF
		l.offset = len(l.src)
		return
	}
	l.offset = l.rdOffset
	if l.ch == '\n' {
		l.line++
		l.column = 0
	}
	r, w := utf8.DecodeRune(l.src[l.rdOffset:])
	l.rdOffset += w
	l.column++
	l.ch = r
}

// peek returns the next character without advancing.
func (l *Lexer) peek() rune {
	if l.rdOffset >= len(l.src) {
		return -1
	}
	r, _ := utf8.DecodeRune(l.src[l.rdOffset:])
	return r
}

func (l *Lexer) pos() token.Position {
	return token.Position{
		Filename: l.filename,
		Offset:   l.offset,
		Line:     l.line,
		Column:   l.column,
	}
}

func (l *Lexer) error(pos token.Position, msg string) {
	l.Errors = append(l.Errors, Error{Pos: pos, Msg: msg})
}

func (l *Lexer) skipWhitespace() {
	for l.ch == ' ' || l.ch == '\t' || l.ch == '\n' || l.ch == '\r' {
		l.next()
	}
}

func (l *Lexer) skipLineComment() {
	for l.ch != '\n' && l.ch != -1 {
		l.next()
	}
}

func (l *Lexer) skipBlockComment() bool {
	// Already consumed /*
	for {
		if l.ch == -1 {
			return false // unterminated
		}
		if l.ch == '*' && l.peek() == '/' {
			l.next() // consume *
			l.next() // consume /
			return true
		}
		l.next()
	}
}

func isLetter(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || ch == '_'
}

func isDigit(ch rune) bool {
	return ch >= '0' && ch <= '9'
}

func isAlphaNumeric(ch rune) bool {
	return isLetter(ch) || isDigit(ch)
}

func (l *Lexer) scanIdent() string {
	start := l.offset
	for isAlphaNumeric(l.ch) {
		l.next()
	}
	return string(l.src[start:l.offset])
}

func (l *Lexer) scanNumber() string {
	start := l.offset

	// Optional leading minus
	if l.ch == '-' {
		l.next()
	}

	// Digits before decimal point
	for isDigit(l.ch) {
		l.next()
	}

	// Optional decimal point and fractional part
	if l.ch == '.' {
		l.next()
		for isDigit(l.ch) {
			l.next()
		}
	}

	return string(l.src[start:l.offset])
}

func (l *Lexer) scanString() (string, bool) {
	// Already consumed opening "
	var sb strings.Builder
	for {
		if l.ch == -1 || l.ch == '\n' {
			return sb.String(), false // unterminated
		}
		if l.ch == '"' {
			l.next() // consume closing "
			return sb.String(), true
		}
		if l.ch == '\\' {
			l.next()
			switch l.ch {
			case 'n':
				sb.WriteRune('\n')
			case 't':
				sb.WriteRune('\t')
			case 'r':
				sb.WriteRune('\r')
			case '"':
				sb.WriteRune('"')
			case '\\':
				sb.WriteRune('\\')
			default:
				// DOT allows escaped newlines and other chars
				sb.WriteRune(l.ch)
			}
		} else {
			sb.WriteRune(l.ch)
		}
		l.next()
	}
}

func (l *Lexer) scanHTMLString() (string, bool) {
	// Already consumed opening <
	var sb strings.Builder
	depth := 1
	for {
		if l.ch == -1 {
			return sb.String(), false // unterminated
		}
		if l.ch == '<' {
			depth++
			sb.WriteRune(l.ch)
		} else if l.ch == '>' {
			depth--
			if depth == 0 {
				l.next() // consume closing >
				return sb.String(), true
			}
			sb.WriteRune(l.ch)
		} else {
			sb.WriteRune(l.ch)
		}
		l.next()
	}
}

// Scan returns the next token.
func (l *Lexer) Scan() (pos token.Position, tok token.Token, lit string) {
	l.skipWhitespace()

	pos = l.pos()

	// Handle comments and preprocessor lines
	for {
		if l.ch == '/' {
			if l.peek() == '/' {
				l.next() // consume first /
				l.next() // consume second /
				l.skipLineComment()
				l.skipWhitespace()
				pos = l.pos()
				continue
			} else if l.peek() == '*' {
				l.next() // consume /
				l.next() // consume *
				if !l.skipBlockComment() {
					l.error(pos, "unterminated block comment")
				}
				l.skipWhitespace()
				pos = l.pos()
				continue
			}
		}
		if l.ch == '#' {
			l.skipLineComment()
			l.skipWhitespace()
			pos = l.pos()
			continue
		}
		break
	}

	switch {
	case l.ch == -1:
		tok = token.EOF

	case isLetter(l.ch):
		lit = l.scanIdent()
		// Case-insensitive keyword lookup
		tok = token.Lookup(strings.ToLower(lit))

	case isDigit(l.ch) || (l.ch == '-' && isDigit(l.peek())):
		lit = l.scanNumber()
		tok = token.IDENT // numbers are valid IDs in DOT

	case l.ch == '.':
		// Could be start of a number like .5
		if isDigit(l.peek()) {
			lit = l.scanNumber()
			tok = token.IDENT
		} else {
			l.error(pos, "unexpected character: "+string(l.ch))
			tok = token.ILLEGAL
			l.next()
		}

	case l.ch == '"':
		l.next() // consume opening "
		var ok bool
		lit, ok = l.scanString()
		if !ok {
			l.error(pos, "unterminated string")
		}
		tok = token.STRING

	case l.ch == '<':
		l.next() // consume opening <
		var ok bool
		lit, ok = l.scanHTMLString()
		if !ok {
			l.error(pos, "unterminated HTML string")
		}
		tok = token.HTML

	case l.ch == '{':
		tok = token.LBRACE
		l.next()

	case l.ch == '}':
		tok = token.RBRACE
		l.next()

	case l.ch == '[':
		tok = token.LBRACKET
		l.next()

	case l.ch == ']':
		tok = token.RBRACKET
		l.next()

	case l.ch == ';':
		tok = token.SEMICOLON
		l.next()

	case l.ch == ':':
		tok = token.COLON
		l.next()

	case l.ch == ',':
		tok = token.COMMA
		l.next()

	case l.ch == '=':
		tok = token.EQUAL
		l.next()

	case l.ch == '-':
		l.next()
		if l.ch == '>' {
			tok = token.ARROW
			l.next()
		} else if l.ch == '-' {
			tok = token.DASHDASH
			l.next()
		} else {
			// Standalone minus is illegal in this context
			l.error(pos, "unexpected character: -")
			tok = token.ILLEGAL
		}

	default:
		if unicode.IsPrint(l.ch) {
			l.error(pos, "unexpected character: "+string(l.ch))
		} else {
			l.error(pos, "unexpected character")
		}
		tok = token.ILLEGAL
		l.next()
	}

	return
}
