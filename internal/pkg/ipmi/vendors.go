// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ipmi

// vendors maps IANA enterprise numbers reported by Get Device ID to a vendor name.
//
// This is intentionally a short list of the BMC vendors commonly found in
// server hardware, not a copy of the IANA registry: the numeric ID is always
// reported alongside the name, so an unknown vendor loses no information.
//
// See https://www.iana.org/assignments/enterprise-numbers/.
var vendors = map[uint32]string{
	2:     "IBM",
	9:     "Cisco",
	11:    "HP",
	343:   "Intel",
	674:   "Dell",
	2011:  "Huawei",
	4753:  "AMI",
	5771:  "Cisco",
	7244:  "Quanta",
	10368: "Fujitsu",
	10876: "Supermicro",
	15370: "Gigabyte",
	19046: "Lenovo",
	37945: "Inspur",
	47196: "HPE",
	47488: "Supermicro",
}

// Vendor returns the vendor name for an IANA enterprise number, or an empty
// string if it is not known.
func Vendor(id uint32) string {
	return vendors[id]
}
