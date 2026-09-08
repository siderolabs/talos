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
	"go.uber.org/zap"

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

	mu       sync.Mutex
	deadline time.Time
}

func newTestListener() *testListener {
	return &testListener{reads: make(chan readResult), closed: make(chan struct{})}
}

func (listener *testListener) SetReadDeadline(deadline time.Time) error {
	listener.mu.Lock()
	defer listener.mu.Unlock()

	listener.deadline = deadline

	return nil
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

	ctx, cancel := context.WithCancel(t.Context())
	op := operator.NewLLDP(zap.NewNop(), "eth0", 7, factory)
	notify := make(chan struct{})

	var notifications atomic.Int32

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-notify:
				notifications.Add(1)
			}
		}
	}()

	go op.Run(ctx, notify)

	return op, &notifications, cancel
}

func TestLLDPOperatorRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32

		opened := make(chan *testListener, 2)

		op, _, cancel := runLLDP(t, func(linkIndex uint32) (lldp.Listener, error) {
			require.EqualValues(t, 7, linkIndex)

			if attempts.Add(1) == 1 {
				return nil, io.ErrUnexpectedEOF
			}

			listener := newTestListener()
			opened <- listener

			return listener, nil
		})

		synctest.Wait()
		require.EqualValues(t, 1, attempts.Load())

		<-time.NewTimer(5 * time.Second).C
		synctest.Wait()
		require.EqualValues(t, 2, attempts.Load(), "a failed listener is retried")

		first := <-opened
		first.reads <- advertisementResult(2)

		synctest.Wait()
		require.Len(t, op.LLDPNeighborSpecs(), 1)

		first.reads <- readResult{err: io.EOF}

		synctest.Wait()
		<-first.closed
		<-time.NewTimer(2 * time.Second).C
		synctest.Wait()
		require.Empty(t, op.LLDPNeighborSpecs(), "expiry must continue during socket retry")
		require.EqualValues(t, 2, attempts.Load())

		<-time.NewTimer(3 * time.Second).C
		synctest.Wait()
		require.EqualValues(t, 3, attempts.Load())

		second := <-opened
		second.reads <- advertisementResult(30)

		synctest.Wait()
		require.Len(t, op.LLDPNeighborSpecs(), 1)

		cancel()
		synctest.Wait()
		<-second.closed
		require.Empty(t, op.LLDPNeighborSpecs(), "a stopped operator cannot expose stale neighbors")

		<-time.NewTimer(10 * time.Second).C
		synctest.Wait()
		require.EqualValues(t, 3, attempts.Load(), "shutdown cancels retries")
	})
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

func TestLLDPOperatorCancelDuringRetry(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var attempts atomic.Int32

		op, _, cancel := runLLDP(t, func(uint32) (lldp.Listener, error) {
			attempts.Add(1)

			return nil, io.ErrUnexpectedEOF
		})

		synctest.Wait()
		cancel()

		<-time.NewTimer(10 * time.Second).C
		synctest.Wait()
		require.EqualValues(t, 1, attempts.Load())
		require.Empty(t, op.LLDPNeighborSpecs())
	})
}

func advertisementResult(ttl uint16) readResult {
	return readResult{frame: advertisement(ttl)}
}
