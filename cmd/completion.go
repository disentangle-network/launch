package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate shell completion scripts",
	Long: `Generate shell completion scripts for launch-disentangle.

To load completions:

Bash:
  $ source <(launch-disentangle completion bash)
  # To load completions for each session, execute once:
  # Linux:
  $ launch-disentangle completion bash > /etc/bash_completion.d/launch-disentangle
  # macOS:
  $ launch-disentangle completion bash > $(brew --prefix)/etc/bash_completion.d/launch-disentangle

Zsh:
  $ source <(launch-disentangle completion zsh)
  # To load completions for each session, execute once:
  $ launch-disentangle completion zsh > "${fpath[1]}/_launch-disentangle"

Fish:
  $ launch-disentangle completion fish | source
  # To load completions for each session, execute once:
  $ launch-disentangle completion fish > ~/.config/fish/completions/launch-disentangle.fish

PowerShell:
  PS> launch-disentangle completion powershell | Out-String | Invoke-Expression
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE: func(cmd *cobra.Command, args []string) error {
		switch args[0] {
		case "bash":
			return cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			return cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			return cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
		}
		return fmt.Errorf("unsupported shell: %s", args[0])
	},
}

func init() {
	rootCmd.AddCommand(completionCmd)
}
