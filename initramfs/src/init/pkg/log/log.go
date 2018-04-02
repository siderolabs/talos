package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
)

var instance = map[string]*Log{}
var mu = &sync.Mutex{}

type Log struct {
	Name        string
	Path        string
	writeCloser io.WriteCloser
}

func New(name string) (*Log, error) {
	if l, ok := instance[name]; ok {
		return l, nil
	}
	logpath := path.Join("/var/log", name)
	w, err := os.Create(logpath)
	if err != nil {
		return nil, fmt.Errorf("create log file: %s", err.Error())
	}

	l := &Log{
		Name:        name,
		Path:        logpath,
		writeCloser: w,
	}

	mu.Lock()
	instance[name] = l
	mu.Unlock()

	return l, nil
}

func Get(name string) *Log {
	if l, ok := instance[name]; ok {
		return l
	}

	return nil
}

func (l *Log) Write(p []byte) (n int, err error) {
	return l.writeCloser.Write(p)
}

func (l *Log) Close() error {
	return l.writeCloser.Close()
}

func (l *Log) Read(ctx context.Context) <-chan []byte {
	// Create a buffered channel of length 1.
	ch := make(chan []byte, 1)
	file, err := os.OpenFile(l.Path, os.O_RDONLY, 0)
	if err != nil {
		return nil
	}

	go func(ch chan []byte, f *os.File) {
		defer close(ch)
		defer f.Close()

		offset, err := f.Seek(0, io.SeekStart)
		if err != nil {
			return
		}
		buf := make([]byte, 1024, 1024)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			n, err := f.ReadAt(buf, offset)
			if err != nil {
				if err != io.EOF {
					fmt.Println("error reading log file: %s", err.Error())
					break
				}
			}
			offset += int64(n)
			if n != 0 {
				// Copy the buffer since we will modify it in the next loop.
				b := make([]byte, n)
				copy(b, buf[:n])
				ch <- b
			}
			// Clear the buffer.
			for i := 0; i < n; i++ {
				buf[i] = 0
			}
		}
	}(ch, file)

	return ch
}
