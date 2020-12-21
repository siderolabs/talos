// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package archiver_test

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/stretchr/testify/suite"
)

type CommonSuite struct {
	suite.Suite

	tmpDir string
}

var filesFixture = []struct {
	Path     string
	Mode     os.FileMode
	Contents []byte
	Size     int
}{
	{
		Path:     "/etc/hostname",
		Mode:     0o644,
		Contents: []byte("localhost"),
	},
	{
		Path:     "/etc/certs/ca.crt",
		Mode:     0o600,
		Contents: []byte("-- CA PEM CERT -- VERY SECRET"),
	},
	{
		Path: "/dev/random",
		Mode: 0o600 | os.ModeDevice | os.ModeCharDevice,
	},
	{
		Path:     "/usr/bin/cp",
		Mode:     0o755,
		Contents: []byte("ELF EXECUTABLE IIRC"),
	},
	{
		Path:     "/usr/bin/mv",
		Mode:     0o644 | os.ModeSymlink,
		Contents: []byte("/usr/bin/cp"),
	},
	{
		Path:     "/lib/dynalib.so",
		Mode:     0o644,
		Contents: []byte("SOME LIBRARY OUT THERE"),
		Size:     20 * 1024,
	},
}

func (suite *CommonSuite) SetupSuite() {
	var err error

	suite.tmpDir, err = ioutil.TempDir("", "archiver")
	suite.Require().NoError(err)

	for _, file := range filesFixture {
		suite.Require().NoError(os.MkdirAll(filepath.Join(suite.tmpDir, filepath.Dir(file.Path)), 0o777))

		if file.Mode&os.ModeSymlink != 0 {
			suite.Require().NoError(os.Symlink(string(file.Contents), filepath.Join(suite.tmpDir, file.Path)))

			continue
		}

		f, err := os.OpenFile(filepath.Join(suite.tmpDir, file.Path), os.O_CREATE|os.O_WRONLY, file.Mode)
		suite.Require().NoError(err)

		var contents []byte

		if file.Size > 0 {
			contents = bytes.Repeat(file.Contents, file.Size/len(file.Contents))
			contents = append(contents, file.Contents[:file.Size-file.Size/len(file.Contents)*len(file.Contents)]...)
		} else {
			contents = file.Contents
		}

		_, err = f.Write(contents)
		suite.Require().NoError(err)

		suite.Require().NoError(f.Close())
	}
}

func (suite *CommonSuite) TearDownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}
