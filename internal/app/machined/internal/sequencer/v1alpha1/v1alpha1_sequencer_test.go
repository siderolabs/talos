// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint: scopelint
package v1alpha1

import (
	"errors"
	"sync"
	"testing"
	"time"

	machineapi "github.com/talos-systems/talos/api/machine"
)

type MockSequencer struct {
	Sequencer
}

// Boot is a mock method that overrides the embedded sequencer's Boot method.
func (s *MockSequencer) Boot() error {
	f := func() error {
		time.Sleep(time.Second)
		return nil
	}

	return s.run(f)
}

// Shutdown is a mock method that overrides the embedded sequencer's Shutdown method.
func (s *MockSequencer) Shutdown() error {
	f := func() error {
		time.Sleep(time.Second)
		return nil
	}

	return s.run(f)
}

// Upgrade is a mock method that overrides the embedded sequencer's Upgrade method.
func (s *MockSequencer) Upgrade(req *machineapi.UpgradeRequest) error {
	f := func() error {
		time.Sleep(time.Second)
		return nil
	}

	return s.run(f)
}

func TestSequencer_run(t *testing.T) {
	s := &MockSequencer{}

	type args struct {
		f func() error
	}

	tests := []struct {
		name  string
		args  args
		count int
	}{
		{
			name: "1 boot request",
			args: args{
				f: s.Boot,
			},
			count: 1,
		},
		{
			name: "5 boot requests",
			args: args{
				f: s.Boot,
			},
			count: 5,
		},
		{
			name: "5 shutdown requests",
			args: args{
				f: s.Shutdown,
			},
			count: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var (
				wg sync.WaitGroup
				mu sync.Mutex
			)

			wg.Add(tt.count)

			var count int

			for i := 0; i < tt.count; i++ {
				go func() {
					defer wg.Done()

					if err := tt.args.f(); err != nil && errors.Is(err, ErrLocked{}) {
						mu.Lock()
						count++
						mu.Unlock()
					}
				}()
			}

			wg.Wait()

			if count != tt.count-1 {
				t.Errorf("Sequencer.run() expected %d errors, got %d", tt.count-1, count)
			}
		})
	}
}
