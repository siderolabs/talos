package baremetal

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
)

func TestUnTar(t *testing.T) {
	f, err := os.Open("testdata/talos_test.tar.gz")
	if err != nil {
		t.Error("Failed to open file", err)
	}

	// nolint: errcheck
	defer f.Close()

	out, err := ioutil.TempDir("", "testbaremetal")
	if err != nil {
		t.Error("Failed to open file", err)
	}

	// nolint: errcheck
	defer os.RemoveAll(out)
	err = untar(f, out)
	if err != nil {
		t.Error("Failed to untar file", err)
	}

	var files []os.FileInfo

	err = filepath.Walk(out, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Logf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		// Skip PWD
		if info.IsDir() && info.Name() == filepath.Base(out) {
			return nil
		}

		files = append(files, info)

		return nil
	})
	if err != nil {
		t.Error("Failed to walk dir", err)
	}

	expected := map[string]interface{}{"talos": nil, "talosdir": nil, "talosfile": nil}

	if len(files) != len(expected) {
		t.Errorf("Did not get back expected number of files - expected %d got %d", len(expected), len(files))
	}

	for _, file := range files {
		if _, ok := expected[file.Name()]; !ok {
			t.Errorf("Unexpected file %s", file.Name())
		}
	}

}

func TestNewDevice(t *testing.T) {
	dev, ts := newdev(t, constants.DataPartitionLabel)

	// nolint: errcheck
	defer ts.Close()
	defer os.RemoveAll(dev.MountBase)

	if err := dev.Install(); err != nil {
		t.Error("Install failed", err)
	}

	var files []os.FileInfo
	err := filepath.Walk(filepath.Join(dev.MountBase, dev.Label), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			t.Logf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		// Skip PWD
		if info.IsDir() && info.Name() == dev.Label {
			return nil
		}

		files = append(files, info)

		return nil
	})
	if err != nil {
		t.Error("Failed to walk dir", err)
	}

	expected := map[string]interface{}{"talos": nil, "talosdir": nil, "talosfile": nil}

	if len(files) != len(expected) {
		t.Errorf("Did not get back expected number of files - expected %d got %d", len(expected), len(files))
	}

	for _, file := range files {
		if _, ok := expected[file.Name()]; !ok {
			t.Errorf("Unexpected file %s", file.Name())
		}
	}

}

func newdev(t *testing.T, label string) (*Device, *httptest.Server) {
	// Set up a simple http server to serve a simple asset
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("testdata")))
	ts := httptest.NewServer(mux)

	// Set up a test for dir creation and file download
	dev := newDevice("testdev", label, 1024, []string{"lala", ts.URL + "/talos_test.tar.gz"})

	out, err := ioutil.TempDir("", "testbaremetal")
	if err != nil {
		t.Error("Failed to open file", err)
	}

	// nolint: errcheck
	dev.MountBase = out

	if err := os.MkdirAll(filepath.Join(out, dev.Label), 0755); err != nil {
		t.Fatalf("Failed to set up 'mountpoint' for %s", filepath.Join(out, dev.Label))
	}

	return dev, ts
}
