// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package loadbalancer_test

import (
	"io/ioutil"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/pkg/loadbalancer"
	"github.com/talos-systems/talos/internal/pkg/loadbalancer/upstream"
)

type mockUpstream struct {
	identity string

	addr string
	l    net.Listener
}

func (u *mockUpstream) Start() error {
	var err error

	u.l, err = net.Listen("tcp", "localhost:0")
	if err != nil {
		return err
	}

	u.addr = u.l.Addr().String()

	go u.serve()

	return nil
}

func (u *mockUpstream) serve() {
	for {
		c, err := u.l.Accept()
		if err != nil {
			return
		}

		c.Write([]byte(u.identity)) //nolint: errcheck
		c.Close()                   //nolint: errcheck
	}
}

func (u *mockUpstream) Close() {
	u.l.Close() //nolint: errcheck
}

func findListenAddress() (string, error) {
	u := mockUpstream{}

	if err := u.Start(); err != nil {
		return "", err
	}

	u.Close()

	return u.addr, nil
}

type TCPSuite struct {
	suite.Suite
}

func (suite *TCPSuite) TestBalancer() {
	const (
		upstreamCount   = 5
		failingUpstream = 1
	)

	upstreams := make([]mockUpstream, upstreamCount)
	for i := range upstreams {
		upstreams[i].identity = strconv.Itoa(i)
		suite.Require().NoError(upstreams[i].Start())
	}

	upstreamAddrs := make([]string, len(upstreams))
	for i := range upstreamAddrs {
		upstreamAddrs[i] = upstreams[i].addr
	}

	listenAddr, err := findListenAddress()
	suite.Require().NoError(err)

	lb := &loadbalancer.TCP{}
	suite.Require().NoError(lb.AddRoute(
		listenAddr,
		upstreamAddrs,
		upstream.WithLowHighScores(-3, 3),
		upstream.WithInitialScore(1),
		upstream.WithScoreDeltas(-1, 1),
		upstream.WithHealthcheckInterval(time.Second),
		upstream.WithHealthcheckTimeout(100*time.Millisecond),
	))

	suite.Require().NoError(lb.Start())

	var wg sync.WaitGroup

	wg.Add(1)

	go func() {
		defer wg.Done()

		lb.Wait() //nolint: errcheck
	}()

	for i := 0; i < 2*upstreamCount; i++ {
		c, err := net.Dial("tcp", listenAddr)
		suite.Require().NoError(err)

		id, err := ioutil.ReadAll(c)
		suite.Require().NoError(err)

		// load balancer should go round-robin across all the upstreams
		suite.Assert().Equal([]byte(strconv.Itoa(i%upstreamCount)), id)

		suite.Require().NoError(c.Close())
	}

	// bring down one upstream
	upstreams[failingUpstream].Close()

	j := 0
	failedRequests := 0

	for i := 0; i < 10*upstreamCount; i++ {
		c, err := net.Dial("tcp", listenAddr)
		suite.Require().NoError(err)

		id, err := ioutil.ReadAll(c)
		suite.Require().NoError(err)

		if len(id) == 0 {
			// hit failing upstream
			suite.Assert().Equal(failingUpstream, j%upstreamCount)

			failedRequests++

			continue
		}

		if j%upstreamCount == failingUpstream {
			j++
		}

		// load balancer should go round-robin across all the upstreams
		suite.Assert().Equal([]byte(strconv.Itoa(j%upstreamCount)), id)
		j++

		suite.Require().NoError(c.Close())
	}

	// worst case: score = 3 (highScore) to go to -1 requires 5 requests
	suite.Assert().Less(failedRequests, 5) // no more than 5 requests should fail

	suite.Require().NoError(lb.Close())
	wg.Wait()

	for i := range upstreams {
		upstreams[i].Close()
	}
}

func TestTCPSuite(t *testing.T) {
	suite.Run(t, new(TCPSuite))
}
