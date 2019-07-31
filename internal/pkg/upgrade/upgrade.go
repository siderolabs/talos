/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package upgrade

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/installer/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/pkg/installer/manifest"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/kubernetes"
	"github.com/talos-systems/talos/pkg/userdata"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"

	yaml "gopkg.in/yaml.v2"
)

// NewUpgrade initiates a Talos upgrade
// nolint: gocyclo
func NewUpgrade(url string) (err error) {
	var hostname string
	if hostname, err = os.Hostname(); err != nil {
		return
	}

	data, err := userdata.Open(constants.UserDataPath)
	if err != nil {
		return err
	}
	if data, err = data.Upgrade(); err != nil {
		return err
	}
	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return err
	}
	if err = ioutil.WriteFile(constants.UserDataPath, dataBytes, 0400); err != nil {
		return err
	}

	if err = upgradeBoot(url); err != nil {
		return err
	}

	// cordon/drain
	var kubeHelper *kubernetes.Helper
	if kubeHelper, err = kubernetes.NewHelper(); err != nil {
		return err
	}

	if err = kubeHelper.CordonAndDrain(hostname); err != nil {
		return err
	}

	if data.Services.Kubeadm.IsControlPlane() {
		var hostname string
		if hostname, err = os.Hostname(); err != nil {
			return
		}

		if err = leaveEtcd(hostname); err != nil {
			return err
		}

		if err = os.RemoveAll("/var/lib/etcd"); err != nil {
			return err
		}
	}

	return err
}

func upgradeBoot(url string) error {
	bootTarget := manifest.Target{
		Label:      constants.BootPartitionLabel,
		MountPoint: constants.BootMountPoint,
		Assets:     []*manifest.Asset{},
	}

	// Kernel
	bootTarget.Assets = append(bootTarget.Assets, &manifest.Asset{
		Source:      url + "/" + constants.KernelAsset,
		Destination: filepath.Join("/", "default", constants.KernelAsset),
	})

	// Initramfs
	bootTarget.Assets = append(bootTarget.Assets, &manifest.Asset{
		Source:      url + "/" + constants.InitramfsAsset,
		Destination: filepath.Join("/", "default", constants.InitramfsAsset),
	})

	var err error
	if err = bootTarget.Save(); err != nil {
		return err
	}

	// TODO: Figure out a method to update kernel args
	nextCmdline := kernel.NewCmdline(kernel.ProcCmdline().String())

	// Set the initrd kernel paramaeter.
	initParam := kernel.NewParameter("initrd")
	initParam.Append(filepath.Join("/", "default", constants.InitramfsAsset))
	if initrd := nextCmdline.Get("initrd"); initrd == nil {
		nextCmdline.Append("initrd", *(initParam.First()))
	} else {
		nextCmdline.Set("initrd", initParam)
	}

	// Create bootloader config
	syslinuxcfg := &syslinux.Cfg{
		Default: "default",
		Labels: []*syslinux.Label{
			{
				Root:   "default",
				Kernel: filepath.Join("/", "default", constants.KernelAsset),
				Initrd: filepath.Join("/", "default", constants.InitramfsAsset),
				Append: nextCmdline.String(),
			},
		},
	}

	return syslinux.Install(constants.BootMountPoint, syslinuxcfg)
}

// Reset calls kubeadm reset to clean up a kubernetes installation
func Reset() (err error) {
	// TODO find some way to flex on debug info
	// can add in -v10 for additional debugging
	cmd := exec.Command(
		"kubeadm",
		"reset",
		"--force",
	)

	return cmd.Run()
}

func leaveEtcd(hostname string) (err error) {
	tlsInfo := transport.TLSInfo{
		CertFile:      constants.KubeadmEtcdPeerCert,
		KeyFile:       constants.KubeadmEtcdPeerKey,
		TrustedCAFile: constants.KubeadmEtcdCACert,
	}
	tlsConfig, err := tlsInfo.ClientConfig()
	if err != nil {
		return err
	}
	cli, err := clientv3.New(clientv3.Config{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
		TLS:         tlsConfig,
	})
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer cli.Close()

	resp, err := cli.MemberList(context.Background())
	if err != nil {
		return err
	}

	var id *uint64
	for _, member := range resp.Members {
		if member.Name == hostname {
			id = &member.ID
		}
	}
	if id == nil {
		return errors.Errorf("failed to find %q in list of etcd members", hostname)
	}

	log.Println("leaving etcd cluster")
	_, err = cli.MemberRemove(context.Background(), *id)
	if err != nil {
		return err
	}

	return nil
}
