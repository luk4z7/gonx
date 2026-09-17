package gonx

import (
	"regexp"
	"strings"
	"testing"

	"github.com/coregx/coregex"
	. "github.com/smartystreets/goconvey/convey"
)

func TestRegexEngine(t *testing.T) {
	Convey("Test regex engine selection", t, func() {
		format := "$remote_addr [$time_local] \"$request\" $status"
		line := `89.234.89.123 [08/Nov/2013:13:39:18 +0000] "GET /api/foo/bar HTTP/1.1" 200`
		expected := NewEntry(Fields{
			"remote_addr": "89.234.89.123",
			"time_local":  "08/Nov/2013:13:39:18 +0000",
			"request":     "GET /api/foo/bar HTTP/1.1",
			"status":      "200",
		})

		Convey("Default constructor uses the standard library", func() {
			parser := NewParser(format)
			So(parser.engine, ShouldResemble, StdRegex)
			_, isStd := parser.regexp.(*regexp.Regexp)
			So(isStd, ShouldBeTrue)
		})

		Convey("WithRegexEngine switches to coregex", func() {
			parser := NewParser(format, WithRegexEngine(CoreRegex))
			So(parser.engine, ShouldResemble, CoreRegex)
			_, isCore := parser.regexp.(*coregex.Regex)
			So(isCore, ShouldBeTrue)
		})

		Convey("A nil engine keeps the default", func() {
			parser := NewParser(format, WithRegexEngine(nil))
			So(parser.engine, ShouldResemble, StdRegex)
		})

		Convey("Both engines compile the same pattern", func() {
			So(
				NewParser(format, WithRegexEngine(CoreRegex)).regexp.String(),
				ShouldEqual,
				NewParser(format).regexp.String(),
			)
		})

		Convey("Both engines parse a line the same way", func() {
			for _, opts := range [][]Option{nil, {WithRegexEngine(CoreRegex)}} {
				entry, err := NewParser(format, opts...).ParseString(line)
				So(err, ShouldBeNil)
				So(entry, ShouldResemble, expected)
			}
		})

		Convey("Concatenated fields agree across engines", func() {
			// Exercises the placeholder replacement, whose template must stay in
			// the bare `$1` form that both engines expand.
			concatenated := `$remote_addr [$time_local] "$host$request_uri$demo" $status`
			So(
				NewParser(concatenated, WithRegexEngine(CoreRegex)).regexp.String(),
				ShouldEqual,
				`^(?P<remote_addr>[^ ]*) \[(?P<time_local>[^]]*)\] "(?P<host>[^"]*)(?P<request_uri>[^"]*)(?P<demo>[^"]*)" (?P<status>[^ ]*)`,
			)
			So(
				NewParser(concatenated, WithRegexEngine(CoreRegex)).regexp.String(),
				ShouldEqual,
				NewParser(concatenated).regexp.String(),
			)
		})

		Convey("Option order does not matter", func() {
			retry := "$remote_addr [$time_local] \"$request\""
			for _, opts := range [][]Option{
				{WithRegexEngine(CoreRegex), WithRetry(retry)},
				{WithRetry(retry), WithRegexEngine(CoreRegex)},
			} {
				parser := NewParser(format, opts...)
				So(parser.regexpRetry, ShouldNotBeNil)
				_, isCore := parser.regexpRetry.(*coregex.Regex)
				So(isCore, ShouldBeTrue)
			}
		})

		Convey("Retry falls back to the second format", func() {
			parser := NewParser(format, WithRetry("$remote_addr [$time_local]"), WithRegexEngine(CoreRegex))
			entry, err := parser.ParseString(`89.234.89.123 [08/Nov/2013:13:39:18 +0000]`)
			So(err, ShouldBeNil)
			addr, err := entry.Field("remote_addr")
			So(err, ShouldBeNil)
			So(addr, ShouldEqual, "89.234.89.123")
		})

		Convey("Sub parsers use the selected engine", func() {
			parser := NewParser(format, WithRegexEngine(CoreRegex))
			parser.AddSubParser(
				map[string]string{"request": `/api/[^ ]*`},
				map[string]map[string]string{},
				map[string]MatchKind{"request": Match},
			)
			So(parser.subParser, ShouldHaveLength, 1)
			_, isCore := parser.subParser[0].regexp.(*coregex.Regex)
			So(isCore, ShouldBeTrue)

			entry, err := parser.ParseString(line)
			So(err, ShouldBeNil)
			request, err := entry.Field("request")
			So(err, ShouldBeNil)
			So(request, ShouldEqual, "/api/foo/bar")
		})

		Convey("NewNginxParser honours the engine option", func() {
			conf := strings.NewReader(`
				http {
					log_format   main     '$remote_addr - $remote_user [$time_local] '
										'"$request" $status';
				}
			`)
			parser, err := NewNginxParser(conf, "main", WithRegexEngine(CoreRegex))
			So(err, ShouldBeNil)
			So(parser.engine, ShouldResemble, CoreRegex)
			_, isCore := parser.regexp.(*coregex.Regex)
			So(isCore, ShouldBeTrue)
		})
	})
}
