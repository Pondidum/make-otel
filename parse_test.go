package main

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseFile(t *testing.T) {
	f, err := os.Open("example/callgrind.out.build-3")
	assert.NoError(t, err)
	defer f.Close()

	p := NewCallgrindParser(f)
	p.parse()

	assert.True(t, p.Eof())
	for n, f := range p.profile.functions {
		fmt.Printf("name: %s, called: %v, took: %v \n", n, f.called, f.samples)

		// for k, v := range f.calls {
		// 	fmt.Printf(" - %s: %v\n", k, v.calls)
		// }
	}
}
