package lexer

import (
	"fmt"
	"io/ioutil"
	"testing"
)

func TestLexerMinimum(t *testing.T) {
	lexer := Lex("function __function_selector__() public {}")
	for x := range lexer.tokens {
		fmt.Printf("%s\n", x.String())
	}
	return
}

func TestLexerLongRunning(t *testing.T) {
	data, err := ioutil.ReadFile("./test")
	if err != nil {
		panic(err)
	}
	lexer := Lex(string(data))
	for x := range lexer.tokens {
		fmt.Printf("%s\n", x.String())
	}
	return
}
