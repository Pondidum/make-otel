package main

import (
	"makeotel/parser"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFile(t *testing.T) {
	f, err := os.Open("example/callgrind.out.build-3")
	assert.NoError(t, err)
	defer f.Close()

	p := parser.NewCallgrindParser(f)
	_, err = p.Parse()
	assert.NoError(t, err)

	// for _, fn := range profile.Roots() {
	// 	fmt.Println(fn.Name)
	// }

}
