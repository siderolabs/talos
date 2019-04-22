/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package conditions_test

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/app/init/pkg/system/conditions"
)

type ConditionsSuite struct {
	suite.Suite

	tempDir string
}

func (suite *ConditionsSuite) SetupSuite() {
	var err error
	suite.tempDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

}

func (suite *ConditionsSuite) TearDownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tempDir))
}

func (suite *ConditionsSuite) createFile(name string) (path string) {
	path = filepath.Join(suite.tempDir, name)
	f, err := os.Create(path)
	suite.Require().NoError(err)

	suite.Require().NoError(f.Close())

	return
}

func (suite *ConditionsSuite) TestFileExists() {
	exists, err := conditions.FileExists("no-such-file")(context.Background())
	suite.Require().NoError(err)
	suite.Require().False(exists)

	exists, err = conditions.FileExists(suite.createFile("a.txt"))(context.Background())
	suite.Require().NoError(err)
	suite.Require().True(exists)
}

func (suite *ConditionsSuite) TestWaitForFileToExist() {
	path := suite.createFile("w.txt")

	exists, err := conditions.WaitForFileToExist(path)(context.Background())
	suite.Require().NoError(err)
	suite.Require().True(exists)

	suite.Require().NoError(os.Remove(path))

	errCh := make(chan error)

	go func() {
		exists, err = conditions.WaitForFileToExist(path)(context.Background())
		suite.Require().True(exists)

		errCh <- err
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
		_, err = conditions.WaitForFileToExist(path)(ctx)
		errCh <- err
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

func (suite *ConditionsSuite) TestWaitForFilesToExist() {
	pathA := suite.createFile("wA.txt")
	pathB := suite.createFile("wB.txt")

	exists, err := conditions.WaitForFilesToExist(pathA, pathB)(context.Background())
	suite.Require().NoError(err)
	suite.Require().True(exists)

	suite.Require().NoError(os.Remove(pathB))

	errCh := make(chan error)

	go func() {
		exists, err = conditions.WaitForFilesToExist(pathA, pathB)(context.Background())
		suite.Require().True(exists)

		errCh <- err
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
		_, err = conditions.WaitForFilesToExist(pathA, pathB)(ctx)
		errCh <- err
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

func TestConditionsSuite(t *testing.T) {
	suite.Run(t, new(ConditionsSuite))
}
