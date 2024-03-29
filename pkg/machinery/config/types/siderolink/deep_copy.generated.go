// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Code generated by "deep-copy -type ConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go ."; DO NOT EDIT.

package siderolink

import (
	"net/url"
)

// DeepCopy generates a deep copy of *ConfigV1Alpha1.
func (o *ConfigV1Alpha1) DeepCopy() *ConfigV1Alpha1 {
	var cp ConfigV1Alpha1 = *o
	if o.APIUrlConfig.URL != nil {
		cp.APIUrlConfig.URL = new(url.URL)
		*cp.APIUrlConfig.URL = *o.APIUrlConfig.URL
		if o.APIUrlConfig.URL.User != nil {
			cp.APIUrlConfig.URL.User = new(url.Userinfo)
			*cp.APIUrlConfig.URL.User = *o.APIUrlConfig.URL.User
		}
	}
	return &cp
}
