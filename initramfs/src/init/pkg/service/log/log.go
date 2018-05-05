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

func Get(name string) chunker.Chunker {
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
	c := chunker.NewDefaultChunker(l.Path)
	return c.Read(ctx)
}
