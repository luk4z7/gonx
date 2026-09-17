package gonx

import (
	"regexp"

	"github.com/coregx/coregex"
)

type (
	// Regexp is the subset of a compiled regular expression that gonx uses.
	// Both the standard library's *regexp.Regexp and coregex's *coregex.Regex
	// satisfy it.
	Regexp interface {
		FindString(s string) string
		FindStringSubmatch(s string) []string
		MatchString(s string) bool
		ReplaceAllString(src, repl string) string
		SubexpNames() []string
		String() string
	}

	// RegexEngine compiles the patterns a Parser needs. Pass one to a
	// constructor with WithRegexEngine; parsers default to StdRegex.
	RegexEngine interface {
		MustCompile(expr string) Regexp
		QuoteMeta(s string) string
	}

	stdEngine  struct{}
	coreEngine struct{}
)

var (
	// StdRegex is the default engine, backed by the standard library regexp.
	StdRegex RegexEngine = stdEngine{}

	// CoreRegex is backed by github.com/coregx/coregex.
	CoreRegex RegexEngine = coreEngine{}
)

func (stdEngine) MustCompile(expr string) Regexp { return regexp.MustCompile(expr) }

func (stdEngine) QuoteMeta(s string) string { return regexp.QuoteMeta(s) }

func (coreEngine) MustCompile(expr string) Regexp { return coregex.MustCompile(expr) }

func (coreEngine) QuoteMeta(s string) string { return coregex.QuoteMeta(s) }
