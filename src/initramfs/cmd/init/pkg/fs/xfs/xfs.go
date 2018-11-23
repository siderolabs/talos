// Package xfs provides an interface to xfsprogs.
package xfs

import (
	"os"
	"os/exec"
)

// GrowFS expands a XFS filesystem to the maximum possible. The partition
// MUST be mounted, or this will fail.
func GrowFS(partname string) error {
	return cmd("xfs_growfs", "-d", partname)
}

// MakeFS creates a XFS filesystem on the specified partition
func MakeFS(partname string) error {
	return cmd("mkfs.xfs", partname)
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
