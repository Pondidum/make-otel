package parser

import (
	"bufio"
	"io"
	"strings"
)

type lineParser struct {
	stream     *bufio.Scanner
	line       string
	eof        bool
	lineNumber int
}

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
		if p.eof || !strings.HasPrefix(p.Line(), "#") {
			break
		}
	}
}

func (p *lineParser) Line() string {
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
