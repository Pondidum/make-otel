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
	assert.Equal(t, "remake --profile build", profile.Command)
	assert.Equal(t, "remake 4.3+dbg-1.5", profile.Creator)

	assert.Len(t, profile.functions, 7)

	assert.Len(t, profile.Roots(), 1)

	// top level, the `build` task
	build := profile.Roots()[0]
	assert.Equal(t, "build", build.ID)
	assert.Equal(t, "build", build.Name)
	assert.Equal(t, "", build.Module)
	assert.Equal(t, int64(8), build.LineNumber)
	assert.Equal(t, 0, build.Called)
	assert.Equal(t, 100*time.Microsecond, build.Cost)

	assert.Len(t, build.calls, 2)
	assert.Contains(t, build.calls, "one.js")
	assert.Contains(t, build.calls, "two.js")

	// one.js
	onejs, found := profile.GetFunction("one.js")
	assert.True(t, found)
	assert.Equal(t, "one.js", onejs.ID)
	assert.Equal(t, "one.js", onejs.Name)
	assert.Equal(t, "", onejs.Module)
	assert.Equal(t, int64(10), onejs.LineNumber)
	assert.Equal(t, 1, onejs.Called)
	assert.Equal(t, 3002300*time.Microsecond, onejs.Cost)

	assert.Len(t, onejs.calls, 1)
	assert.Contains(t, onejs.calls, "one.ts")

	// one.ts
	onets, found := profile.GetFunction("one.ts")
	assert.True(t, found)
	assert.Equal(t, "one.ts", onets.ID)
	assert.Equal(t, "one.ts", onets.Name)
	assert.Equal(t, "", onets.Module)
	assert.Equal(t, int64(0), onets.LineNumber)
	assert.Equal(t, 1, onets.Called)
	assert.Equal(t, 100*time.Microsecond, onets.Cost)

	assert.Len(t, onets.calls, 0)

	// two.js
	twojs, found := profile.GetFunction("two.js")
	assert.True(t, found)
	assert.Equal(t, "two.js", twojs.ID)
	assert.Equal(t, "two.js", twojs.Name)
	assert.Equal(t, "", twojs.Module)
	assert.Equal(t, int64(15), twojs.LineNumber)
	assert.Equal(t, 1, twojs.Called)
	assert.Equal(t, 6006900*time.Microsecond, twojs.Cost)

	assert.Len(t, twojs.calls, 2)
	assert.Contains(t, twojs.calls, "two.ts")
	assert.Contains(t, twojs.calls, "three.js")

	// two.ts
	twots, found := profile.GetFunction("two.ts")
	assert.True(t, found)
	assert.Equal(t, "two.ts", twots.ID)
	assert.Equal(t, "two.ts", twots.Name)
	assert.Equal(t, "", twots.Module)
	assert.Equal(t, int64(0), twots.LineNumber)
	assert.Equal(t, 1, twots.Called)
	assert.Equal(t, 100*time.Microsecond, twots.Cost)

	assert.Len(t, twots.calls, 0)

	// three.js
	threejs, found := profile.GetFunction("three.js")
	assert.True(t, found)
	assert.Equal(t, "three.js", threejs.ID)
	assert.Equal(t, "three.js", threejs.Name)
	assert.Equal(t, "", threejs.Module)
	assert.Equal(t, int64(20), threejs.LineNumber)
	assert.Equal(t, 1, threejs.Called)
	assert.Equal(t, 5003400*time.Microsecond, threejs.Cost)

	assert.Len(t, threejs.calls, 1)
	assert.Contains(t, threejs.calls, "three.ts")

	// three.ts
	threets, found := profile.GetFunction("three.ts")
	assert.True(t, found)
	assert.Equal(t, "three.ts", threets.ID)
	assert.Equal(t, "three.ts", threets.Name)
	assert.Equal(t, "", threets.Module)
	assert.Equal(t, int64(0), threets.LineNumber)
	assert.Equal(t, 1, threets.Called)
	assert.Equal(t, 100*time.Microsecond, threets.Cost)

	assert.Len(t, threets.calls, 0)
}
