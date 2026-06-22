// rocway-cli 脚手架工具：cobra 顶层维护命令树，子命令内部参数解析使用标准库 flag。
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	cmdcli "github.com/cuiyuanxin/roc_way/cmd/rocway-cli/cmd"
)

func main() {
	root := &cobra.Command{
		Use:   "rocway-cli",
		Short: "rocway framework scaffolding CLI",
	}
	root.AddCommand(cmdcli.NewCmd(), cmdcli.GenCmd(), cmdcli.VersionCmd())
	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
