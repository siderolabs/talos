// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package global

import (
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// InsecureFlags is a mix-in args struct for commands that support the --insecure flag.
type InsecureFlags struct {
	Insecure         bool
	CertFingerprints []string
}

// AddFlags adds the InsecureFlags flags to the given command.
func (a *InsecureFlags) AddFlags(cmd *cobra.Command) {
	a.addFlags(cmd.Flags())
}

// AddPersistentFlags adds the InsecureFlags flags to the given command and to its subcommands.
//
// Use it on a command that groups subcommands instead of running itself: cobra merges a parent's
// persistent flags into its subcommands, but not its local ones.
func (a *InsecureFlags) AddPersistentFlags(cmd *cobra.Command) {
	a.addFlags(cmd.PersistentFlags())
}

func (a *InsecureFlags) addFlags(flags *pflag.FlagSet) {
	flags.BoolVarP(&a.Insecure, "insecure", "i", false, "use the insecure (encrypted with no auth) maintenance service")
	flags.StringSliceVar(&a.CertFingerprints, "cert-fingerprint", nil, "list of server certificate fingerprints to accept (defaults to no check, only used with --insecure flag)")
}

// GetInsecureFlag returns the value of the --insecure flag.
func (a *InsecureFlags) GetInsecureFlag() bool {
	return a.Insecure
}

// GetCertFingerprints returns the value of the --cert-fingerprint flag.
func (a *InsecureFlags) GetCertFingerprints() []string {
	return a.CertFingerprints
}

// InsecureArgser is an interface for commands that support the --insecure flag.
type InsecureArgser interface {
	GetInsecureFlag() bool
	GetCertFingerprints() []string
}

// Check interface implementation.
var _ InsecureArgser = (*InsecureFlags)(nil)
