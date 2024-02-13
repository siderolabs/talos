// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime provides runtime machine configuration documents.
package runtime

//go:generate docgen -output runtime_doc.go runtime.go kmsg_log.go event_sink.go watchdog_timer.go

//go:generate deep-copy -type EventSinkV1Alpha1 -type KmsgLogV1Alpha1 -type WatchdogTimerV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
