package cmd

import "github.com/spf13/cobra"

// VersionCmd 返回 version 子命令。
func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version",
		Run: func(cmd *cobra.Command, _ []string) {
			cmd.Println("rocway-cli v0.1.0")
		},
	}
}
