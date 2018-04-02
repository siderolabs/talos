package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

	logstream "github.com/autonomy/dianemo/initramfs/src/init/pkg/log"
	"github.com/kubernetes-incubator/cri-o/client"
)

type containerRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Status string `json:"status"`
}

func ContainerHandleFunc(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/container/")
	cli, err := client.New("/var/run/crio/crio.sock")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	info, err := cli.ContainerInfo(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	logBytes, err := ioutil.ReadFile(info.LogPath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	js, err := json.Marshal(string(logBytes))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}

func KubeConfigHandleFunc(w http.ResponseWriter, r *http.Request) {
	openFile, err := os.Open("/etc/kubernetes/admin.conf")
	defer openFile.Close()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	FileHeader := make([]byte, 512)
	openFile.Read(FileHeader)
	FileContentType := http.DetectContentType(FileHeader)

	FileStat, _ := openFile.Stat()
	FileSize := strconv.FormatInt(FileStat.Size(), 10)

	w.Header().Set("Content-Disposition", "attachment; filename=admin.conf")
	w.Header().Set("Content-Type", FileContentType)
	w.Header().Set("Content-Length", FileSize)

	openFile.Seek(0, io.SeekStart)
	io.Copy(w, openFile)
}

func ProcessLogHandleFunc(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/logs/")
	stream := logstream.Get(name)

	if stream == nil {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(fmt.Sprintf("process not found: %s", name)))
		return
	}

	w.WriteHeader(http.StatusOK)
	w.Header().Set("Connection", "Keep-Alive")
	w.Header().Set("Transfer-Encoding", "chunked")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for data := range stream.Read(ctx) {
		if _, err := w.Write(data); err != nil {
			cancel()
			// Drain the channel
			continue
		}
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
	}
}
