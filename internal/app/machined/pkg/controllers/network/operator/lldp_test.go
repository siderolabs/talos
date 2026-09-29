// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package operator_test

import (
	"context"
	"encoding/binary"
	"io"
	"net/netip"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/internal/lldp"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/network/operator"
)

func tlv(kind uint16, value ...byte) []byte {
	return append(binary.BigEndian.AppendUint16(nil, kind<<9|uint16(len(value))), value...)
}

// advertisement builds an Ethernet LLDP frame carrying the mandatory TLVs.
func advertisement(ttl uint16, optional ...[]byte) []byte {
	result := []byte{1, 0x80, 0xc2, 0, 0, 0x0e, 2, 0, 0, 0, 0, 1, 0x88, 0xcc}

	values := make([][]byte, 0, 4+len(optional))
	values = append(values, tlv(1, 4, 2, 0, 0, 0, 0, 1), tlv(2, 5, 'e', 't', 'h', '0'), tlv(3, byte(ttl>>8), byte(ttl)))
	values = append(values, optional...)
	values = append(values, tlv(0))

	for _, value := range values {
		result = append(result, value...)
	}

	return result
}

func management(ip string) []byte {
	address := netip.MustParseAddr(ip)
	family := byte(2)

	if address.Is4() {
		family = 1
	}

	value := append([]byte{byte(address.BitLen()/8 + 1), family}, address.AsSlice()...)
	value = append(value, 2, 0, 0, 0, 1, 0) // interface subtype, number and empty OID

	return tlv(8, value...)
}

func vlanName(id uint16, name string) []byte {
	value := make([]byte, 0, 7+len(name))
	value = append(value, 0, 0x80, 0xc2, 3, byte(id>>8), byte(id), byte(len(name)))

	return tlv(127, append(value, name...)...)
}

type readResult struct {
	frame []byte
	err   error
}

// testListener honors read deadlines, as the operator relies on them to age out neighbors.
type testListener struct {
	reads  chan readResult
	closed chan struct{}
	once   sync.Once

	mu          sync.Mutex
	deadline    time.Time
	deadlineErr error
}

func newTestListener() *testListener {
	return &testListener{reads: make(chan readResult), closed: make(chan struct{})}
}

func (listener *testListener) SetReadDeadline(deadline time.Time) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	listener.deadline = deadline

	return listener.deadlineErr
}

func (listener *testListener) ReadFrame() ([]byte, error) {
	listener.mu.Lock()
	deadline := listener.deadline
	listener.mu.Unlock()

	var expired <-chan time.Time

	if !deadline.IsZero() {
		timer := time.NewTimer(time.Until(deadline))
		defer timer.Stop()

		expired = timer.C
	}

	select {
	case result := <-listener.reads:
		return result.frame, result.err
	case <-expired:
		return nil, os.ErrDeadlineExceeded
	case <-listener.closed:
		return nil, io.EOF
	}
}

func (listener *testListener) Close() error {
	listener.once.Do(func() { close(listener.closed) })

	return nil
}

// runLLDP starts the operator, draining notifications the way the controller does.
func runLLDP(t *testing.T, factory lldp.ListenerFactory) (*operator.LLDP, *atomic.Int32, context.CancelFunc) {
	t.Helper()

	fixture := newLLDPTest(t, factory)

	return fixture.op, &fixture.notifications, fixture.cancel
}

type lldpTest struct {
	t             *testing.T
	op            *operator.LLDP
	listener      *testListener
	cancel        context.CancelFunc
	done          chan struct{}
	logs          *observer.ObservedLogs
	notifications atomic.Int32
	attempts      atomic.Int32
	index         atomic.Uint32
}

func newLLDPTest(t *testing.T, factory lldp.ListenerFactory) *lldpTest {
	t.Helper()

	ctx, cancel := context.WithCancel(t.Context())
	core, logs := observer.New(zap.WarnLevel)
	fixture := &lldpTest{
		t: t, listener: newTestListener(), cancel: cancel,
		done: make(chan struct{}), logs: logs,
	}
	fixture.op = operator.NewLLDP(zap.New(core), "eth0", 7, func(index uint32) (lldp.Listener, error) {
		fixture.attempts.Add(1)
		fixture.index.Store(index)

		if factory != nil {
			return factory(index)
		}

		return fixture.listener, nil
	})
	notify := make(chan struct{})

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-notify:
				fixture.notifications.Add(1)
			}
		}
	}()

	go func() {
		defer close(fixture.done)

		fixture.op.Run(ctx, notify)
	}()

	return fixture
}

type LLDPOperatorSuite struct {
	suite.Suite
}

func (suite *LLDPOperatorSuite) TestOpenFailure() {
	synctest.Test(suite.T(), func(t *testing.T) {
		fixture := newLLDPTest(t, func(uint32) (lldp.Listener, error) {
			return nil, io.ErrUnexpectedEOF
		})
		defer fixture.cancel()

		fixture.assertFailure(0)
	})
}

func (suite *LLDPOperatorSuite) TestReadFailure() {
	suite.testSocketFailure(false, false)
}

func (suite *LLDPOperatorSuite) TestReadFailureWithNeighbors() {
	suite.testSocketFailure(true, false)
}

func (suite *LLDPOperatorSuite) TestDeadlineFailure() {
	suite.testSocketFailure(false, true)
}

func (suite *LLDPOperatorSuite) TestDeadlineFailureWithNeighbors() {
	suite.testSocketFailure(true, true)
}

func (suite *LLDPOperatorSuite) testSocketFailure(populated, deadline bool) {
	synctest.Test(suite.T(), func(t *testing.T) {
		fixture := newLLDPTest(t, nil)
		defer fixture.cancel()

		synctest.Wait() // block the initial read before injecting a failure

		var notifications int32

		if populated {
			fixture.listener.reads <- advertisementResult(120)

			synctest.Wait()
			require.Len(t, fixture.op.LLDPNeighborSpecs(), 1)

			notifications = 2 // publication and cleanup must each notify
		}

		if deadline {
			fixture.listener.mu.Lock()
			fixture.listener.deadlineErr = io.ErrUnexpectedEOF
			fixture.listener.mu.Unlock()

			// Finish the current read so the next deadline call fails.
			fixture.listener.reads <- readResult{frame: []byte{0}}
		} else {
			fixture.listener.reads <- readResult{err: io.ErrUnexpectedEOF}
		}

		synctest.Wait()
		assertClosed(t, fixture.listener.closed, "failed listener must close")
		fixture.assertFailure(notifications)
	})
}

func (fixture *lldpTest) assertFailure(notifications int32) {
	fixture.t.Helper()
	synctest.Wait()
	require.EqualValues(fixture.t, 1, fixture.attempts.Load())
	require.EqualValues(fixture.t, 7, fixture.index.Load())
	assertClosed(fixture.t, fixture.done, "Run must terminate after one listener failure")
	require.Empty(fixture.t, fixture.op.LLDPNeighborSpecs(), "failure must immediately discard neighbors")
	require.Equal(fixture.t, notifications, fixture.notifications.Load(), "cleanup must notify exactly when neighbors existed")
	require.Equal(fixture.t, 1, fixture.logs.Len(), "socket failure must be logged once")

	<-time.NewTimer(10 * time.Second).C
	synctest.Wait()
	require.EqualValues(fixture.t, 1, fixture.attempts.Load(), "failure must not schedule a retry")
}

func assertClosed(t *testing.T, ch <-chan struct{}, message string) {
	t.Helper()

	select {
	case <-ch:
	default:
		t.Error(message)
	}
}

func TestLLDPOperatorNeighborLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := newTestListener()

		op, notifications, cancel := runLLDP(t, func(uint32) (lldp.Listener, error) {
			return listener, nil
		})
		defer cancel()

		send := func(ttl uint16) {
			listener.reads <- readResult{frame: advertisement(ttl, management("192.0.2.1"), vlanName(10, "clients"))}

			synctest.Wait()
		}

		advance := func(duration time.Duration) {
			<-time.NewTimer(duration).C
			synctest.Wait()
		}

		send(10)

		neighbors := op.LLDPNeighborSpecs()
		require.Len(t, neighbors, 1)

		neighbors[0].ManagementAddresses[0] = "mutated"
		neighbors[0].VLANs[0].Name = "mutated"
		neighbors = op.LLDPNeighborSpecs()
		require.Equal(t, "192.0.2.1", neighbors[0].ManagementAddresses[0], "results are independently owned")
		require.Equal(t, "clients", neighbors[0].VLANs[0].Name)

		advance(6 * time.Second)
		send(20)
		advance(4 * time.Second)
		require.Equal(t, neighbors, op.LLDPNeighborSpecs(), "TTL-only refresh does not change public data")

		expiryNotifications := notifications.Load()

		advance(16 * time.Second)
		require.Empty(t, op.LLDPNeighborSpecs())
		require.Greater(t, notifications.Load(), expiryNotifications, "expiry wakes the controller without link events")

		send(30)

		listener.reads <- readResult{frame: []byte{0}}

		synctest.Wait()
		require.Len(t, op.LLDPNeighborSpecs(), 1, "invalid frames must not clear neighbors")

		send(0)
		require.Empty(t, op.LLDPNeighborSpecs(), "zero TTL withdraws the neighbor")
	})
}

func TestLLDPOperatorRepeatedAdvertisement(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		listener := newTestListener()

		op, notifications, cancel := runLLDP(t, func(uint32) (lldp.Listener, error) {
			return listener, nil
		})
		defer cancel()

		frame := advertisement(30, management("192.0.2.1"))

		listener.reads <- readResult{frame: frame}

		synctest.Wait()
		require.Len(t, op.LLDPNeighborSpecs(), 1)

		first := notifications.Load()

		// The same neighbor re-advertises unchanged information every tx interval.
		for range 5 {
			<-time.NewTimer(10 * time.Second).C

			listener.reads <- readResult{frame: frame}

			synctest.Wait()
		}

		require.Equal(t, first, notifications.Load(), "an unchanged advertisement does not wake the controller")

		// Lifetime is still extended, so the neighbor outlives its original TTL.
		<-time.NewTimer(20 * time.Second).C
		synctest.Wait()
		require.Len(t, op.LLDPNeighborSpecs(), 1, "a repeated advertisement refreshes the TTL")

		<-time.NewTimer(31 * time.Second).C
		synctest.Wait()
		require.Empty(t, op.LLDPNeighborSpecs(), "refreshing stops when advertisements stop")
	})
}

func (suite *LLDPOperatorSuite) TestCancellation() {
	suite.testCancellation(false)
}

func (suite *LLDPOperatorSuite) TestCancellationWithNeighbors() {
	suite.testCancellation(true)
}

func (suite *LLDPOperatorSuite) testCancellation(populated bool) {
	synctest.Test(suite.T(), func(t *testing.T) {
		fixture := newLLDPTest(t, nil)
		defer fixture.cancel()

		if populated {
			fixture.listener.reads <- advertisementResult(120)
		}

		synctest.Wait()
		fixture.cancel()
		synctest.Wait()

		assertClosed(t, fixture.done, "cancellation must unblock Run even when notifications stop draining")
		assertClosed(t, fixture.listener.closed, "cancellation must close the listener")
		require.Empty(t, fixture.op.LLDPNeighborSpecs())
		require.Zero(t, fixture.logs.Len(), "normal cancellation must not log a listener failure")

		<-time.NewTimer(10 * time.Second).C
		synctest.Wait()
		require.EqualValues(t, 1, fixture.attempts.Load(), "cancellation must not reopen the listener")
	})
}

func TestLLDPOperatorSuite(t *testing.T) {
	suite.Run(t, &LLDPOperatorSuite{})
}

func advertisementResult(ttl uint16) readResult {
	return readResult{frame: advertisement(ttl)}
}
