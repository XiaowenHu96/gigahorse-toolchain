package lexer

import (
	"fmt"
	"os"
)

type Opcode = TokenType

// GVariable is either a variable or a constant
// Xihu: Because I don't know if we need the constant information or not later for execution
// For now I'll just keep both
type GVariable struct {
	name string
	val  string
	typ  int
}

// GStatement is a single statement
type GStatement struct {
	address   string
	operation Opcode
	args      []GVariable
}

// GBlock is a basic control block representation
type GBlock struct {
	address     string
	predecessor []string
	successor   []string
	statements  []GStatement
}

// GFunction is a function representation
type GFunction struct {
	name        string
	args        []string
	phi_mapping map[string]string // phi is a mapping from incoming edge to variable
	blocks      []GBlock
}

// GProgram
type GProgram struct {
	functions []GFunction
}

type Parser struct {
	tokens []Token
	cur    int // current cursor (not yet read)
}

func (p *Parser) next() (Token, bool) {
	if p.cur >= len(p.tokens) {
		return Token{EOF, "", 0, 0}, false
	}
	tok := p.tokens[p.cur]
	p.cur++
	return tok, true
}

func (p *Parser) peek(i int) (Token, bool) {
	if p.cur+i >= len(p.tokens) {
		return Token{EOF, "", 0, 0}, false
	}
	return p.tokens[p.cur+i], true
}

func (p *Parser) expect(expected []TokenType) Token {
	next, ok := p.next()
	if !ok {
		fmt.Fprintf(os.Stderr, "No more tokens")
		// TODO: throw, exit
	}
	for _, ele := range expected {
		if next.typ == ele {
			return next
		}
	}
	fmt.Fprintf(os.Stderr, "Unexpected %s at %d:%d", next.val, next.pos, next.line)
	return next
}

func (p *Parser) parse_program() GProgram {
	var program GProgram
	for next, ok := p.peek(1); ok && next.typ == FUNCTION; next, ok = p.peek(1) {
		program.functions = append(program.functions, p.parse_function())
	}
	// sanity check, token is exhausted
	if _, ok := p.peek(1); ok {
		// throw error
	}
	return program
}

// parse_list parses a list of args of type 'arg_types', closed by 'start' and 'end'
// seperated by comma
func (p *Parser) parse_list(start, end TokenType, arg_types []TokenType) []string {
	p.expect([]TokenType{start})
	var argument_list []string
	for peek_next, ok := p.peek(1); ok && peek_next.typ != end; {
		next := p.expect(arg_types)
		argument_list = append(argument_list, next.val)
		p.expect([]TokenType{COMMA})
		peek_next, ok = p.peek(1)
	}
	p.expect([]TokenType{end})
	return argument_list
}

func (p *Parser) parse_function() GFunction {
	var function GFunction
	p.expect([]TokenType{FUNCTION})

	function_name := p.expect([]TokenType{IDENT, HEX_NUM})
	if function_name.typ == IDENT { // public function
		function.args = p.parse_list(LEFT_PAREN, RIGHT_PAREN,
			[]TokenType{ADDRESS, UINT256, BOOL, BYTE})
		p.expect([]TokenType{LEFT_PAREN})
		p.expect([]TokenType{RIGHT_PAREN})
		p.expect([]TokenType{PUBLIC})
	} else { // Hex name, private function
		function.args = p.parse_list(LEFT_PAREN, RIGHT_PAREN, []TokenType{HEX_NUM})
		p.expect([]TokenType{PRIVATE})
	}
	p.expect([]TokenType{LEFT_BRAC})
	for next, _ := p.peek(1); next.typ != RIGHT_BRAC; next, _ = p.peek(1) {
		function.blocks = append(function.blocks, p.parse_block())
	}
	p.expect([]TokenType{RIGHT_BRAC})
	return function
}

func (p *Parser) parse_block() GBlock {
	var block GBlock
	p.expect([]TokenType{BEGIN})
	p.expect([]TokenType{BLOCK})
	block.address = p.expect([]TokenType{HEX_NUM}).val

	// prev=[..] ,
	p.expect([]TokenType{PREV})
	p.expect([]TokenType{ASSIGN})
	block.predecessor = p.parse_list(LEFT_SQUARE_BRAC, RIGHT_SQUARE_BRAC, []TokenType{HEX_NUM, IDENT})
	p.expect([]TokenType{COMMA})

	// succ=[..]
	p.expect([]TokenType{SUCC})
	p.expect([]TokenType{ASSIGN})
	block.successor = p.parse_list(LEFT_SQUARE_BRAC, RIGHT_SQUARE_BRAC, []TokenType{HEX_NUM, IDENT})

	for next, _ := p.peek(1); next.typ != BEGIN; next, _ = p.peek(1) {
		block.statements = append(block.statements, p.parse_statement())
	}
	return block
}

func (p *Parser) parse_statement() GStatement {
	var statement GStatement
	// 0xabc :
	statement.address = p.expect([]TokenType{IDENT, HEX_NUM}).val
	p.expect([]TokenType{SEMI_COLON})

	next, _ := p.peek(1)
	if next.typ == IDENT {
		statement.args = append(statement.args, p.parse_variable())
		p.expect([]TokenType{ASSIGN})
		var num_args int
		statement.operation, num_args = p.parse_operation()
		for i := 0; i < num_args; i++ {
			if i < num_args-1 {
				p.expect([]TokenType{COMMA})
			}
		}

		if statement.operation == PHI {
			// phi needs special treatment, variable length arguments
			for tk, ok := p.peek(2); ok && tk.typ == COMMA; {
				statement.args = append(statement.args, p.parse_variable())
				p.expect([]TokenType{COMMA})
			}
			// parse the last arg
			statement.args = append(statement.args, p.parse_variable())
		}
	} else {
		var num_args int
		statement.operation, num_args = p.parse_operation()
		for i := 0; i < num_args; i++ {
			if i < num_args-1 {
				p.expect([]TokenType{COMMA})
			}
		}
	}
	return statement
}

func (p *Parser) parse_variable() GVariable {
	var variable GVariable
	variable.name = p.expect([]TokenType{IDENT}).val

	if next, _ := p.peek(1); next.typ == LEFT_PAREN { // a constant variable
		p.expect([]TokenType{LEFT_PAREN})
		variable.val = p.expect([]TokenType{HEX_NUM}).val
		p.expect([]TokenType{RIGHT_PAREN})
		variable.typ = CONSTANT
		return variable
	}

	variable.typ = VARIABLE
	return variable
}

func num_of_args(tok Token) int {
	if tok.typ >= CONST && tok.typ <= GAS {
		// nullary
		return 0
	} else if tok.typ <= NOT {
		return 1
	} else if tok.typ <= LOG0 {
		return 2
	} else {
		return -1
	}
}

func (p *Parser) parse_operation() (Opcode, int) {
	tok, _ := p.next()
	return tok.typ, num_of_args(tok)
}
