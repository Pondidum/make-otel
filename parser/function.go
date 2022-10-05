package parser

func NewFunction(id string, name string) *Function {
	return &Function{
		ID:      id,
		Name:    name,
		Called:  0,
		samples: 0,
		calls:   map[string]*Call{},
	}
}

type Function struct {
	ID     string
	Name   string
	Module string
	Called int

	samples float64
	calls   map[string]*Call
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
	Cost  float64
}
