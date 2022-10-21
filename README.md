# gonx

`gonx` is Nginx access log reader library for `Go`. In fact you can use it for any format.


## Usage

The library provides `Reader` type and two constructors for it.

Common constructor `NewReader` gets opened file (any `io.Reader` in fact) and log format of type `string` as argumets.
[Format](#format) is in form os nginx `log_format` string.

```go
reader := gonx.NewReader(file, format)
```

`NewNginxReader` provides more magic. It gets log file `io.Reader`, nginx config file `io.Reader`
and `log_format` name `string` as a third. The actual format for `Parser` will be extracted from
given nginx config.

```go
reader := gonx.NewNginxReader(file, nginxConfig, format_name)
```

`Reader` implements `io.Reader`. Here is example usage

```go
for {
	rec, err := reader.Read()
	if err == io.EOF {
		break
	}
	// Process the record... e.g.
}
```

`NewMapReduce` Group informations that you need from `log_format`, 1000 is the number of capacity of concurrency goroutines

```go
	parser := gonx.NewParser(format)
	reducer := gonx.NewGroupBy(
		[]string{"http_referer"},
		&gonx.Sum{[]string{"body_bytes_sent"}},
		&gonx.Count{},
	)

	output := gonx.NewMapReduce(gz, parser, reducer, 1000)
	for res := range output.Wait() {
		fmt.Printf("Parsed entry: %+v - filename: %s\n", res, filename)
	}
```


See more examples in `example/*.go` sources.


## Format

As I said above this library is primary for nginx access log parsing, but it can be configured to parse any
other format. `NewReader` accepts `format` argument, it will be transformed to regular expression and used
for log line by line parsing. Format is nginx-like, here is example

	`$remote_addr [$time_local] "$request"`

It should contain variables in form `$name`. The regular expression will be created using this string
format representation

	`^(?P<remote_addr>[^ ]+) \[(?P<time_local>[^]]+)\] "(?P<request>[^"]+)"$`

`Reader.Read` returns a record of type `Entry` (which is customized `map[string][string]`). For this example
the returned record map will contain `remote_addr`, `time_local` and `request` keys filled with parsed values.


## Changelog

All major changes will be noticed in [release notes](https://github.com/luk4z7/gonx/releases).


## Contributing

Fork the repo, create a feature branch then send me pull request. Feel free to create new issues
