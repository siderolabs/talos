// Package xfs provides an interface to xfsprogs.
package xfs

import (
	"os"
	"os/exec"
)

// GrowFS expands an XFS filesystem to the maximum possible. The partition
// MUST be mounted, or this will fail.
func GrowFS(partname string) error {
	return cmd("xfs_growfs", "-d", partname)
}

func cmd(name string, arg ...string) error {
	cmd := exec.Command(name, arg...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stdout
	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}
