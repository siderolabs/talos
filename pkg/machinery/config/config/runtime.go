// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"iter"
	"maps"
	"net/url"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/cel"
	"github.com/siderolabs/talos/pkg/machinery/cel/celenv"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// RuntimeConfig defines the interface to access Talos runtime configuration.
type RuntimeConfig interface {
	EventsEndpoint() *string
	KmsgLogURLs() []*url.URL
	WatchdogTimer() WatchdogTimerConfig
	FilesystemScrub() []FilesystemScrubConfig
}

// EnvironmentConfig defines the interface to access Talos environment configuration.
type EnvironmentConfig interface {
	Variables() map[string]string
}

// WrapEnvironmentConfigList wraps a list of EnvironmentConfig into a single EnvironmentConfig aggregating the results.
func WrapEnvironmentConfigList(configs ...EnvironmentConfig) EnvironmentConfig {
	return environmentConfigWrapper(configs)
}

type environmentConfigWrapper []EnvironmentConfig

func (w environmentConfigWrapper) Variables() map[string]string {
	return mergeMaps(w, func(c EnvironmentConfig) iter.Seq2[string, string] {
		return maps.All(c.Variables())
	})
}

// WatchdogTimerConfig defines the interface to access Talos watchdog timer configuration.
type WatchdogTimerConfig interface {
	Device() string
	Timeout() time.Duration
}

// FilesystemScrubConfig defines the interface to access Talos filesystem scrub configuration.
type FilesystemScrubConfig interface {
	Name() string
	Mountpoint() string
	Period() time.Duration
}

// WrapRuntimeConfigList wraps a list of RuntimeConfig into a single RuntimeConfig aggregating the results.
func WrapRuntimeConfigList(configs ...RuntimeConfig) RuntimeConfig {
	return runtimeConfigWrapper(configs)
}

type runtimeConfigWrapper []RuntimeConfig

func (w runtimeConfigWrapper) EventsEndpoint() *string {
	return findFirstValue(w, func(c RuntimeConfig) *string {
		return c.EventsEndpoint()
	})
}

func (w runtimeConfigWrapper) KmsgLogURLs() []*url.URL {
	return aggregateValues(w, func(c RuntimeConfig) []*url.URL {
		return c.KmsgLogURLs()
	})
}

func (w runtimeConfigWrapper) WatchdogTimer() WatchdogTimerConfig {
	return findFirstValue(w, func(c RuntimeConfig) WatchdogTimerConfig {
		return c.WatchdogTimer()
	})
}

func (w runtimeConfigWrapper) FilesystemScrub() []FilesystemScrubConfig {
	return aggregateValues(w, func(c RuntimeConfig) []FilesystemScrubConfig {
		return c.FilesystemScrub()
	})
}

// OOMConfig defines the interface to access OOM configuration.
type OOMConfig interface {
	TriggerExpression() cel.Expression
	CgroupRankingExpression() cel.Expression
	StrictCgroupClassOrdering() bool
	SampleInterval() time.Duration
}

// DefaultOOMConfig provides default OOM configuration values.
type DefaultOOMConfig struct{}

// TriggerExpression implements OOMConfig interface, returning the default OOM trigger expression.
func (DefaultOOMConfig) TriggerExpression() cel.Expression {
	return cel.MustExpression(
		cel.ParseBooleanExpression(
			constants.DefaultOOMTriggerExpression,
			celenv.OOMTrigger(),
		),
	)
}

// CgroupRankingExpression implements OOMConfig interface, returning the default cgroup ranking expression.
//
// Sort processes by the following hierarchy:
// First, sort by high-level group:
//
//	kubepods (workloads)
//	podruntime (CRI, kubelet, etcd)
//	runtime (core containerd, system services)
//	init
//
// Second, inside kubepods we have QoS groups:
//
//	first priority: BestEffort
//	second: Burstable
//	last: Guaranteed
//
// Third, look into other attributes, e.g. OOM score.
// Fourth, look into memory max - memory current (if memory max is set).
//
// Sort to make the most prioritized to OOM-kill cgroup to the first place.
func (DefaultOOMConfig) CgroupRankingExpression() cel.Expression {
	return cel.MustExpression(
		cel.ParseDoubleExpression(
			constants.DefaultOOMCgroupRankingExpression,
			celenv.OOMCgroupScoring(),
		),
	)
}

// StrictCgroupClassOrdering implements OOMConfig interface, returning the default value for strict cgroup class ordering.
func (DefaultOOMConfig) StrictCgroupClassOrdering() bool {
	return constants.DefaultOOMStrictCgroupClassOrdering
}

// SampleInterval implements OOMConfig interface, returning the default OOM sample interval.
func (DefaultOOMConfig) SampleInterval() time.Duration {
	return constants.DefaultOOMSampleInterval
}
