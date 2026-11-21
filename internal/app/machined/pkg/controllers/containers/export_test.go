// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers

import "time"

// SetPullRetryPolicy overrides the pull timeout and the first retry backoff.
func (ctrl *ImageController) SetPullRetryPolicy(timeout, interval time.Duration) {
	ctrl.pullTimeout, ctrl.retryInterval = timeout, interval
}

// Exported for tests.
var (
	AllocateSELinuxLevels = allocateSELinuxLevels
	SELinuxPair           = selinuxPair
	SELinuxPairCount      = selinuxPairCount
	DecodeSeccompProfile  = decodeSeccompProfile
	RunProbeLoop          = runProbeLoop
	NetNSPath             = netNSPath
	NetNSDialer           = netNSDialer
	OpenTaskNetNS         = openTaskNetNS
	TaskNetNSDialer       = taskNetNSDialer
	TaskCgroupPath        = taskCgroupPath

	LogContainerStatusTransition = logContainerStatusTransition
)
