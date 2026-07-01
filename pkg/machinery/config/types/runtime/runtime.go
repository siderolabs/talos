// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package runtime provides runtime machine configuration documents.
package runtime

//go:generate go tool github.com/siderolabs/talos/tools/docgen -output runtime_doc.go runtime.go kmsg_log.go event_sink.go environment.go oom.go sysctl.go sysfs.go etc_file.go udev_rules.go unattended_install.go watchdog_timer.go kernel_module.go

//go:generate go tool github.com/siderolabs/deep-copy -type EventSinkV1Alpha1 -type EnvironmentV1Alpha1 -type KmsgLogV1Alpha1 -type OOMV1Alpha1 -type SysctlConfigV1Alpha1 -type SysfsConfigV1Alpha1 -type EtcFileConfigV1Alpha1 -type UdevRulesConfigV1Alpha1 -type UnattendedInstallConfigV1Alpha1 -type WatchdogTimerV1Alpha1 -type KernelModuleConfigV1Alpha1 -pointer-receiver -header-file ../../../../../hack/boilerplate.txt -o deep_copy.generated.go .
