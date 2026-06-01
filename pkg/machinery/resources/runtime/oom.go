// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

// QoSCgroupClass is a quality of service class of cgroup.
type QoSCgroupClass int

// QoSCgroupClass constants.
//
// Higher value corresponds to a more important cgroup.
const (
	QoSCgroupClassBesteffort QoSCgroupClass = iota
	QoSCgroupClassBurstable
	QoSCgroupClassGuaranteed
	QoSCgroupClassPodruntime
	QoSCgroupClassSystem
)
