package lexer

import (
	// "fmt"
	"io/ioutil"
	"testing"
)

var token_result Token

func TestLexer_0(t *testing.T) {
	lexer := Lex("function __function_selector__() public {}")
    for x := range lexer.tokens {
		token_result = x
	}
	return
}

func TestLexerOneFunction(t *testing.T) {
	data, err := ioutil.ReadFile("./one_function")
	if err != nil {
		panic(err)
	}
	lexer := Lex(string(data))
    for x := range lexer.tokens {
		token_result = x
	}
	return
}

func TestLexerLongRunning(t *testing.T) {
	data, err := ioutil.ReadFile("./long_running")
	if err != nil {
		panic(err)
	}
	lexer := Lex(string(data))
    for x := range lexer.tokens {
		token_result = x
	}
	return
}
