package gonx

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

const (
	Match MatchKind = iota
	SubMatch
)

type (
	// StringParser is the interface that wraps the ParseString method.
	StringParser interface {
		ParseString(line string) (entry *Entry, err error)
	}

	// Parser is a log record parser. Use specific constructors to initialize it.
	Parser struct {
		format      string
		retryFormat string
		engine      RegexEngine
		regexp      Regexp
		regexpRetry Regexp
		subParser   []*SubParser
	}

	Option func(p *Parser)

	MatchKind int

	SubParser struct {
		field       string
		regexp      Regexp
		regexpInner map[string]Regexp
		match       MatchKind
	}
)

func WithRetry(format string) Option {
	return func(p *Parser) {
		p.retryFormat = format
	}
}

// WithRegexEngine sets the regular expression implementation used to compile
// every pattern the parser needs. Without it parsers use StdRegex, the
// standard library engine.
func WithRegexEngine(engine RegexEngine) Option {
	return func(p *Parser) {
		if engine != nil {
			p.engine = engine
		}
	}
}

// newParser applies opts over the defaults without compiling anything, so the
// engine is known before the first pattern is compiled.
func newParser(opts ...Option) *Parser {
	parser := &Parser{
		engine:    StdRegex,
		subParser: []*SubParser{},
	}

	for _, opt := range opts {
		opt(parser)
	}

	return parser
}

// NewParser returns a new Parser, use given log format to create its internal
// strings parsing regexp.
func NewParser(format string, opts ...Option) *Parser {
	parser := newParser(opts...)
	parser.format = format
	parser.regexp = prepareFormat(parser.engine, format)

	if parser.retryFormat != "" {
		parser.regexpRetry = prepareFormat(parser.engine, parser.retryFormat)
	}

	return parser
}

func prepareFormat(engine RegexEngine, format string) Regexp {
	// First split up multiple concatenated fields with placeholder.
	// Group references stay in the bare `$1` form: coregex leaves the `${1}`
	// brace form unexpanded, while both engines agree on the bare one.
	placeholder := " _PLACEHOLDER___ "
	preparedFormat := format
	concatenatedRe := engine.MustCompile(`[A-Za-z0-9_]\$[A-Za-z0-9_]`)
	for concatenatedRe.MatchString(preparedFormat) {
		preparedFormat = engine.MustCompile(`([A-Za-z0-9_])\$([A-Za-z0-9_]+)(\\?([^$\\A-Za-z0-9_]))`).ReplaceAllString(
			preparedFormat, fmt.Sprintf("$1$3%s$$$2$3", placeholder),
		)
	}

	// Second replace each fields to regexp grouping
	quotedFormat := engine.QuoteMeta(preparedFormat + " ")
	re := engine.MustCompile(`\\\$([A-Za-z0-9_]+)(?:\\\$[A-Za-z0-9_])*(\\?([^$\\A-Za-z0-9_]))`).ReplaceAllString(
		quotedFormat, "(?P<$1>[^$3]*)$2")

	// Finally remove placeholder
	re = engine.MustCompile(fmt.Sprintf(".%s", placeholder)).ReplaceAllString(re, "")

	return engine.MustCompile(fmt.Sprintf("^%v", strings.Trim(re, " ")))
}

func (p *Parser) AddSubParser(values map[string]string, inner map[string]map[string]string, matching ...interface{}) {
	var subParser []*SubParser

	match := make(map[string]MatchKind)
	for _, v := range matching {
		if kind, ok := v.(map[string]MatchKind); ok {
			match = kind
		}
	}

	for k, v := range values {
		re := p.engine.MustCompile(v)
		res := make(map[string]Regexp)

		for k, v := range inner[k] {
			res[k] = p.engine.MustCompile(v)
		}

		sub := &SubParser{
			field:       k,
			regexp:      re,
			regexpInner: res,
			match:       match[k],
		}

		subParser = append(subParser, sub)
	}

	p.subParser = subParser
}

// ParseString parses a log file line using internal format regexp. If a line
// does not match the given format an error will be returned.
func (parser *Parser) ParseString(line string) (entry *Entry, err error) {
	re := parser.regexp
	fields := re.FindStringSubmatch(line)
	if fields == nil {
		if parser.regexpRetry != nil {
			re = parser.regexpRetry
			fields = re.FindStringSubmatch(line)
		}

		if fields == nil {
			err = fmt.Errorf("access log line '%v' does not match given format '%v'", line, re)
			return
		}
	}

	// Iterate over subexp found and fill the map record
	entry = NewEmptyEntry()
	for i, name := range re.SubexpNames() {
		if i == 0 {
			continue
		}

		// custom parser for a key that's returned from a line
		for _, v := range parser.subParser {

			if v.field == name {
				if v.match == Match {
					if match := v.regexp.FindString(fields[i]); match != "" {
						fields[i] = match
					}
				}

				if v.match == SubMatch {
					groups := v.regexp.FindStringSubmatch(fields[i])

					if len(groups) >= 2 {
						result := groups[1]

						if _, ok := v.regexpInner[result]; ok {
							innerMatch := v.regexpInner[result].FindStringSubmatch(fields[i])
							if len(innerMatch) >= 2 {
								fields[i] = innerMatch[1]
							}
						} else {
							fields[i] = result
						}
					}
				}
			}
		}

		entry.SetField(name, fields[i])
	}

	return
}

// NewNginxParser parses the nginx conf file to find log_format with the given
// name and returns a parser for this format. It returns an error if cannot find
// the given log format.
func NewNginxParser(conf io.Reader, name string, opts ...Option) (parser *Parser, err error) {
	engine := newParser(opts...).engine
	scanner := bufio.NewScanner(conf)
	re := engine.MustCompile(fmt.Sprintf(`^\s*log_format\s+%v\s+(.+)\s*$`, name))
	found := false
	var format string
	for scanner.Scan() {
		var line string
		if !found {
			// Find a log_format definition
			line = scanner.Text()
			formatDef := re.FindStringSubmatch(line)
			if formatDef == nil {
				continue
			}
			found = true
			line = formatDef[1]
		} else {
			line = scanner.Text()
		}
		// Look for a definition end
		re = engine.MustCompile(`^\s*(.*?)\s*(;|$)`)
		lineSplit := re.FindStringSubmatch(line)
		if l := len(lineSplit[1]); l > 2 {
			format += lineSplit[1][1 : l-1]
		}
		if lineSplit[2] == ";" {
			break
		}
	}
	if !found {
		err = fmt.Errorf("`log_format %v` not found in given config", name)
	} else {
		err = scanner.Err()
	}
	parser = NewParser(format, opts...)
	return
}
