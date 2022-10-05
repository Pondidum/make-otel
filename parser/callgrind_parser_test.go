package parser

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBuildCost(t *testing.T) {
	costDefinition := "100usec"
	cost := float64(50034)

	duration := buildCost(cost, costDefinition)
	assert.Equal(t, 5*time.Second, duration.Truncate(time.Second))
}

func TestParseFile(t *testing.T) {
	f, err := os.Open("../example/callgrind.out.build-3")
	assert.NoError(t, err)
	defer f.Close()

	p := NewCallgrindParser(f)
	profile, err := p.Parse()
	assert.NoError(t, err)
	assert.Equal(t, 9*time.Second, profile.TotalCost.Truncate(time.Second))

	// for _, fn := range profile.Roots() {
	// 	fmt.Println(fn.Name)
	// }

}
