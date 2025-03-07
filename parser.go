package gonx

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
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
		regexp      *regexp.Regexp
		regexpRetry *regexp.Regexp
		subParser   []*SubParser
	}

	Option func(p *Parser)

	MatchKind int

	SubParser struct {
		field       string
		regexp      *regexp.Regexp
		regexpInner map[string]*regexp.Regexp
		match       MatchKind
	}
)

func WithRetry(format string) Option {
	return func(p *Parser) {
		p.regexpRetry = prepareFormat(format)
	}
}

// NewParser returns a new Parser, use given log format to create its internal
// strings parsing regexp.
func NewParser(format string, opts ...Option) *Parser {
	parser := &Parser{
		format:    format,
		regexp:    prepareFormat(format),
		subParser: []*SubParser{},
	}

	for _, opt := range opts {
		opt(parser)
	}

	return parser
}

func prepareFormat(format string) *regexp.Regexp {
	// First split up multiple concatenated fields with placeholder
	placeholder := " _PLACEHOLDER___ "
	preparedFormat := format
	concatenatedRe := regexp.MustCompile(`[A-Za-z0-9_]\$[A-Za-z0-9_]`)
	for concatenatedRe.MatchString(preparedFormat) {
		preparedFormat = regexp.MustCompile(`([A-Za-z0-9_])\$([A-Za-z0-9_]+)(\\?([^$\\A-Za-z0-9_]))`).ReplaceAllString(
			preparedFormat, fmt.Sprintf("${1}${3}%s$$${2}${3}", placeholder),
		)
	}

	// Second replace each fields to regexp grouping
	quotedFormat := regexp.QuoteMeta(preparedFormat + " ")
	re := regexp.MustCompile(`\\\$([A-Za-z0-9_]+)(?:\\\$[A-Za-z0-9_])*(\\?([^$\\A-Za-z0-9_]))`).ReplaceAllString(
		quotedFormat, "(?P<$1>[^$3]*)$2")

	// Finally remove placeholder
	re = regexp.MustCompile(fmt.Sprintf(".%s", placeholder)).ReplaceAllString(re, "")

	return regexp.MustCompile(fmt.Sprintf("^%v", strings.Trim(re, " ")))
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
		re := regexp.MustCompile(v)
		res := make(map[string]*regexp.Regexp)

		for k, v := range inner[k] {
			res[k] = regexp.MustCompile(v)
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
func NewNginxParser(conf io.Reader, name string) (parser *Parser, err error) {
	scanner := bufio.NewScanner(conf)
	re := regexp.MustCompile(fmt.Sprintf(`^\s*log_format\s+%v\s+(.+)\s*$`, name))
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
		re = regexp.MustCompile(`^\s*(.*?)\s*(;|$)`)
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
	parser = NewParser(format)
	return
}
