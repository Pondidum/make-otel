package main

import (
	"bufio"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/exp/slices"
)

type lineParser struct {
	stream     *bufio.Scanner
	line       string
	eof        bool
	lineNumber int
}

/*
scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
*/

func NewLineParser(r io.Reader) *lineParser {
	return &lineParser{
		stream:     bufio.NewScanner(r),
		line:       "",
		eof:        false,
		lineNumber: 0,
	}
}

func (p *lineParser) read() {
	if !p.stream.Scan() {
		p.line = ""
		p.eof = true
	} else {
		p.lineNumber++
	}
	p.line = strings.TrimSpace(p.stream.Text())
}

func (p *lineParser) ReadLine() {
	for {
		p.read()
		if p.eof || !strings.HasPrefix(p.Lookahead(), "#") {
			break
		}
	}
}

func (p *lineParser) Lookahead() string {
	return p.line
}

func (p *lineParser) Consume() string {
	l := p.line
	p.ReadLine()
	return l
}

func (p *lineParser) Eof() bool {
	return p.eof
}

type callgrindParser struct {
	*lineParser

	profile *Profile

	position_ids map[string]string
	positions    map[string]string

	num_positions  int
	cost_positions []string
	last_positions []int64

	num_events  int
	cost_events []string
}

func NewCallgrindParser(r io.Reader) *callgrindParser {
	return &callgrindParser{
		lineParser: NewLineParser(r),
		profile: &Profile{
			functions: map[string]*Function{},
			samples:   0,
		},

		positions:    map[string]string{},
		position_ids: map[string]string{},

		num_positions:  1,
		cost_positions: []string{"line"},
		last_positions: []int64{0},

		num_events:  0,
		cost_events: []string{},
	}
}

func (p *callgrindParser) parse() {
	p.ReadLine()

	p.parse_key("version")
	p.parse_key("creator")

	for p.parse_part() {
	}

	if !p.Eof() {
		panic("expected end of file: " + p.Lookahead())
	}
}

func (p *callgrindParser) parse_part() bool {
	if !p.parse_header_line() {
		return false
	}
	for p.parse_header_line() {

	}

	if !p.parse_body_line() {
		return false
	}
	for p.parse_body_line() {

	}

	return true
}

func (p *callgrindParser) parse_header_line() bool {
	return p.parse_empty() ||
		p.parse_comment() ||
		p.parse_part_detail() ||
		p.parse_description() ||
		p.parse_event_specification() ||
		p.parse_cost_line_def() ||
		p.parse_cost_summary()
}

func (p *callgrindParser) parse_empty() bool {
	if p.Eof() {
		return false
	}

	if line := p.Lookahead(); line != "" {
		return false
	}

	p.Consume()
	return true
}

func (p *callgrindParser) parse_comment() bool {
	if line := p.Lookahead(); strings.HasPrefix(line, "#") {
		p.Consume()
		return true
	}

	return false
}

func (p *callgrindParser) parse_part_detail() bool {
	k, _ := p.parse_keys("cmd", "pid", "thread", "part")
	return k != ""
}

func (p *callgrindParser) parse_description() bool {
	return p.parse_key("desc") != ""
}

func (p *callgrindParser) parse_event_specification() bool {
	return p.parse_key("event") != ""
}

func (p *callgrindParser) parse_cost_line_def() bool {
	key, v := p.parse_keys("events", "positions")
	if key == "" {
		return false
	}

	items := strings.Fields(v)
	if key == "events" {
		p.num_events = len(items)
		p.cost_events = items
	}

	if key == "positions" {
		p.num_positions = len(items)
		p.cost_positions = items
		p.last_positions = make([]int64, len(items))
	}

	return true
}

func (p *callgrindParser) parse_cost_summary() bool {
	key, _ := p.parse_keys("summary", "totals")
	return key != ""
}

func (p *callgrindParser) parse_body_line() bool {
	return p.parse_empty() ||
		p.parse_comment() ||
		p.parse_cost_line(0) ||
		p.parse_position_spec() ||
		p.parse_association_spec()
}

var subposition = `(0x[0-9a-fA-F]+|\d+|\+\d+|-\d+|\*)`
var costRx = regexp.MustCompile(`^` + subposition + `(` + subposition + `)*( +\d+)*$`)

func (p *callgrindParser) parse_cost_line(calls int) bool {
	line := p.Lookahead()
	if mo := costRx.MatchString(line); !mo {
		return false
	}

	fn := p.get_function()

	if calls == 0 {
		if x, found := p.positions["ob"]; found {
			p.positions["cob"] = x
		}
	}

	values := strings.Fields(line)
	if len(values) > p.num_positions+p.num_events {
		panic("too many values on line " + line)
	}

	// todo: off by one?
	positions := values[:p.num_positions]

	for i := 0; i < p.num_positions; i++ {
		position := positions[i]
		value := int64(0)

		if position == "*" {
			value = p.last_positions[i]
		} else if position[0] == '-' || position[0] == '+' {
			i, _ := strconv.ParseInt(position, 0, 64)
			value = p.last_positions[i] + i
		} else if strings.HasPrefix(position, "0x") {
			value, _ = strconv.ParseInt(position, 0, 64)
		} else {
			value, _ = strconv.ParseInt(position, 0, 64)
		}

		p.last_positions[i] = value
	}

	eventData := values[p.num_positions:]
	eventData = append(eventData, make([]string, p.num_events-len(eventData))...)

	events := make([]float64, len(eventData))
	for i, e := range eventData {
		events[i], _ = strconv.ParseFloat(e, 32)
	}

	if calls == 0 {
		fn.samples += events[0]
		p.profile.samples += events[0]
	} else {
		callee := p.get_callee()
		callee.called += calls

		call, found := fn.calls[callee.id]
		if !found {
			call = &Call{
				callee_id: callee.id,
				calls:     calls,
				samples:   events[0],
			}
			fn.add_call(call)
		} else {
			call.calls += calls
			call.samples += events[0]
		}
	}

	p.Consume()
	return true

}

var positionRx = regexp.MustCompile(`^(?P<position>[cj]?(?:ob|fl|fi|fe|fn))=\s*(?:\((?P<id>\d+)\))?(?:\s*(?P<name>.+))?`)
var positionTableMap = map[string]string{
	"ob":  "ob",
	"fl":  "fl",
	"fi":  "fl",
	"fe":  "fl",
	"fn":  "fn",
	"cob": "ob",
	"cfl": "fl",
	"cfi": "fl",
	"cfe": "fl",
	"cfn": "fn",
	"jfi": "fl",
}
var positionMap = map[string]string{
	"ob":  "ob",
	"fl":  "fl",
	"fi":  "fl",
	"fe":  "fl",
	"fn":  "fn",
	"cob": "cob",
	"cfl": "cfl",
	"cfi": "cfl",
	"cfe": "cfl",
	"cfn": "cfn",
	"jfi": "jfi",
}

func (p *callgrindParser) parse_position_spec() bool {
	line := p.Lookahead()

	if strings.HasPrefix(line, "jump=") || strings.HasPrefix(line, "jcnd=") {
		p.Consume()
		return true
	}

	groups := positionRx.FindStringSubmatch(line)
	if len(groups) == 0 {
		return false
	}

	position := groups[positionRx.SubexpIndex("position")]
	id := groups[positionRx.SubexpIndex("id")]
	name := groups[positionRx.SubexpIndex("name")]

	if id != "" {
		table := positionTableMap[position]
		if name != "" {
			p.position_ids[table+":"+id] = name
		} else {
			name = get(p.position_ids, table+":"+id, "")
		}
	}
	p.positions[positionMap[position]] = name

	p.Consume()
	return true
}

func (p *callgrindParser) parse_association_spec() bool {
	line := p.Lookahead()
	if !strings.HasPrefix(line, "calls=") {
		return false
	}

	values := strings.Fields(strings.TrimPrefix(line, "calls="))
	calls, _ := strconv.Atoi(values[0])
	// call_position := values[1:]

	p.Consume()
	p.parse_cost_line(calls)

	return true
}

func (p *callgrindParser) parse_key(key string) string {
	k, v := p.parse_keys(key)
	if k == "" {
		return ""
	}

	return v
}

func (p *callgrindParser) parse_keys(keys ...string) (string, string) {
	line := p.Lookahead()

	if !_key_re.MatchString(line) {
		return "", ""
	}

	parts := strings.Split(line, ":")
	key := parts[0]
	value := parts[1]

	if !slices.Contains(keys, key) {
		return "", ""
	}

	value = strings.TrimSpace(value)

	p.Consume()
	return key, value
}

func (p *callgrindParser) get_callee() *Function {
	module := get(p.positions, "cob", "")
	filename := get(p.positions, "cfi", "")
	function := get(p.positions, "cfn", "")

	return p.make_function(module, filename, function)
}

func (p *callgrindParser) get_function() *Function {
	module := get(p.positions, "ob", "")
	filename := get(p.positions, "fl", "")
	function := get(p.positions, "fn", "")

	return p.make_function(module, filename, function)
}

func (p callgrindParser) make_function(module, filename, name string) *Function {
	id := name
	if fn, ok := p.profile.getFunction(id); ok {
		return fn
	}

	fn := &Function{
		id:      id,
		name:    name,
		called:  0,
		samples: 0,
		calls:   map[string]*Call{},
	}
	if module != "" {
		fn.module = path.Base(module)
	}

	p.profile.add_function(fn)
	return fn
}

func get(m map[string]string, key, defaultValue string) string {
	if v, found := m[key]; found {
		return v
	}
	return defaultValue
}

var _key_re = regexp.MustCompile(`^(\w+):`)

type Function struct {
	id     string
	name   string
	module string
	called int

	samples float64
	calls   map[string]*Call
}

func (f *Function) add_call(c *Call) {
	f.calls[c.callee_id] = c
}

type Call struct {
	callee_id string
	ratio     float64
	weight    float64

	calls   int
	samples float64
}

type Profile struct {
	functions map[string]*Function
	samples   float64
}

func (p *Profile) add_function(f *Function) {
	p.functions[f.id] = f
}

func (p *Profile) getFunction(id string) (*Function, bool) {
	f, found := p.functions[id]
	return f, found
}
