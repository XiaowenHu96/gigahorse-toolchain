package gvm

// Parser parse the input into a CFG-like IR, called GProgram
// CFG IR is not directly runnable, but easy to be handled by our transformer

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
	typ  VarType
}

type VarType int

const (
	CONSTANT VarType = iota
	VARIABLE
)

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
	phi_mapping map[string]string // phi is a mapping from incoming edge to variable
}

// GFunction is a function representation
type GFunction struct {
	name        string
	args        []string
	blocks      []GBlock
}

func (this *GFunction) get_block(address string) *GBlock{
    for _, block := range(this.blocks) {
        if (block.address == address) {
            return &block
        }
    }
    panic(fmt.Sprintf("Cannot find block %s in GFunc %s", address, this.name))
}

// GProgram
type GProgram struct {
	functions []GFunction
}

type Parser struct {
	tokens []Token
	cur    int // current cursor (not yet read)
}

func ParseProgram(input string) GProgram {
	lexer := Lex(input)
	parser := &Parser{
		tokens: lexer.drain(),
		cur:    0,
	}
	return parser.parse_program()
}

func (p *Parser) next() (Token, bool) {
	if p.cur >= len(p.tokens) {
		return Token{EOF, "", 0, 0}, false
	}
	tok := p.tokens[p.cur]
	p.cur++
	return tok, true
}

// peek peek the ith item
func (p *Parser) peek(i int) (Token, bool) {
	if p.cur+i >= len(p.tokens) {
		return Token{EOF, "", 0, 0}, false
	}
	return p.tokens[p.cur+i], true
}

func (p *Parser) expect(expected []TokenType) Token {
	next, ok := p.next()
	if !ok {
		panic("No more token")
	}
	for _, ele := range expected {
		if next.typ == ele {
			return next
		}
	}
	panic(fmt.Sprintf("Unexpected %s", next.String()))
}

func (p *Parser) parse_program() GProgram {
	var program GProgram
	for next, ok := p.peek(0); ok && next.typ == FUNCTION; next, ok = p.peek(0) {
		program.functions = append(program.functions, p.parse_function())
	}
	// sanity check, token is exhausted
	if next, ok := p.peek(0); ok {
		// throw error
		fmt.Fprintf(os.Stderr, "Program is not exhausted! Got %s", next.String())
	}
	return program
}

// parse_list parses a list of args of type 'arg_types', enclosed by 'start' and 'end'
// seperated by comma
func (p *Parser) parse_list(start, end TokenType, arg_types []TokenType) []string {
	p.expect([]TokenType{start})
	var argument_list []string
	for peek_next, ok := p.peek(0); ok && peek_next.typ != end; peek_next, ok = p.peek(0) {
		next := p.expect(arg_types)
		argument_list = append(argument_list, next.val)
		if peek_next, ok = p.peek(0); ok && peek_next.typ != end {
			p.expect([]TokenType{COMMA})
		}
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
			[]TokenType{ADDRESS, UINT256, BOOL, BYTES})
        // TODO: TAC have this "optional" empty parenthesis
        if peek_next, ok := p.peek(0); ok && peek_next.typ == LEFT_PAREN {
            p.expect([]TokenType{LEFT_PAREN})
            p.expect([]TokenType{RIGHT_PAREN})
        }
        p.expect([]TokenType{PUBLIC})
	} else { // Hex name, private function
		function.args = p.parse_list(LEFT_PAREN, RIGHT_PAREN, []TokenType{IDENT})
		p.expect([]TokenType{PRIVATE})
	}
	p.expect([]TokenType{LEFT_BRAC})
	for next, _ := p.peek(0); next.typ != RIGHT_BRAC; next, _ = p.peek(0) {
		function.blocks = append(function.blocks, p.parse_block())
	}
	p.expect([]TokenType{RIGHT_BRAC})
	return function
}

func (p *Parser) parse_block() GBlock {
	var block GBlock
	p.expect([]TokenType{BEGIN})
	p.expect([]TokenType{BLOCK})
	block.address = p.expect([]TokenType{HEX_NUM, IDENT}).val

	// prev=[..] ,
	p.expect([]TokenType{PREV})
	p.expect([]TokenType{ASSIGN})
	block.predecessor = p.parse_list(LEFT_SQUARE_BRAC, RIGHT_SQUARE_BRAC, []TokenType{HEX_NUM, IDENT})
	p.expect([]TokenType{COMMA})

	// succ=[..]
	p.expect([]TokenType{SUCC})
	p.expect([]TokenType{ASSIGN})
	block.successor = p.parse_list(LEFT_SQUARE_BRAC, RIGHT_SQUARE_BRAC, []TokenType{HEX_NUM, IDENT})

	for next, _ := p.peek(0); next.typ != BEGIN && next.typ != RIGHT_BRAC; next, _ = p.peek(0) {
		block.statements = append(block.statements, p.parse_statement())
	}
	return block
}

func (p *Parser) parse_statement() GStatement {
	var statement GStatement
	// 0xabc :
	statement.address = p.expect([]TokenType{IDENT, HEX_NUM}).val
	p.expect([]TokenType{SEMI_COLON})

	next, _ := p.peek(0)
	var num_args int
	if next.typ == IDENT {
		statement.args = append(statement.args, p.parse_variable())
		p.expect([]TokenType{ASSIGN})
		statement.operation, num_args = p.parse_operation()
		if statement.operation == PHI {
			// TODO: parse mapping
		}
	} else {
		statement.operation, num_args = p.parse_operation()
	}
	if num_args != 0 {
		statement.args = append(statement.args, p.parse_variable())
		for {
			if peek_next, ok := p.peek(0); ok && peek_next.typ == COMMA {
				p.expect([]TokenType{COMMA})
				statement.args = append(statement.args, p.parse_variable())
			} else {
				break
			}
		}
	}
	return statement
}

func (p *Parser) parse_variable() GVariable {
	var variable GVariable
	variable.name = p.expect([]TokenType{IDENT}).val

	if next, _ := p.peek(0); next.typ == LEFT_PAREN { // a constant variable
		p.expect([]TokenType{LEFT_PAREN})
		variable.val = p.expect([]TokenType{HEX_NUM}).val
		p.expect([]TokenType{RIGHT_PAREN})
		variable.typ = CONSTANT
		return variable
	}

	variable.typ = VARIABLE
	return variable
}

func (p *Parser) parse_operation() (Opcode, int) {
	tok, _ := p.next()
    if (tok.typ < CONST) {
        panic(fmt.Sprintf("Unkown operation %s", tok.String()))
    }
	if tok.typ >= CONST && tok.typ <= GAS {
	    return tok.typ, 0
    }
	return tok.typ, -1
}
