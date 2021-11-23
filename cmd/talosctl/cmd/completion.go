// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/talos-systems/talos/pkg/cli"
)

// completionCmd represents the completion command.
var completionCmd = &cobra.Command{
	Use:   "completion SHELL",
	Short: "Output shell completion code for the specified shell (bash, fish or zsh)",
	Long: `Output shell completion code for the specified shell (bash, fish or zsh).
The shell code must be evaluated to provide interactive
completion of talosctl commands.  This can be done by sourcing it from
the .bash_profile.

Note for zsh users: [1] zsh completions are only supported in versions of zsh >= 5.2`,
	Example: `# Installing bash completion on macOS using homebrew
## If running Bash 3.2 included with macOS
	brew install bash-completion
## or, if running Bash 4.1+
	brew install bash-completion@2
## If talosctl is installed via homebrew, this should start working immediately.
## If you've installed via other means, you may need add the completion to your completion directory
	talosctl completion bash > $(brew --prefix)/etc/bash_completion.d/talosctl

# Installing bash completion on Linux
## If bash-completion is not installed on Linux, please install the 'bash-completion' package
## via your distribution's package manager.
## Load the talosctl completion code for bash into the current shell
	source <(talosctl completion bash)
## Write bash completion code to a file and source if from .bash_profile
	talosctl completion bash > ~/.talos/completion.bash.inc
	printf "
		# talosctl shell completion
		source '$HOME/.talos/completion.bash.inc'
		" >> $HOME/.bash_profile
	source $HOME/.bash_profile
# Load the talosctl completion code for fish[1] into the current shell
	talosctl completion fish | source
# Set the talosctl completion code for fish[1] to autoload on startup
    talosctl completion fish > ~/.config/fish/completions/talosctl.fish
# Load the talosctl completion code for zsh[1] into the current shell
	source <(talosctl completion zsh)
# Set the talosctl completion code for zsh[1] to autoload on startup
    talosctl completion zsh > "${fpath[1]}/_talosctl"`,
	ValidArgs: []string{"bash", "fish", "zsh"},
	Args:      cobra.ExactValidArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) != 1 {
			cli.Should(cmd.Usage())
			os.Exit(1)
		}

		switch args[0] {
		case "bash":
			return rootCmd.GenBashCompletion(os.Stdout)
		case "fish":
			return rootCmd.GenFishCompletion(os.Stdout, true)
		case "zsh":
			err := rootCmd.GenZshCompletion(os.Stdout)
			// cobra does not hook the completion, so let's do it manually
			fmt.Printf("compdef _talosctl talosctl")

			return err
		default:
			return fmt.Errorf("unsupported shell %q", args[0])
		}
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
