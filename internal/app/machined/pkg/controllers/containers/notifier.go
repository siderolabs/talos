// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

// notifier wakes a controller's main loop from the goroutines it runs work on.
//
// It is a coalescing "something changed, look again" signal: a single slot, so a pending wake-up
// already covers every change made before the loop gets to it, and a second one is redundant.
//
// Receive from it in the main loop like from any channel. How long it lives is up to the controller:
// one whose goroutines outlive a Run has to keep its notifier across Runs too, or a change made just
// after a restart never reaches the next loop.
type notifier chan struct{}

func newNotifier() notifier {
	return make(notifier, 1)
}

// notify wakes the main loop, never blocking.
//
// Blocking here would deadlock the controller's shutdown: once the main loop has returned, nothing
// drains the notifier again.
func (n notifier) notify() {
	select {
	case n <- struct{}{}:
	default:
	}
}
