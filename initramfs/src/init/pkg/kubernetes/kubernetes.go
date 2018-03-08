package kubernetes

import (
	"io"
	"net/http"
	"os"
	"strconv"
)

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
