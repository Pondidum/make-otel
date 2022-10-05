package parser

type Profile struct {
	functions map[string]*Function
	samples   float64
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
