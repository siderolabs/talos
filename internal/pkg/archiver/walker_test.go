/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package archiver provides a service to archive part of the filesystem into tar archive
package archiver_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/internal/pkg/archiver"
)

type WalkerSuite struct {
	CommonSuite
}

func (suite *WalkerSuite) TestIterationDir() {
	ch, errCh, err := archiver.Walker(context.Background(), suite.tmpDir)
	suite.Require().NoError(err)

	relPaths := []string(nil)

	for fi := range ch {
		relPaths = append(relPaths, fi.RelPath)

		if fi.RelPath == "usr/bin/mv" {
			suite.Assert().Equal("/usr/bin/cp", fi.Link)
		}
	}

	suite.Require().NoError(<-errCh)

	suite.Assert().Equal([]string{
		"dev", "dev/random",
		"etc", "etc/certs", "etc/certs/ca.crt", "etc/hostname",
		"lib", "lib/dynalib.so",
		"usr", "usr/bin", "usr/bin/cp", "usr/bin/mv"},
		relPaths)
}

func (suite *WalkerSuite) TestIterationFile() {
	ch, errCh, err := archiver.Walker(context.Background(), filepath.Join(suite.tmpDir, "usr/bin/cp"))
	suite.Require().NoError(err)

	relPaths := []string(nil)

	for fi := range ch {
		relPaths = append(relPaths, fi.RelPath)
	}

	suite.Require().NoError(<-errCh)

	suite.Assert().Equal([]string{"cp"},
		relPaths)
}

func (suite *WalkerSuite) TestIterationNotFound() {
	_, _, err := archiver.Walker(context.Background(), filepath.Join(suite.tmpDir, "doesntlivehere"))
	suite.Require().Error(err)
}

func TestWalkerSuite(t *testing.T) {
	suite.Run(t, new(WalkerSuite))
}
