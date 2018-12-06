package userdata

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestDownloadRetry(t *testing.T) {
	ts := testUDServer(t)
	defer ts.Close()

	_, err := Download(ts.URL)
	if err != nil {
		t.Error("Failed to download userdata", err)
	}
}

func testUDServer(t *testing.T) *httptest.Server {
	var count int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		log.Printf("Request %d\n", count)
		if count == 4 {
			f, err := os.Open("testdata/userdata.yaml")
			if err != nil {
				log.Println("failed to open")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			defer f.Close()
			b, err := ioutil.ReadAll(f)
			if err != nil {
				log.Println("failed to read")
				w.WriteHeader(http.StatusInternalServerError)
				return

			}
			w.Write(b)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}))

	return ts
}
