package lexer

/**
* Luckily the syntax are straight forward.
* The only conflict are "=" v.s. block cut line "=======..."
* I'll just remove the cut line from py script and scanning requires no context anymore.
 */

import (
	"fmt"
	"os"
	"strings"
	"unicode/utf8"
)

type TokenType int

type Token struct {
	Typ  TokenType
	Val  string
	Pos  int
	Line int
}

func (tok Token) String() string {
	str := ""
	switch tok.Typ {
	case IDENT:
		str += "IDENT: "
	case HEX_NUM:
		str += "HEX: "
	}
	str = str + fmt.Sprintf("%s at %d:%d", tok.Val, tok.Line, tok.Pos)
	return str
}

type stateFn func(*Lexer) stateFn

type Lexer struct {
	input  string     // input string
	start  int        // start pos of current token
	cur    int        // current pos on the line (not yet read)
	line   int        // line scanning
	tokens chan Token // token channel
}

var keywords = map[string]TokenType{
	"FUNCTION":       FUNCTION,
	"PRIVATE":        PRIVATE,
	"PUBLIC":         PUBLIC,
	"PREV":           PREV,
	"SUCC":           SUCC,
	"BEGIN":          BEGIN,
	"BLOCK":          BLOCK,
	"STOP":           STOP,
	"ADD":            ADD,
	"MUL":            MUL,
	"SUB":            SUB,
	"DIV":            DIV,
	"SDIV":           SDIV,
	"MOD":            MOD,
	"SMOD":           SMOD,
	"ADDMOD":         ADDMOD,
	"EXP":            EXP,
	"SIGNEXTEND":     SIGNEXTEND,
	"LT":             LT,
	"GT":             GT,
	"SLT":            SLT,
	"SGT":            SGT,
	"EQ":             EQ,
	"ISZERO":         ISZERO,
	"AND":            AND,
	"OR":             OR,
	"XOR":            XOR,
	"NOT":            NOT,
	"BYTE":           BYTE,
	"SHL":            SHL,
	"SHR":            SHR,
	"SAR":            SAR,
	"SHA3":           SHA3,
	"ADDRESS":        ADDRESS,
	"BALANCE":        BALANCE,
	"ORIGIN":         ORIGIN,
	"CALLER":         CALLER,
	"CALLVALUE":      CALLVALUE,
	"CALLDATALOAD":   CALLDATALOAD,
	"CALLDATASIZE":   CALLDATASIZE,
	"CALLDATACOPY":   CALLDATACOPY,
	"CODESIZE":       CODESIZE,
	"CODECOPY":       CODECOPY,
	"GASPRICE":       GASPRICE,
	"EXTCODESIZE":    EXTCODESIZE,
	"EXTCODECOPY":    EXTCODECOPY,
	"RETURNDATASIZE": RETURNDATASIZE,
	"RETURNDATACOPY": RETURNDATACOPY,
	"EXTCODEHASH":    EXTCODEHASH,
	"BLOCKHASH":      BLOCKHASH,
	"COINBASE":       COINBASE,
	"TIMESTAMP":      TIMESTAMP,
	"NUMBER":         NUMBER,
	"DIFFICULTY":     DIFFICULTY,
	"GASLIMIT":       GASLIMIT,
	"POP":            POP,
	"MLOAD":          MLOAD,
	"MSTORE":         MSTORE,
	"MSTORE8":        MSTORE8,
	"SLOAD":          SLOAD,
	"SSTORE":         SSTORE,
	"JUMP":           JUMP,
	"JUMPI":          JUMPI,
	"PC":             PC,
	"MSIZE":          MSIZE,
	"GAS":            GAS,
	"JUMPDEST":       JUMPDEST,
	"LOG0":           LOG0,
	"LOG1":           LOG1,
	"LOG2":           LOG2,
	"LOG3":           LOG3,
	"LOG4":           LOG4,
	"CREATE":         CREATE,
	"CALL":           CALL,
	"CALLCODE":       CALLCODE,
	"RETURN":         RETURN,
	"DELEGATECALL":   DELEGATECALL,
	"CREATE2":        CREATE2,
	"STATICCALL":     STATICCALL,
	"REVERT":         REVERT,
	"SELFDESTRUCT":   SELFDESTRUCT,
	"BOOL":           BOOL,
	"UINT256":        UINT256,
	"CONST":          CONST,
	"PHI":            PHI,
	"RETURNPRIVATE":  RETURNPRIVATE,
}

// next returns the next char in the input.
func (l *Lexer) next() rune {
	if l.cur >= len(l.input) {
		return EOF
	}
	r, _ := utf8.DecodeRuneInString(l.input[l.cur:])
	l.cur++
	if r == '\n' {
		l.line++
	}
	return r
}

func (l *Lexer) peek() rune {
	r := l.next()
	if r != EOF {
		l.backup()
	}
	return r
}

func (l *Lexer) backup() {
	l.cur--
	if l.input[l.cur] == '\n' {
		l.line--
	}
}

func (l *Lexer) content() string {
	return l.input[l.start:l.cur]
}

// emit passes an item back to the client.
func (l *Lexer) emit(t TokenType) {
	l.tokens <- Token{t, l.input[l.start:l.cur], l.start, l.line}
	l.start = l.cur
}

func (l *Lexer) emitIgnore() {
	l.start = l.cur
}

// ignore skips over the pending input before this point.
func (l *Lexer) ignore() {
	l.start = l.cur
}

// accept consumes the next rune if it's from the valid set.
func (l *Lexer) accept(valid string) bool {
	if strings.ContainsRune(valid, l.next()) {
		return true
	}
	l.backup()
	return false
}

// acceptRun consumes a run of runes from the valid set.
func (l *Lexer) acceptRun(valid string) {
	for strings.ContainsRune(valid, l.next()) {
	}
	l.backup()
}

var delimiters = map[rune]TokenType{
	'{': LEFT_BRAC,
	'}': RIGHT_BRAC,
	'[': LEFT_SQUARE_BRAC,
	']': RIGHT_SQUARE_BRAC,
	',': COMMA,
	':': SEMI_COLON,
	'(': LEFT_PAREN,
	')': RIGHT_PAREN,
	'=': ASSIGN,
}

// isSpace reports whether r is a space character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

func isDelimiter(r rune) bool {
	return r == '{' || r == '}' || r == '(' || r == ')' || r == ':' || r == ',' || r == '[' || r == ']' || r == '=' || isSpace(r)
}

func isHex(str string) bool {
	if !strings.HasPrefix(str, "0X") {
		return false
	}

	valid := "1234567890ABCDEF"
	for _, char := range str[2:] {
		if !strings.ContainsRune(valid, char) {
			return false
		}
	}

	return true
}

// scan untils meets a delimiter
func (l *Lexer) scanUntilDelimeter() bool {
	for l.peek() != EOF && !isDelimiter(l.peek()) {
		l.next()
	}

	// emit the scanned token
	str := strings.ToUpper(l.content())

	if tok, ok := keywords[str]; ok {
		// a keyword
		l.emit(tok)
	} else if isHex(str) {
		// a hex
		l.emit(HEX_NUM)
	} else if len(str) > 0 {
		// a IDENT
		l.emit(IDENT)
	} /* else: empty content, omit */

	// emit the delimiter
	r := l.next()

	if tok, ok := delimiters[r]; ok {
		l.emit(tok)
	} else if r == EOF {
		return false
	} else if isSpace(r) {
		l.emitIgnore()
	} else {
		// error
		fmt.Fprintf(os.Stderr, "Cannot recognize delimiter %s", string(r))
		return false
	}
	return true
}

// nextItem returns the next item from the input.
func (l *Lexer) NextTokens() Token {
	return <-l.tokens
}

// nextItem returns the next item from the input.
func (l *Lexer) NextToken() <- chan Token {
	return l.tokens
}

// drain drains the output so the lexing goroutine will exit.
func (l *Lexer) Drain() {
	for range l.tokens {
	}
}

// lex creates a new scanner for the input string.
func Lex(input string) *Lexer {
	l := &Lexer{
		input:  input,
		start:  0,
		cur:    0,
		line:   1,
		tokens: make(chan Token),
	}
	go l.run()
	return l
}

// run runs the state machine for the Lexer.
func (l *Lexer) run() {
	for l.scanUntilDelimeter() == true {
	}
	close(l.tokens)
}

const (
	Constant TokenType = iota

	/* */
	HEX_NUM
	IDENT

	/** Delimiters **/
	COMMA
	SEMI_COLON
	RIGHT_ARROW
	LEFT_BRAC
	RIGHT_BRAC
	LEFT_PAREN
	RIGHT_PAREN
	LEFT_SQUARE_BRAC
	RIGHT_SQUARE_BRAC
	ASSIGN
	EOF = -1

	/** misc keyword */
	FUNCTION
	PRIVATE
	PUBLIC
	PREV
	SUCC
	BEGIN
	BLOCK
	CONST

	/** type keywords */
	BOOL
	UINT256

	/** op keywords **/
	PHI
	STOP
	ADD
	MUL
	SUB
	DIV
	SDIV
	MOD
	SMOD
	ADDMOD
	EXP
	SIGNEXTEND
	LT
	GT
	SLT
	SGT
	EQ
	ISZERO
	AND
	OR
	XOR
	NOT
	BYTE
	SHL
	SHR
	SAR
	SHA3
	ADDRESS
	BALANCE
	ORIGIN
	CALLER
	CALLVALUE
	CALLDATALOAD
	CALLDATASIZE
	CALLDATACOPY
	CODESIZE
	CODECOPY
	GASPRICE
	EXTCODESIZE
	EXTCODECOPY
	RETURNDATASIZE
	RETURNDATACOPY
	EXTCODEHASH
	BLOCKHASH
	COINBASE
	TIMESTAMP
	NUMBER
	DIFFICULTY
	GASLIMIT
	POP
	MLOAD
	MSTORE
	MSTORE8
	SLOAD
	SSTORE
	JUMP
	JUMPI
	PC
	MSIZE
	GAS
	JUMPDEST
	LOG0
	LOG1
	LOG2
	LOG3
	LOG4
	CREATE
	CALL
	CALLCODE
	RETURN
	DELEGATECALL
	CREATE2
	STATICCALL
	REVERT
	SELFDESTRUCT
	RETURNPRIVATE
)
