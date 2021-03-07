// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/conditions"
)

type FilesSuite struct {
	suite.Suite

	tempDir string
}

func (suite *FilesSuite) SetupSuite() {
	var err error
	suite.tempDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)
}

func (suite *FilesSuite) TearDownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tempDir))
}

func (suite *FilesSuite) createFile(name string) (path string) {
	path = filepath.Join(suite.tempDir, name)
	f, err := os.Create(path)
	suite.Require().NoError(err)

	suite.Require().NoError(f.Close())

	return
}

func (suite *FilesSuite) TestString() {
	suite.Require().Equal("file \"abc.txt\" to exist", conditions.WaitForFileToExist("abc.txt").String())
}

func (suite *FilesSuite) TestWaitForFileToExist() {
	path := suite.createFile("w.txt")

	err := conditions.WaitForFileToExist(path).Wait(context.Background())
	suite.Require().NoError(err)

	suite.Require().NoError(os.Remove(path))

	errCh := make(chan error)

	go func() {
		errCh <- conditions.WaitForFileToExist(path).Wait(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case <-errCh:
		suite.Fail("unexpected return")
	default:
	}

	suite.createFile("w.txt")

	suite.Require().NoError(<-errCh)

	suite.Require().NoError(os.Remove(path))

	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- conditions.WaitForFileToExist(path).Wait(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case <-errCh:
		suite.Fail("unexpected return")
	default:
	}

	ctxCancel()

	suite.Require().EqualError(<-errCh, context.Canceled.Error())
}

func (suite *FilesSuite) TestWaitForAllFilesToExist() {
	pathA := suite.createFile("wA.txt")
	pathB := suite.createFile("wB.txt")

	err := conditions.WaitForFilesToExist(pathA, pathB).Wait(context.Background())
	suite.Require().NoError(err)

	suite.Require().NoError(os.Remove(pathB))

	errCh := make(chan error)

	go func() {
		errCh <- conditions.WaitForFilesToExist(pathA, pathB).Wait(context.Background())
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case <-errCh:
		suite.Fail("unexpected return")
	default:
	}

	suite.createFile("wB.txt")

	suite.Require().NoError(<-errCh)

	suite.Require().NoError(os.Remove(pathA))
	suite.Require().NoError(os.Remove(pathB))

	ctx, ctxCancel := context.WithCancel(context.Background())

	go func() {
		errCh <- conditions.WaitForFilesToExist(pathA, pathB).Wait(ctx)
	}()

	time.Sleep(50 * time.Millisecond)

	select {
	case <-errCh:
		suite.Fail("unexpected return")
	default:
	}

	ctxCancel()

	suite.Require().EqualError(<-errCh, context.Canceled.Error())
}

func TestFilesSuite(t *testing.T) {
	suite.Run(t, new(FilesSuite))
}
