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
	"github.com/talos-systems/talos/internal/pkg/blockdevice/bootloader/syslinux"
	"github.com/talos-systems/talos/internal/pkg/blockdevice/probe"
	"github.com/talos-systems/talos/internal/pkg/constants"
	"github.com/talos-systems/talos/internal/pkg/install"
	"github.com/talos-systems/talos/internal/pkg/kernel"
	"github.com/talos-systems/talos/internal/pkg/kubernetes"
	"github.com/talos-systems/talos/internal/pkg/mount"
	"github.com/talos-systems/talos/pkg/userdata"

	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/pkg/transport"

	"golang.org/x/sys/unix"

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

	if err = upgradeRoot(url); err != nil {
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

func upgradeRoot(url string) (err error) {
	// Identify next root disk
	var dev *probe.ProbedBlockDevice
	if dev, err = probe.GetDevWithFileSystemLabel(constants.NextRootPartitionLabel()); err != nil {
		return
	}

	// Should we handle anything around the disk/partition itself? maybe Format()
	rootTarget := install.Target{
		Label:          constants.NextRootPartitionLabel(),
		MountPoint:     "/var/nextroot",
		Device:         dev.Path,
		FileSystemType: "xfs",
		Force:          true,
		PartitionName:  dev.Path,
		Assets:         []*install.Asset{},
	}

	rootTarget.Assets = append(rootTarget.Assets, &install.Asset{
		Source:      url + "/" + constants.RootfsAsset,
		Destination: constants.RootfsAsset,
	})

	if err = rootTarget.Format(); err != nil {
		return
	}

	// mount up 'next' root disk rw
	mountpoint := mount.NewMountPoint(dev.Path, rootTarget.MountPoint, dev.SuperBlock.Type(), unix.MS_NOATIME, "")

	if err = mount.WithRetry(mountpoint); err != nil {
		return errors.Errorf("error mounting partitions: %v", err)
	}

	// Ensure we unmount the new rootfs in case of failure
	// nolint: errcheck
	defer mount.UnWithRetry(mountpoint)

	// install assets
	return rootTarget.Install()
}

func upgradeBoot(url string) error {
	bootTarget := install.Target{
		Label:      constants.BootPartitionLabel,
		MountPoint: "/boot",
		Assets:     []*install.Asset{},
	}

	// Kernel
	bootTarget.Assets = append(bootTarget.Assets, &install.Asset{
		Source:      url + "/" + constants.KernelAsset,
		Destination: filepath.Join(constants.NextRootPartitionLabel(), constants.KernelAsset),
	})

	// Initramfs
	bootTarget.Assets = append(bootTarget.Assets, &install.Asset{
		Source:      url + "/" + constants.InitramfsAsset,
		Destination: filepath.Join(constants.NextRootPartitionLabel(), constants.InitramfsAsset),
	})

	var err error
	if err = bootTarget.Install(); err != nil {
		return err
	}

	// TODO: Figure out a method to update kernel args
	nextCmdline := kernel.NewCmdline(kernel.Cmdline().String())

	// talos.root=
	talosRoot := kernel.NewParameter(constants.KernelCurrentRoot)
	talosRoot.Append(constants.NextRootPartitionLabel())
	if root := nextCmdline.Get(constants.KernelCurrentRoot); root == nil {
		nextCmdline.Append(constants.KernelCurrentRoot, *(talosRoot.First()))
	} else {
		nextCmdline.Set(constants.KernelCurrentRoot, talosRoot)
	}

	// initrd=
	initParam := kernel.NewParameter("initrd")
	initParam.Append(filepath.Join("/", constants.NextRootPartitionLabel(), constants.InitramfsAsset))
	if initrd := nextCmdline.Get("initrd"); initrd == nil {
		nextCmdline.Append("initrd", *(initParam.First()))
	} else {
		nextCmdline.Set("initrd", initParam)
	}

	// Create bootloader config
	bootcfg := &syslinux.Cfg{
		Default: constants.NextRootPartitionLabel(),
		Labels: []*syslinux.Label{
			{
				Root:   constants.NextRootPartitionLabel(),
				Kernel: filepath.Join("/", constants.NextRootPartitionLabel(), constants.KernelAsset),
				Initrd: filepath.Join("/", constants.NextRootPartitionLabel(), constants.InitramfsAsset),
				Append: nextCmdline.String(),
			},
			// TODO see if we can omit this section and/or pass in only the root(?)
			// so we can make use of existing boot config
			{
				Root:   constants.CurrentRootPartitionLabel(),
				Kernel: filepath.Join("/", constants.CurrentRootPartitionLabel(), constants.KernelAsset),
				Initrd: filepath.Join("/", constants.CurrentRootPartitionLabel(), constants.InitramfsAsset),
				Append: kernel.Cmdline().String(),
			},
		},
	}

	return syslinux.Install(constants.BootMountPoint, bootcfg)
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

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

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
