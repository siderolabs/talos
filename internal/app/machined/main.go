// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"golang.org/x/net/http/httpproxy"
	"golang.org/x/sys/unix"

	v1alpha1server "github.com/talos-systems/talos/internal/app/machined/internal/server/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	v1alpha1runtime "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/syslinux"
	"github.com/talos-systems/talos/pkg/grpc/factory"
	"github.com/talos-systems/talos/pkg/universe"
)

func init() {
	// Explicitly set the default http client transport to work around proxy.Do
	// once. This is the http.DefaultTransport with the Proxy func overridden so
	// that the environment variables with be reread/initialized each time the
	// http call is made.
	http.DefaultClient.Transport = &http.Transport{
		Proxy: func(req *http.Request) (*url.URL, error) {
			return httpproxy.FromEnvironment().ProxyFunc()(req.URL)
		},
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

func reboot(err error) {
	log.Print(err)

	if err := revert(); err != nil {
		log.Printf("failed to revert upgrade: %v", err)
	}

	for i := 10; i >= 0; i-- {
		log.Printf("rebooting in %d seconds\n", i)
		time.Sleep(1 * time.Second)
	}

	v1alpha1runtime.SyncNonVolatileStorageBuffers()

	if unix.Reboot(unix.LINUX_REBOOT_CMD_RESTART) == nil {
		select {}
	}
}

// nolint: gocyclo
func revert() (err error) {
	f, err := os.OpenFile(syslinux.SyslinuxLdlinux, os.O_RDWR, 0700)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	// nolint: errcheck
	defer f.Close()

	adv, err := syslinux.NewADV(f)
	if err != nil {
		return err
	}

	label, ok := adv.ReadTag(syslinux.AdvUpgrade)
	if !ok {
		return nil
	}

	if label == "" {
		adv.DeleteTag(syslinux.AdvUpgrade)

		if _, err = f.Write(adv); err != nil {
			return err
		}

		return nil
	}

	log.Printf("reverting default boot to %q", label)

	var b []byte

	if b, err = ioutil.ReadFile(syslinux.SyslinuxConfig); err != nil {
		return err
	}

	re := regexp.MustCompile(`^DEFAULT\s(.*)`)
	matches := re.FindSubmatch(b)

	if len(matches) != 2 {
		return fmt.Errorf("expected 2 matches, got %d", len(matches))
	}

	b = re.ReplaceAll(b, []byte(fmt.Sprintf("DEFAULT %s", label)))

	if err = ioutil.WriteFile(syslinux.SyslinuxConfig, b, 0600); err != nil {
		return err
	}

	adv.DeleteTag(syslinux.AdvUpgrade)

	if _, err = f.Write(adv); err != nil {
		return err
	}

	return nil
}

// nolint: gocyclo
func main() {
	// Initialize the controller without a config.
	c, err := v1alpha1runtime.NewController(nil)
	if err != nil {
		reboot(err)
	}

	// Start event listeners.
	go func() {
		if err = c.ListenForEvents(); err != nil {
			log.Printf("WARNING: signals and ACPI events will be ignored: %+v", err)
		}
	}()

	// Initialize the machine.
	if err = c.Run(runtime.SequenceInitialize, nil); err != nil {
		reboot(err)
	}

	// Reset the controller with a config. This MUST happen before running any
	// other sequences.
	c, err = v1alpha1runtime.NewController(c.Runtime().State().Config())
	if err != nil {
		reboot(err)
	}

	// Start the API server.
	go func() {
		server := &v1alpha1server.Server{
			Controller: c,
		}

		if err = factory.ListenAndServe(server, factory.Network("unix"), factory.SocketPath(universe.MachineSocketPath)); err != nil {
			reboot(err)
		}
	}()

	// Perform an installation if required.
	if c.Runtime().Platform().Mode() != runtime.ModeContainer {
		if !c.Runtime().State().Installed() {
			if err = c.Run(runtime.SequenceInstall, nil); err != nil {
				reboot(err)
			}
		}
	}

	// Boot the machine.
	if err = c.Run(runtime.SequenceBoot, nil); err != nil {
		reboot(err)
	}

	// Wait forever.
	select {}
}
