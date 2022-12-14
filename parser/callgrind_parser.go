package parser

import (
	"fmt"
	"io"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type callgrindParser struct {
	*lineParser

	profile *Profile

	position_ids map[string]string
	positions    map[string]string

	positionCount int
	costPositions []string
	lastPositions []int64

	eventCount int
	costEvents []string
}

func NewCallgrindParser(r io.Reader) *callgrindParser {
	return &callgrindParser{
		lineParser: NewLineParser(r),
		profile: &Profile{
			functions: map[string]*Function{},
		},

		positions:    map[string]string{},
		position_ids: map[string]string{},

		positionCount: 1,
		costPositions: []string{"line"},
		lastPositions: []int64{0},

		eventCount: 0,
		costEvents: []string{},
	}
}

func (p *callgrindParser) Parse() (*Profile, error) {
	p.ReadLine()

	p.parseKey("version")

	if val, found := p.parseKey("creator"); found {
		p.profile.Creator = val
	}

	for p.parsePart() {
	}

	if !p.Eof() {
		return nil, fmt.Errorf("expected to be at end of file, but had line left: %s", p.Line())
	}

	return p.profile, nil
}

func (p *callgrindParser) parsePart() bool {
	if !p.parseHeaderLine() {
		return false
	}
	for p.parseHeaderLine() {
	}

	if !p.parseBodyLine() {
		return false
	}
	for p.parseBodyLine() {
	}

	return true
}

func (p *callgrindParser) parseHeaderLine() bool {
	return p.parseEmpty() ||
		p.parseComment() ||
		p.parsePartDetail() ||
		p.parseCommand() ||
		p.parseDescription() ||
		p.parseEventSpecification() ||
		p.parseCostLineDefinition() ||
		p.parseCostSummary()
}

func (p *callgrindParser) parseEmpty() bool {
	if p.Eof() {
		return false
	}

	if line := p.Line(); line != "" {
		return false
	}

	p.Consume()
	return true
}

func (p *callgrindParser) parseComment() bool {
	if line := p.Line(); strings.HasPrefix(line, "#") {
		p.Consume()
		return true
	}

	return false
}

func (p *callgrindParser) parsePartDetail() bool {
	if _, found := p.parseKey("pid"); found {
		return true
	}

	if _, found := p.parseKey("thread"); found {
		return true
	}

	if _, found := p.parseKey("part"); found {
		return true
	}

	return false
}

func (p *callgrindParser) parseCommand() bool {
	val, found := p.parseKey("cmd")
	if found {
		p.profile.Command = val
	}

	return val != ""
}

func (p *callgrindParser) parseDescription() bool {
	_, found := p.parseKey("desc")
	return found
}

func (p *callgrindParser) parseEventSpecification() bool {
	_, found := p.parseKey("event")
	return found
}

func (p *callgrindParser) parseCostLineDefinition() bool {

	if value, found := p.parseKey("events"); found {
		items := strings.Fields(value)
		p.eventCount = len(items)
		p.costEvents = items
		return true
	}

	if value, found := p.parseKey("positions"); found {
		items := strings.Fields(value)
		p.positionCount = len(items)
		p.costPositions = items
		p.lastPositions = make([]int64, len(items))
		return true
	}

	return false
}

func (p *callgrindParser) parseCostSummary() bool {
	var fields []string

	if value, found := p.parseKey("summary"); found {
		fields = strings.Fields(value)
	} else if value, found := p.parseKey("totals"); found {
		fields = strings.Fields(value)
	} else {
		return false
	}

	if totalCost, err := strconv.ParseFloat(fields[0], 32); err == nil {
		p.profile.TotalCost = buildCost(totalCost, p.costEvents[0])
	}

	return true
}

func (p *callgrindParser) parseBodyLine() bool {
	return p.parseEmpty() ||
		p.parseComment() ||
		p.parseCostLine(0) ||
		p.parsePositionSpec() ||
		p.parseAssociationSpec()
}

var subposition = `(0x[0-9a-fA-F]+|\d+|\+\d+|-\d+|\*)`
var costRx = regexp.MustCompile(`^` + subposition + `(` + subposition + `)*( +\d+)*$`)

func (p *callgrindParser) parseCostLine(calls int) bool {
	line := p.Line()
	if mo := costRx.MatchString(line); !mo {
		return false
	}

	fn := p.getFunction()

	if calls == 0 {
		if x, found := p.positions["ob"]; found {
			p.positions["cob"] = x
		}
	}

	values := strings.Fields(line)
	if len(values) > p.positionCount+p.eventCount {
		panic("too many values on line " + line)
	}

	positions := values[:p.positionCount]

	for i := 0; i < p.positionCount; i++ {
		position := positions[i]
		value := int64(0)

		if position == "*" {
			value = p.lastPositions[i]
		} else if position[0] == '-' || position[0] == '+' {
			i, _ := strconv.ParseInt(position, 0, 64)
			value = p.lastPositions[i] + i
		} else if strings.HasPrefix(position, "0x") {
			value, _ = strconv.ParseInt(position, 0, 64)
		} else {
			value, _ = strconv.ParseInt(position, 0, 64)
		}

		p.lastPositions[i] = value
	}

	fn.LineNumber = p.lastPositions[0]

	eventData := values[p.positionCount:]
	eventData = append(eventData, make([]string, p.eventCount-len(eventData))...)

	events := make([]float64, len(eventData))
	for i, e := range eventData {
		events[i], _ = strconv.ParseFloat(e, 32)
	}

	cost := buildCost(events[0], p.costEvents[0])
	if calls == 0 {
		fn.Cost += cost
	} else {
		callee := p.getCallee()
		callee.Called += calls

		call, found := fn.calls[callee.ID]
		if !found {
			call = &Call{
				CalleeId: callee.ID,
				Calls:    calls,
				Cost:     cost,
			}
			fn.addCall(call)
		} else {
			call.Calls += calls
			call.Cost += cost
		}
	}

	p.Consume()
	return true
}

func buildCost(cost float64, costDefinition string) time.Duration {

	multiplier := time.Duration(1)

	if strings.HasSuffix(costDefinition, "usec") {
		v, _ := strconv.Atoi(strings.TrimSuffix(costDefinition, "usec"))
		multiplier = time.Duration(v) * time.Microsecond
	}

	return time.Duration(cost * float64(multiplier))
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

func (p *callgrindParser) parsePositionSpec() bool {
	line := p.Line()

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

func (p *callgrindParser) parseAssociationSpec() bool {
	line := p.Line()
	if !strings.HasPrefix(line, "calls=") {
		return false
	}

	values := strings.Fields(strings.TrimPrefix(line, "calls="))
	calls, _ := strconv.Atoi(values[0])
	// call_position := values[1:]

	p.Consume()
	p.parseCostLine(calls)

	return true
}

var keyRx = regexp.MustCompile(`^(\w+):`)

func (p *callgrindParser) parseKey(key string) (string, bool) {
	line := p.Line()

	if !keyRx.MatchString(line) {
		return "", false
	}

	parts := strings.Split(line, ":")
	k := parts[0]
	v := parts[1]

	if k != key {
		return "", false
	}

	p.Consume()
	return strings.TrimSpace(v), true
}

func (p *callgrindParser) getCallee() *Function {
	module := get(p.positions, "cob", "")
	filename := get(p.positions, "cfi", "")
	function := get(p.positions, "cfn", "")

	return p.makeFunction(module, filename, function)
}

func (p *callgrindParser) getFunction() *Function {
	module := get(p.positions, "ob", "")
	filename := get(p.positions, "fl", "")
	function := get(p.positions, "fn", "")

	return p.makeFunction(module, filename, function)
}

func (p callgrindParser) makeFunction(module, filename, name string) *Function {
	id := name
	if fn, ok := p.profile.GetFunction(id); ok {
		return fn
	}

	fn := NewFunction(id, name)
	if module != "" {
		fn.Module = path.Base(module)
	}

	p.profile.addFunction(fn)
	return fn
}
