package main

import (
	"compress/gzip"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gitlab.globoi.com/webmedia/videos-edge/billing/gonx"
)

var format string

func init() {
	flag.StringVar(&format, "format", "$remote_addr [$time_local] \"$request\"", "Log format")
}

func main() {
	flag.Parse()

	var g sync.WaitGroup
	var list []string

	err := filepath.Walk(".",
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			list = append(list, path)
			return nil
		})
	if err != nil {
		panic(err)
	}

	for _, f := range list {
		if strings.Contains(f, ".log.gz") {
			f := f
			g.Add(1)

			go func(file string, wg *sync.WaitGroup) {
				err := readFile(f, wg)
				if err != nil {
					panic(err)
				}
			}(f, &g)
		}
	}

	g.Wait()
}

func readFile(filename string, wg *sync.WaitGroup) error {
	defer wg.Done()

	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	gz, err := gzip.NewReader(file)
	if err != nil {
		return err
	}
	defer gz.Close()

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

	return nil
}
