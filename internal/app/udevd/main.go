/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

// Package udevd is a library for working with uevent messages from the netlink
// socket.
package main

import (
	"fmt"
	"log"
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/talos-systems/talos/internal/app/udevd/internal/drivers/scsi"
	"github.com/talos-systems/talos/internal/app/udevd/internal/uevent"
)

func init() {
	log.SetFlags(log.Lshortfile | log.Ldate | log.Lmicroseconds | log.Ltime)
}

func watch() error {
	k, err := uevent.Dial()
	if err != nil {
		return err
	}
	// nolint: errcheck
	defer k.Close()

	for u := range k.Watch() {
		if u.Error != nil {
			log.Printf("uevent error: %+v\n", err)
			continue
		}
		if err := handle(u.Message); err != nil {
			log.Printf("event handler error: %+v\n", err)
		}
	}

	return nil
}

// nolint: gocyclo
func handle(message *uevent.Message) (err error) {
	if message.Subsystem == uevent.SubsystemBlock {
		var devname, devtype, partn string
		var ok bool

		if devname, ok = message.Values["DEVNAME"]; !ok {
			return errors.Errorf("DEVNAME not found\n")
		}

		devpath := path.Join("/dev", devname)
		devtype = message.Values["DEVTYPE"]

		var device *scsi.Device
		if device, err = scsi.NewDevice(devpath); err != nil {
			return errors.Errorf("error opening %s: %+v\n", devpath, err)
		}
		// nolint: errcheck
		defer device.Close()

		if err = device.Identify(); err != nil {
			return errors.Errorf("error identifying %s: %+v\n", devpath, err)
		}

		if device.WWN == "" {
			log.Printf("no wwn found for %s\n", devpath)
			return nil
		}

		oldname := fmt.Sprintf("../../%s", devname)
		newname := fmt.Sprintf("/dev/disk/by-id/wwn-%s", device.WWN)

		if partn, ok = message.Values["PARTN"]; ok && devtype == "partition" {
			newname += "-part" + partn
		}

		switch message.Action {
		case uevent.ActionAdd:
			log.Printf("creating symlink %s -> %s\n", newname, oldname)

			if _, err = os.Lstat(newname); err == nil {
				if err = os.Remove(newname); err != nil {
					log.Printf("failed to remove symlink: %v\n", err)
				}
			}

			if err = os.Symlink(oldname, newname); err != nil {
				return errors.Errorf("failed to create symlink %s: %+v", newname, err)
			}
		case uevent.ActionRemove:
			log.Printf("removing symlink %s -> %s\n", newname, oldname)

			if _, err = os.Lstat(newname); err == nil {
				if err = os.Remove(newname); err != nil {
					log.Printf("failed to remove symlink: %v\n", err)
				}
			}
		default:
			log.Printf("unhandled action %q on %s", message.Action, devname)
		}

	}

	return nil
}

func main() {
	if err := os.MkdirAll("/dev/disk/by-id", os.ModeDir); err != nil && !os.IsExist(err) {
		log.Printf("failed to create directoy /dev/disk/by-id: %+v\n", err)
		os.Exit(1)
	}

	if err := watch(); err != nil {
		log.Printf("failed watch uevents: %+v\n", err)
		os.Exit(1)
	}
}
