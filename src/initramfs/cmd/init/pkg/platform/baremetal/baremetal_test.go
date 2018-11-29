package baremetal

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"unsafe"

	"github.com/autonomy/talos/src/initramfs/cmd/init/pkg/constants"
	"github.com/autonomy/talos/src/initramfs/pkg/blockdevice"
	"golang.org/x/sys/unix"
)

// nolint: gocyclo
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
	// nolint: errcheck
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

func TestPartition(t *testing.T) {
	f := newbd(t)

	// nolint: errcheck
	defer f.Close()
	// nolint: errcheck
	defer os.RemoveAll(f.Name())

	lo := newloop(t, f)

	// nolint: errcheck
	defer lo.Close()
	// nolint: errcheck
	defer os.RemoveAll(lo.Name())

	dev := NewDevice(lo.Name(), constants.RootPartitionLabel, 512*1000*1000, []string{})
	bd, err := blockdevice.Open(dev.Name, blockdevice.WithNewGPT(true))
	if err != nil {
		t.Error("Failed to create block device", err)
	}
	pt, err := bd.PartitionTable(false)
	if err != nil {
		t.Error("Failed to get partition table", err)
	}
	dev.PartitionTable = pt

	err = dev.Partition()
	if err != nil {
		t.Error("Failed to create new partition", err)
	}

	err = dev.PartitionTable.Write()
	if err != nil {
		t.Error("Failed to write partition to disk", err)
	}

	/*
		ud := userdata.UserData{
			Install: &userdata.Install{
				Wipe: true,
				Boot: &userdata.InstallDevice{
					Device: f.Name(),
					Size:   512 * 1000 * 1000,
				},
				Root: &userdata.InstallDevice{
					Device: f.Name(),
					Size:   512 * 1000 * 1000,
					Data:   []string{"https://storage.googleapis.com/kubernetes-helm/helm-v2.11.0-linux-amd64.tar.gz"},
				},
				Data: &userdata.InstallDevice{
					Device: f.Name(),
					Size:   512 * 1000 * 1000,
				},
			},
		}

		bm := BareMetal{}

		err := bm.Install(ud)
		if err != nil {
			t.Fatal("Installation failed: ", err)
		}
	*/
}

func newdev(t *testing.T, label string) (*Device, *httptest.Server) {
	// Set up a simple http server to serve a simple asset
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("testdata")))
	ts := httptest.NewServer(mux)

	// Set up a test for dir creation and file download
	dev := NewDevice("testdev", label, 1024, []string{"lala", ts.URL + "/talos_test.tar.gz"})

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

func newbd(t *testing.T) *os.File {

	tmpfile, err := ioutil.TempFile("", "testbaremetal")
	if err != nil {
		t.Fatal("Failed to create tempfile", err)
	}

	// Create a 3G sparse file so we can partition it
	if err := tmpfile.Truncate(3e9); err != nil {
		t.Fatal("Failed to truncate tempfile", err)
	}

	_, err = tmpfile.Seek(0, 0)
	if err != nil {
		t.Fatal("Failed to reset tmpfile read position", err)
	}

	return tmpfile
}

func newloop(t *testing.T, backer *os.File) *os.File {
	err := unix.Mknod("/dev/loop", 0660, 7)
	if err != nil {
		t.Fatal("Failed to create loopback device", err)
	}

	loopFile, err := os.Open("/dev/loop")
	if err != nil {
		t.Fatal("Failed to open loopback device", err)
	}

	_, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), 0x4C00, backer.Fd())
	if errno == 0 {
		type Info struct {
			Device         uint64
			INode          uint64
			RDevice        uint64
			Offset         uint64
			SizeLimit      uint64
			Number         uint32
			EncryptType    uint32
			EncryptKeySize uint32
			Flags          uint32
			FileName       [64]byte
			CryptName      [64]byte
			EncryptKey     [32]byte
			Init           [2]uint64
		}
		info := Info{}
		copy(info.FileName[:], []byte(backer.Name()))
		info.Offset = 0

		_, _, errno := unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), 0x4C04, uintptr(unsafe.Pointer(&info)))
		if errno == unix.ENXIO {
			// nolint: errcheck
			unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), 0x4C01, 0)
			t.Error("device not backed by a file")
		} else if errno != 0 {
			// nolint: errcheck
			unix.Syscall(unix.SYS_IOCTL, loopFile.Fd(), 0x4C01, 0)
			t.Errorf("could not get info about (err: %d): %v", errno, errno)
		}
	}

	_, err = loopFile.Seek(0, 0)
	if err != nil {
		t.Fatal("Failed to reset tmpfile read position", err)
	}

	return loopFile
}
