package log

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"sync"

	"github.com/autonomy/dianemo/initramfs/src/init/pkg/chunker"
)

var instance = map[string]*Log{}
var mu = &sync.Mutex{}

// Log represents the log of a service. It supports streaming of the contents of
// the log file by way of implementing the chunker.Chunker interface.
type Log struct {
	Name        string
	Path        string
	writeCloser io.WriteCloser
}

// New initializes and registers a log for a service.
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

// Chunker returns a chunker.Chunker implementation.
func Chunker(name string) chunker.Chunker {
	if l, ok := instance[name]; ok {
		return l
	}

	return nil
}

// Write implements io.WriteCloser.
func (l *Log) Write(p []byte) (n int, err error) {
	return l.writeCloser.Write(p)
}

// Close implements io.WriteCloser.
func (l *Log) Close() error {
	return l.writeCloser.Close()
}

// Read implements chunker.Chunker.
func (l *Log) Read(ctx context.Context) <-chan []byte {
	c := chunker.NewDefaultChunker(l.Path)
	return c.Read(ctx)
}
