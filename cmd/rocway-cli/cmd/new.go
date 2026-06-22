// `rocway-cli new` 拷贝脚手架模板到目标目录。
//
// 内部参数使用标准库 flag 解析（简单子命令：≤2 个 flag）。
package cmd

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a new project from scaffold",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// 简单子命令内部走标准库 flag
			fs := flag.NewFlagSet("new", flag.ContinueOnError)
			module := fs.String("m", "", "go module path, e.g. github.com/me/myapp")
			dest := fs.String("o", ".", "output directory")
			if err := fs.Parse(cmd.Flags().Args()); err != nil {
				return err
			}
			return runNew(args[0], *module, *dest)
		},
	}
	return c
}

func runNew(name, module, dest string) error {
	if name == "" {
		return errors.New("name required")
	}
	if module == "" {
		module = "github.com/cuiyuanxin/roc_way/" + name
	}
	target := filepath.Join(dest, name)
	if err := copyScaffold(target); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(target, ".module"), []byte(module), 0o644); err != nil {
		return err
	}
	fmt.Printf("✔ created %s (module=%s)\n", target, module)
	return nil
}

// copyScaffold 从 assets/scaffold 拷贝到 target。
func copyScaffold(target string) error {
	src := "assets/scaffold"
	if _, err := os.Stat(src); err != nil {
		// 离线友好：无模板时创建空目录
		return os.MkdirAll(target, 0o755)
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		dst := filepath.Join(target, rel)
		if info.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		return copyFile(path, dst)
	})
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
