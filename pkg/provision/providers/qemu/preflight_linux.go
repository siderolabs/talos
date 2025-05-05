// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package qemu

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/coreos/go-iptables/iptables"
	"github.com/hashicorp/go-getter/v2"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func (check *preflightCheckContext) verifyPlatformSpecific(ctx context.Context) error {
	for _, check := range []func(ctx context.Context) error{
		check.cniDirectories,
		check.cniBundle,
		check.checkIptables,
		check.swtpmExecutable,
		check.checkKVM,
	} {
		if err := check(ctx); err != nil {
			return err
		}
	}

	return nil
}

func (check *preflightCheckContext) cniDirectories(ctx context.Context) error {
	cniDirs := append([]string{}, check.request.Network.CNI.BinPath...)
	cniDirs = append(cniDirs, check.request.Network.CNI.CacheDir, check.request.Network.CNI.ConfDir)

	for _, cniDir := range cniDirs {
		st, err := os.Stat(cniDir)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("error checking CNI directory %q: %w", cniDir, err)
			}

			fmt.Fprintf(check.options.LogWriter, "creating %q\n", cniDir)

			err = os.MkdirAll(cniDir, 0o777)
			if err != nil {
				return err
			}

			continue
		}

		if !st.IsDir() {
			return fmt.Errorf("CNI path %q exists, but it's not a directory", cniDir)
		}
	}

	return nil
}

func (check *preflightCheckContext) cniBundle(ctx context.Context) error {
	var missing bool

	requiredCNIPlugins := []string{"bridge", "firewall", "static", "tc-redirect-tap"}

	for _, cniPlugin := range requiredCNIPlugins {
		missing = true

		for _, binPath := range check.request.Network.CNI.BinPath {
			_, err := os.Stat(filepath.Join(binPath, cniPlugin))
			if err == nil {
				missing = false

				break
			}
		}

		if missing {
			break
		}
	}

	if !missing {
		return nil
	}

	if check.request.Network.CNI.BundleURL == "" {
		return fmt.Errorf("error: required CNI plugins %q were not found in %q", requiredCNIPlugins, check.request.Network.CNI.BinPath)
	}

	pwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := getter.Client{}
	src := strings.ReplaceAll(check.request.Network.CNI.BundleURL, constants.ArchVariable, runtime.GOARCH)
	dst := check.request.Network.CNI.BinPath[0]

	fmt.Fprintf(check.options.LogWriter, "downloading CNI bundle from %q to %q\n", src, dst)

	_, err = client.Get(ctx, &getter.Request{
		Src:     src,
		Dst:     dst,
		Pwd:     pwd,
		GetMode: getter.ModeDir,
	})

	return err
}

func (check *preflightCheckContext) checkIptables(ctx context.Context) error {
	_, err := iptables.New()
	if err != nil {
		return fmt.Errorf("error accessing iptables: %w", err)
	}

	return nil
}

func (check *preflightCheckContext) swtpmExecutable(ctx context.Context) error {
	if check.options.TPM1_2Enabled || check.options.TPM2Enabled {
		if _, err := exec.LookPath("swtpm"); err != nil {
			return fmt.Errorf("swtpm not found in PATH, please install swtpm-tools with the package manager: %w", err)
		}
	}

	return nil
}

func (check *preflightCheckContext) checkKVM(ctx context.Context) error {
	err := checkKVM()
	if err != nil {
		fmt.Printf("error opening /dev/kvm, please make sure KVM support is enabled in Linux kernel: %s\n", err)
		fmt.Println("running without KVM")
	}

	return nil
}

func checkKVM() error {
	f, err := os.OpenFile("/dev/kvm", os.O_RDWR, 0)
	if err != nil {
		return err
	}

	return f.Close()
}
