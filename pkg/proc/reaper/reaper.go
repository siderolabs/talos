// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package reaper implements zombie process reaper with notifications.
package reaper

import "syscall"

// Run launches loop for the zombie process reaper.
func Run() {
	hunter.Run()
}

// Shutdown stops the process reaper.
func Shutdown() {
	hunter.Shutdown()
}

// ProcessInfo describes reaped zombie process.
type ProcessInfo struct {
	Pid    int
	Status syscall.WaitStatus
}

// Notify causes reaper to deliver notifications about reaped zombies.
//
// If Notify returns false, reaper is not running, and Notify does nothing.
func Notify(ch chan<- ProcessInfo) bool {
	return hunter.Notify(ch)
}

// Stop sending notifications to the channel.
func Stop(ch chan<- ProcessInfo) {
	hunter.Stop(ch)
}

// Singleton instance of zombieHunter.
var hunter = &zombieHunter{}
