/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package platform

import (
	"log"

	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/runtime/initializer"
)

// Platform represents the Platform task.
type Platform struct{}

// NewPlatformTask initializes and returns an Platform task.
func NewPlatformTask() phase.Task {
	return &Platform{}
}

// TaskFunc returns the runtime function.
func (task *Platform) TaskFunc(mode runtime.Mode) phase.TaskFunc {
	return task.runtime
}

func (task *Platform) runtime(r runtime.Runtime) (err error) {
	i, err := initializer.New(r.Platform().Mode())
	if err != nil {
		return err
	}

	if err = i.Initialize(r.Platform(), r.Config().Machine().Install()); err != nil {
		return err
	}

	hostname, err := r.Platform().Hostname()
	if err != nil {
		return err
	}

	if hostname != nil {
		r.Config().Machine().Network().SetHostname(string(hostname))
	}

	addrs, err := r.Platform().ExternalIPs()
	if err != nil {
		log.Printf("certificates will be created without external IPs: %v\n", err)
	}

	sans := make([]string, 0, len(addrs))
	for _, addr := range addrs {
		sans = append(sans, addr.String())
	}

	if r.Platform().Mode() == runtime.Container {
		// TODO: add ::1 back once I figure out why bootkube barfs
		sans = append(sans, "127.0.0.1")
	}

	r.Config().Machine().Security().SetCertSANs(sans)
	r.Config().Cluster().SetCertSANs(sans)

	return nil
}
