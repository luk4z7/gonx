package gonx

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"sync"
)

type MapReduce struct {
	file    io.Reader
	parser  StringParser
	reducer Reducer
	limit   chan struct{}
	lines   chan string
	entries chan *Entry
	output  chan *Entry
	wg      *sync.WaitGroup
}

func NewMapReduce(file io.Reader, parser StringParser, reducer Reducer, limit int) *MapReduce {
	return &MapReduce{
		file:    file,
		parser:  parser,
		reducer: reducer,
		lines:   make(chan string),
		limit:   make(chan struct{}, limit),
		entries: make(chan *Entry, limit),
		output:  make(chan *Entry),
		wg:      new(sync.WaitGroup),
	}
}

func (m *MapReduce) Wait() chan *Entry {
	go func() {
		for line := range m.lines {
			m.limit <- struct{}{}
			m.wg.Add(1)

			go func(l string, g *sync.WaitGroup) {
				defer g.Done()

				entry, err := m.parser.ParseString(l)
				if err != nil {
					fmt.Println("Error on given format", "err", err)
					<-m.limit

					return
				}

				m.entries <- entry
				<-m.limit

			}(line, m.wg)
		}

		m.wg.Wait()
		close(m.entries)
	}()

	// Run reducer routine.
	go m.reducer.Reduce(m.entries, m.output)

	go func() {
		err := m.readWithReadLine(m.file, m.lines)
		if err != nil {
			fmt.Println("Error on read line", "err", err)
		}
	}()

	return m.output

}

func (m *MapReduce) readWithReadLine(file io.Reader, lines chan string) error {
	defer close(m.lines)

	reader := bufio.NewReader(file)
	for {
		line, err := m.readLine(reader)
		if err != nil {
			if err == io.EOF {
				break
			}

			return err
		}

		m.lines <- line
	}

	return nil
}

func (m *MapReduce) readLine(reader *bufio.Reader) (string, error) {
	line, isPrefix, err := reader.ReadLine()
	if err != nil {
		return "", err
	}
	if !isPrefix {
		return string(line), nil
	}

	var buffer bytes.Buffer
	_, err = buffer.Write(line)
	for isPrefix && err == nil {
		line, isPrefix, err = reader.ReadLine()
		if err == nil {
			_, err = buffer.Write(line)
		}
	}

	return buffer.String(), err
}
