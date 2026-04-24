package client

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"strings"
)

// Decision is the parsed first-line verdict in decision mode.
type Decision string

const (
	DecisionYes     Decision = "yes"
	DecisionNo      Decision = "no"
	DecisionUnknown Decision = "unknown"
)

// processor is a streaming writer. In normal mode it forwards deltas
// verbatim (after the outer control-byte filter runs). In decision mode it
// buffers until the first \n, parses the verdict, skips an optional blank
// separator line, then streams the rest.
//
// It also accumulates the prose (everything that would be written to the
// final output, minus the decision line) so history.answer matches what
// the user saw on stdout.
type processor struct {
	w             io.Writer
	stderr        io.Writer
	decisionMode  bool
	phase         decisionPhase // only meaningful when decisionMode
	lineBuf       strings.Builder
	decision      Decision
	decisionKnown bool
	skipNextLF    bool // consume one trailing LF as the blank separator
	prose         strings.Builder
}

type decisionPhase int

const (
	phaseBufferLine1 decisionPhase = iota
	phaseSkipSeparator
	phaseStream
)

var firstWordRE = regexp.MustCompile(`[a-z]+`)

func newProcessor(w, stderr io.Writer, decisionMode bool) *processor {
	p := &processor{w: w, stderr: stderr, decisionMode: decisionMode}
	if decisionMode {
		p.phase = phaseBufferLine1
	} else {
		p.phase = phaseStream
	}
	return p
}

// Write processes a delta chunk. The returned error is whatever the
// underlying writer returns.
func (p *processor) Write(chunk []byte) (int, error) {
	n := len(chunk)
	if !p.decisionMode {
		if err := p.writeProse(chunk); err != nil {
			return 0, err
		}
		return n, nil
	}
	for len(chunk) > 0 {
		switch p.phase {
		case phaseBufferLine1:
			nl := bytes.IndexByte(chunk, '\n')
			if nl < 0 {
				p.lineBuf.Write(chunk)
				chunk = nil
				continue
			}
			p.lineBuf.Write(chunk[:nl])
			p.parseDecision()
			p.phase = phaseSkipSeparator
			p.skipNextLF = true
			chunk = chunk[nl+1:]

		case phaseSkipSeparator:
			// consume a single leading blank line if present: if the next
			// byte is \n, eat it; otherwise just move on.
			if p.skipNextLF && len(chunk) > 0 && chunk[0] == '\n' {
				chunk = chunk[1:]
			}
			p.skipNextLF = false
			p.phase = phaseStream

		case phaseStream:
			if err := p.writeProse(chunk); err != nil {
				return 0, err
			}
			chunk = nil
		}
	}
	return n, nil
}

// Close handles end-of-stream cleanup. In decision mode, if the stream
// ended before a newline the buffered line is still parsed as a verdict.
// It also ensures the output ends with a newline.
func (p *processor) Close() error {
	if p.decisionMode && p.phase == phaseBufferLine1 {
		p.parseDecision()
	}
	if p.prose.Len() == 0 || !strings.HasSuffix(p.prose.String(), "\n") {
		if _, err := p.w.Write([]byte{'\n'}); err != nil {
			return err
		}
	}
	return nil
}

// Decision returns the parsed verdict, and whether one was parsed at all.
// In normal mode, always returns "", false.
func (p *processor) Decision() (Decision, bool) {
	return p.decision, p.decisionKnown
}

// Prose returns the text that was (or would have been) written to stdout,
// minus the decision line. Used for history.
func (p *processor) Prose() string { return p.prose.String() }

func (p *processor) writeProse(chunk []byte) error {
	if _, err := p.w.Write(chunk); err != nil {
		return err
	}
	p.prose.Write(chunk)
	return nil
}

func (p *processor) parseDecision() {
	line := strings.ToLower(p.lineBuf.String())
	word := firstWordRE.FindString(line)
	switch word {
	case "yes":
		p.decision = DecisionYes
	case "no":
		p.decision = DecisionNo
	case "unknown":
		p.decision = DecisionUnknown
	default:
		p.decision = DecisionUnknown
		fmt.Fprintln(p.stderr, "qq: model didn't follow decision format, treating as unknown")
	}
	p.decisionKnown = true
}
