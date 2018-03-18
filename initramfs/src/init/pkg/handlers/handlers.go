package handlers

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"

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

	// File is found, create and send the correct headers

	// Get the Content-Type of the file
	// Create a buffer to store the header of the file in
	FileHeader := make([]byte, 512)
	// Copy the headers into the FileHeader buffer
	openFile.Read(FileHeader)
	// Get content type of file
	FileContentType := http.DetectContentType(FileHeader)

	// Get the file size
	FileStat, _ := openFile.Stat()
	FileSize := strconv.FormatInt(FileStat.Size(), 10)

	// Send the headers
	w.Header().Set("Content-Disposition", "attachment; filename=admin.conf")
	w.Header().Set("Content-Type", FileContentType)
	w.Header().Set("Content-Length", FileSize)

	// Send the file
	// We read 512 bytes from the file already so we reset the offset back to 0
	openFile.Seek(0, 0)
	io.Copy(w, openFile) // 'Copy' the file to the client
}
