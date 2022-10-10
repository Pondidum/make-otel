package parser

import "time"

type Profile struct {
	functions map[string]*Function

	Creator string
	Command string

	TotalCost time.Duration
}

func (p *Profile) addFunction(f *Function) {
	p.functions[f.ID] = f
}

func (p *Profile) GetFunction(id string) (*Function, bool) {
	f, found := p.functions[id]
	return f, found
}

func (p *Profile) Roots() []*Function {

	roots := []*Function{}

	for _, fn := range p.functions {
		if fn.Called == 0 {
			roots = append(roots, fn)
		}
	}

	return roots
}
