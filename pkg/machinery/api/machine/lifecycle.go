// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package machine contains the machine service API definitions.
package machine

import "fmt"

// Fmt formats the pull progress status into a human-readable string.
func (s *LifecycleServiceInstallProgress) Fmt() string {
	switch msg := s.GetResponse().(type) {
	case *LifecycleServiceInstallProgress_Message:
		return msg.Message
	case *LifecycleServiceInstallProgress_ExitCode:
		return fmt.Sprintf("Exit code: %d", msg.ExitCode)
	}

	return ""
}
