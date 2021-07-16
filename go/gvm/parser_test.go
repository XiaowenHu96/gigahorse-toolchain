package gvm

import (
	"fmt"
	"io/ioutil"
	"testing"
)

var prog_result GProgram

func TestParserOneFunction(t *testing.T) {
	data, err := ioutil.ReadFile("./one_function")
	if err != nil {
		panic(err)
	}
    prog_result = ParseProgram(string(data))
    fmt.Println(len(prog_result.functions))
    return
}

func TestParserLongRunning(t *testing.T) {
	data, err := ioutil.ReadFile("./long_running")
	if err != nil {
		panic(err)
	}
    prog_result = ParseProgram(string(data))
    fmt.Println(len(prog_result.functions))
    return
}
