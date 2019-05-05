/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package bully

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/stretchr/testify/suite"
)

type BullySuite struct {
	suite.Suite

	tmpDir string
}

func TestBullySuite(t *testing.T) {
	suite.Run(t, new(BullySuite))
}

func (suite *BullySuite) SetupSuite() {
	var err error
	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)
}

func (suite *BullySuite) TeardownSuite() {
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

func (suite *BullySuite) TestElect() {
	// Start a 3 node cluster.
	size := 3

	sockets := make([]string, 0, size)
	for i := 0; i < cap(sockets); i++ {
		sock := filepath.Join(suite.tmpDir, strconv.Itoa(i), "bully.sock")
		err := os.MkdirAll(filepath.Dir(sock), os.ModeDir)
		suite.Require().NoError(err)
		sockets = append(sockets, sock)
	}

	bullies := make([]*Bully, 0, size)
	for i := 0; i < cap(bullies); i++ {
		addrs := make([]string, len(sockets))
		copy(addrs, sockets)
		addrs = append(addrs[:i], addrs[i+1:]...)
		bully := NewBullyServer(uint32(i), sockets[i], addrs...)
		bullies = append(bullies, bully)
	}

	var wg sync.WaitGroup
	wg.Add(len(bullies))

	for _, bully := range bullies {
		go func(b *Bully) {
			wg.Done()
			err := b.Start()
			suite.Require().NoError(err)
		}(bully)
	}

	// Wait here so that all that servers have started and the sockets exist.
	wg.Wait()

	wg.Add(len(bullies))

	for _, bully := range bullies {
		go func(b *Bully) {
			defer wg.Done()
			err := b.Join()
			suite.Require().NoError(err)
		}(bully)
	}

	wg.Wait()

	wg.Add(len(bullies))

	for _, bully := range bullies {
		go func(b *Bully) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()
			_, err := b.Elect(ctx, &empty.Empty{})
			suite.Require().NoError(err)
		}(bully)
	}

	wg.Wait()

	for _, bully := range bullies {
		suite.Assert().Equal(bully.coordinator.Pid.Value, bullies[size-1].coordinator.Pid.Value)
	}
}
