package cmd

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/autonomy/talos/internal/pkg/crypto/x509"
	"github.com/autonomy/talos/internal/pkg/userdata"
	"github.com/spf13/cobra"
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
			fmt.Printf("%v\n", err)
			os.Exit(1)
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
			fmt.Printf("%v\n", err)
			os.Exit(1)
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
			fmt.Printf("%v\n", err)
			os.Exit(1)
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
	if err := injectCmd.MarkPersistentFlagRequired("crt"); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}
	injectCmd.PersistentFlags().StringVar(&key, "key", "", "the path to the PKI key")
	if err := injectCmd.MarkPersistentFlagRequired("key"); err != nil {
		fmt.Printf("%v\n", err)
		os.Exit(1)
	}

	injectCmd.AddCommand(injectOSCmd, injectIdentityCmd, injectKubernetesCmd)
	rootCmd.AddCommand(injectCmd)
}
