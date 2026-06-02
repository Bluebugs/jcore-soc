package vhdl

// Lexer holds the state for lexing a single VHDL source file.
type Lexer struct {
	src     []byte
	file    string
	off     int  // current byte offset into src
	line    int  // 1-based
	col     int  // 1-based
	prevKind Kind // Kind of the last emitted token (used for tick disambiguation)
}

// NewLexer returns a new Lexer for the given source bytes.
func NewLexer(src []byte, file string) *Lexer {
	return &Lexer{src: src, file: file, line: 1, col: 1}
}

// peek returns the byte at the current offset, or 0 if at end.
func (l *Lexer) peek() byte {
	if l.off >= len(l.src) {
		return 0
	}
	return l.src[l.off]
}

// peekAt returns the byte at offset+n, or 0 if out of range.
func (l *Lexer) peekAt(n int) byte {
	i := l.off + n
	if i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

// advance consumes one byte, updating line/col tracking.
func (l *Lexer) advance() byte {
	if l.off >= len(l.src) {
		return 0
	}
	c := l.src[l.off]
	l.off++
	if c == '\n' {
		l.line++
		l.col = 1
	} else {
		l.col++
	}
	return c
}

func isLetter(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isDigit(c byte) bool {
	return c >= '0' && c <= '9'
}

func isIdentStart(c byte) bool {
	return isLetter(c)
}

func isIdentCont(c byte) bool {
	return isLetter(c) || isDigit(c) || c == '_'
}

// isBitstringBase returns true if c is b/B/o/O/x/X.
func isBitstringBase(c byte) bool {
	return c == 'b' || c == 'B' || c == 'o' || c == 'O' || c == 'x' || c == 'X'
}

// isNameEnd returns true for token kinds that can end a name (IDENT, EXTIDENT, RPAREN, TICK).
// After these, a single-quote is the attribute tick, not the start of a char literal.
func isNameEnd(k Kind) bool {
	return k == IDENT || k == EXTIDENT || k == RPAREN || k == TICK
}

// emit records the kind as the most recently emitted and returns the token.
func (l *Lexer) emit(tok Token) Token {
	l.prevKind = tok.Kind
	return tok
}

// Next returns the next token from the source.
func (l *Lexer) Next() Token {
	// Skip whitespace.
	for l.off < len(l.src) {
		c := l.peek()
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			l.advance()
		} else {
			break
		}
	}

	if l.off >= len(l.src) {
		return l.emit(Token{Kind: EOF, Pos: Pos{Line: l.line, Col: l.col, Offset: l.off}})
	}

	startPos := Pos{Line: l.line, Col: l.col, Offset: l.off}
	c := l.peek()

	// -- comment
	if c == '-' && l.peekAt(1) == '-' {
		start := l.off
		// consume until end of line or EOF
		for l.off < len(l.src) && l.peek() != '\n' {
			l.advance()
		}
		return l.emit(Token{Kind: COMMENT, Lit: string(l.src[start:l.off]), Pos: startPos})
	}

	// Extended identifier: \...\  with \\ escape inside
	if c == '\\' {
		start := l.off
		l.advance() // consume opening '\'
		for l.off < len(l.src) {
			ch := l.peek()
			if ch == '\\' {
				l.advance() // consume '\'
				if l.peek() == '\\' {
					l.advance() // escape: consume second '\'
				} else {
					// closing backslash
					break
				}
			} else {
				l.advance()
			}
		}
		return l.emit(Token{Kind: EXTIDENT, Lit: string(l.src[start:l.off]), Pos: startPos})
	}

	// String literal: "..." with "" as escaped quote
	if c == '"' {
		start := l.off
		l.advance() // consume opening '"'
		for l.off < len(l.src) {
			ch := l.peek()
			if ch == '"' {
				l.advance() // consume '"'
				if l.peek() == '"' {
					l.advance() // escaped quote inside string
				} else {
					// closing quote
					break
				}
			} else {
				l.advance()
			}
		}
		return l.emit(Token{Kind: STRINGLIT, Lit: string(l.src[start:l.off]), Pos: startPos})
	}

	// Identifier, keyword, or bit-string literal
	if isIdentStart(c) {
		start := l.off
		for l.off < len(l.src) && isIdentCont(l.peek()) {
			l.advance()
		}
		lit := string(l.src[start:l.off])
		// Check for bit-string literal: single base letter followed by "
		if len(lit) == 1 && isBitstringBase(lit[0]) && l.peek() == '"' {
			// lex the string part
			strStart := l.off
			l.advance() // opening '"'
			for l.off < len(l.src) {
				ch := l.peek()
				if ch == '"' {
					l.advance()
					if l.peek() == '"' {
						l.advance() // escaped
					} else {
						break
					}
				} else {
					l.advance()
				}
			}
			return l.emit(Token{Kind: BITSTRINGLIT, Lit: string(l.src[start:strStart]) + string(l.src[strStart:l.off]), Pos: startPos})
		}
		if kind, ok := LookupKeyword(lit); ok {
			return l.emit(Token{Kind: kind, Lit: lit, Pos: startPos})
		}
		return l.emit(Token{Kind: IDENT, Lit: lit, Pos: startPos})
	}

	// Numeric literals: integer, based, or real
	if isDigit(c) {
		start := l.off
		// consume leading digit run (with underscores)
		for l.off < len(l.src) {
			ch := l.peek()
			if isDigit(ch) || ch == '_' {
				l.advance()
			} else {
				break
			}
		}
		// Based literal: digits # based_digits #
		if l.peek() == '#' {
			l.advance() // consume first '#'
			// consume based digits (hex digits + underscores)
			for l.off < len(l.src) {
				ch := l.peek()
				if isDigit(ch) || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F') || ch == '_' {
					l.advance()
				} else {
					break
				}
			}
			if l.peek() == '#' {
				l.advance() // consume closing '#'
			}
			// optional exponent
			if l.peek() == 'e' || l.peek() == 'E' {
				l.advance()
				if l.peek() == '+' || l.peek() == '-' {
					l.advance()
				}
				for l.off < len(l.src) && isDigit(l.peek()) {
					l.advance()
				}
			}
			return l.emit(Token{Kind: BASEDLIT, Lit: string(l.src[start:l.off]), Pos: startPos})
		}
		// Real literal: digits . digits [exponent]
		if l.peek() == '.' && isDigit(l.peekAt(1)) {
			l.advance() // consume '.'
			for l.off < len(l.src) && (isDigit(l.peek()) || l.peek() == '_') {
				l.advance()
			}
			// optional exponent
			if l.peek() == 'e' || l.peek() == 'E' {
				l.advance()
				if l.peek() == '+' || l.peek() == '-' {
					l.advance()
				}
				for l.off < len(l.src) && isDigit(l.peek()) {
					l.advance()
				}
			}
			return l.emit(Token{Kind: REAL, Lit: string(l.src[start:l.off]), Pos: startPos})
		}
		// Integer with exponent is also REAL (e.g. 1e3)
		if l.peek() == 'e' || l.peek() == 'E' {
			l.advance()
			if l.peek() == '+' || l.peek() == '-' {
				l.advance()
			}
			for l.off < len(l.src) && isDigit(l.peek()) {
				l.advance()
			}
			return l.emit(Token{Kind: REAL, Lit: string(l.src[start:l.off]), Pos: startPos})
		}
		return l.emit(Token{Kind: INT, Lit: string(l.src[start:l.off]), Pos: startPos})
	}

	// Delimiters — maximal munch.
	l.advance() // consume first char
	switch c {
	case '(':
		return l.emit(Token{Kind: LPAREN, Pos: startPos})
	case ')':
		return l.emit(Token{Kind: RPAREN, Pos: startPos})
	case ',':
		return l.emit(Token{Kind: COMMA, Pos: startPos})
	case ';':
		return l.emit(Token{Kind: SEMICOLON, Pos: startPos})
	case '+':
		return l.emit(Token{Kind: PLUS, Pos: startPos})
	case '-':
		return l.emit(Token{Kind: MINUS, Pos: startPos})
	case '&':
		return l.emit(Token{Kind: AMP, Pos: startPos})
	case '|':
		return l.emit(Token{Kind: BAR, Pos: startPos})
	case '.':
		return l.emit(Token{Kind: DOT, Pos: startPos})
	case '\'':
		// Tick disambiguation: if prev token can end a name, this is attribute tick.
		// Otherwise, if the next two bytes form '<char>' it is a char literal.
		if isNameEnd(l.prevKind) {
			return l.emit(Token{Kind: TICK, Pos: startPos})
		}
		// Check for char literal: 'x' — one graphic char then closing quote
		if l.peekAt(1) == '\'' {
			ch := l.advance() // consume the graphic char
			l.advance()       // consume closing quote
			lit := "'" + string(ch) + "'"
			return l.emit(Token{Kind: CHARLIT, Lit: lit, Pos: startPos})
		}
		return l.emit(Token{Kind: TICK, Pos: startPos})
	case ':':
		if l.peek() == '=' {
			l.advance()
			return l.emit(Token{Kind: ASSIGN, Pos: startPos})
		}
		return l.emit(Token{Kind: COLON, Pos: startPos})
	case '=':
		if l.peek() == '>' {
			l.advance()
			return l.emit(Token{Kind: ARROW, Pos: startPos})
		}
		return l.emit(Token{Kind: EQ, Pos: startPos})
	case '>':
		if l.peek() == '=' {
			l.advance()
			return l.emit(Token{Kind: GE, Pos: startPos})
		}
		return l.emit(Token{Kind: GT, Pos: startPos})
	case '<':
		if l.peek() == '=' {
			l.advance()
			return l.emit(Token{Kind: LE, Pos: startPos})
		}
		if l.peek() == '>' {
			l.advance()
			return l.emit(Token{Kind: BOX, Pos: startPos})
		}
		return l.emit(Token{Kind: LT, Pos: startPos})
	case '/':
		if l.peek() == '=' {
			l.advance()
			return l.emit(Token{Kind: NE, Pos: startPos})
		}
		return l.emit(Token{Kind: SLASH, Pos: startPos})
	case '*':
		if l.peek() == '*' {
			l.advance()
			return l.emit(Token{Kind: EXP, Pos: startPos})
		}
		return l.emit(Token{Kind: STAR, Pos: startPos})
	default:
		return l.emit(Token{Kind: ILLEGAL, Lit: string(c), Pos: startPos})
	}
}
