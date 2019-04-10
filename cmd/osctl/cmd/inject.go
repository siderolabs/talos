/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cmd

import (
	"fmt"
	"io/ioutil"

	"github.com/spf13/cobra"
	"github.com/talos-systems/talos/cmd/osctl/pkg/helpers"
	"github.com/talos-systems/talos/pkg/crypto/x509"
	"github.com/talos-systems/talos/pkg/userdata"
	yaml "gopkg.in/yaml.v2"
)

// injectCmd represents the inject command
var injectCmd = &cobra.Command{
	Use:   "inject",
	Short: "inject data into fields in the user data.",
	Long:  ``,
}

// injectOSCmd represents the inject command
// nolint: dupl
var injectOSCmd = &cobra.Command{
	Use:   "os",
	Short: "inject OS data.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if err := inject(args, crt, key, injectOSData); err != nil {
			helpers.Fatalf("%s", err)
		}
	},
}

// injectIdentityCmd represents the inject command
// nolint: dupl
var injectIdentityCmd = &cobra.Command{
	Use:   "identity",
	Short: "inject identity data.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if err := inject(args, crt, key, injectIdentityData); err != nil {
			helpers.Fatalf("%s", err)
		}
	},
}

// injectKubernetesCmd represents the inject command
// nolint: dupl
var injectKubernetesCmd = &cobra.Command{
	Use:   "kubernetes",
	Short: "inject Kubernetes data.",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if err := inject(args, crt, key, injectKubernetesData); err != nil {
			helpers.Fatalf("%s", err)
		}
	},
}

// nolint: dupl
func injectOSData(u *userdata.UserData, crt, key string) (err error) {
	if u.Security == nil {
		u.Security = newSecurity()
	}
	crtAndKey, err := x509.NewCertificateAndKeyFromFiles(crt, key)
	if err != nil {
		return
	}
	u.Security.OS.CA = crtAndKey

	return nil
}

// nolint: dupl
func injectIdentityData(u *userdata.UserData, crt, key string) (err error) {
	if u.Security == nil {
		u.Security = newSecurity()
	}
	crtAndKey, err := x509.NewCertificateAndKeyFromFiles(crt, key)
	if err != nil {
		return
	}
	u.Security.OS.Identity = crtAndKey

	return nil
}

// nolint: dupl
func injectKubernetesData(u *userdata.UserData, crt, key string) (err error) {
	if u.Security == nil {
		u.Security = newSecurity()
	}
	crtAndKey, err := x509.NewCertificateAndKeyFromFiles(crt, key)
	if err != nil {
		return
	}
	u.Security.Kubernetes.CA = crtAndKey

	return nil
}

func inject(args []string, crt, key string, f func(*userdata.UserData, string, string) error) (err error) {
	if len(args) != 1 {
		err = fmt.Errorf("expected 1 argument, got %d", len(args))
		return
	}

	configBytes, err := ioutil.ReadFile(args[0])
	if err != nil {
		return
	}

	data := &userdata.UserData{}
	if err = yaml.Unmarshal(configBytes, data); err != nil {
		return
	}

	if err = f(data, crt, key); err != nil {
		return
	}

	dataBytes, err := yaml.Marshal(data)
	if err != nil {
		return
	}
	if err = ioutil.WriteFile(args[0], dataBytes, 0600); err != nil {
		return
	}

	return nil
}

func newSecurity() *userdata.Security {
	return &userdata.Security{
		OS:         &userdata.OSSecurity{},
		Kubernetes: &userdata.KubernetesSecurity{},
	}
}

func init() {
	injectCmd.PersistentFlags().StringVar(&crt, "crt", "", "the path to the PKI certificate")
	helpers.Should(injectCmd.MarkPersistentFlagRequired("crt"))
	injectCmd.PersistentFlags().StringVar(&key, "key", "", "the path to the PKI key")
	helpers.Should(injectCmd.MarkPersistentFlagRequired("key"))
	injectCmd.AddCommand(injectOSCmd, injectIdentityCmd, injectKubernetesCmd)
	rootCmd.AddCommand(injectCmd)
}
