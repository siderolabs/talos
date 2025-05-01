// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"errors"
	"runtime"
)

func (check *preflightCheckContext) verifyPlatformSpecific(ctx context.Context) error {
	return check.verifyAppleMachine(ctx)
}

func (check *preflightCheckContext) verifyAppleMachine(context.Context) error {
	if runtime.GOARCH != "arm64" {
		return errors.New("currently qemu on darwin is supported only on arm machines")
	}

	return nil
}
