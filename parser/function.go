package parser

import "time"

func NewFunction(id string, name string) *Function {
	return &Function{
		ID:         id,
		Name:       name,
		LineNumber: 0,
		Called:     0,
		Cost:       0,
		calls:      map[string]*Call{},
	}
}

type Function struct {
	ID         string
	Name       string
	Module     string
	LineNumber int64
	Called     int

	Cost  time.Duration
	calls map[string]*Call
}

func (f *Function) addCall(c *Call) {
	f.calls[c.CalleeId] = c
}

func (f *Function) Calls() []*Call {
	calls := make([]*Call, 0, len(f.calls))

	for _, call := range f.calls {
		calls = append(calls, call)
	}

	return calls
}

type Call struct {
	CalleeId string
	// ratio     float64
	// weight    float64

	Calls int
	Cost  time.Duration
}
