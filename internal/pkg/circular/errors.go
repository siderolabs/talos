// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package circular

import "errors"

// ErrClosed is raised on read from closed Reader.
var ErrClosed = errors.New("reader is closed")

// ErrSeekBeforeStart is raised when seek goes beyond start of the file.
var ErrSeekBeforeStart = errors.New("seek before start")

// ErrOutOfSync is raised when reader got too much out of sync with the writer.
var ErrOutOfSync = errors.New("buffer overrun, read position overwritten")
