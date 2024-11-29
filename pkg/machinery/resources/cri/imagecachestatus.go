// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

// ImageCacheStatus describes image cache status type.
type ImageCacheStatus int

// ImageCacheStatus values.
//
//structprotogen:gen_enum
const (
	ImageCacheStatusUnknown   ImageCacheStatus = iota // unknown
	ImageCacheStatusDisabled                          // disabled
	ImageCacheStatusPreparing                         // preparing
	ImageCacheStatusReady                             // ready
)

// ImageCacheCopyStatus describes image cache copy status type.
type ImageCacheCopyStatus int

// ImageCacheCopyStatus values.
//
//structprotogen:gen_enum
const (
	ImageCacheCopyStatusUnknown ImageCacheCopyStatus = iota // unknown
	ImageCacheCopyStatusSkipped                             // skipped
	ImageCacheCopyStatusPending                             // copying
	ImageCacheCopyStatusReady                               // ready
)
