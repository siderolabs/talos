// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// structprotogen is a tool to generate proto files from Go structs.
package main

//nolint:gci
import (
	"fmt"
	"os"
	"path"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/siderolabs/structprotogen/ast"
	"github.com/siderolabs/structprotogen/consts"
	"github.com/siderolabs/structprotogen/loader"
	"github.com/siderolabs/structprotogen/proto"
	"github.com/siderolabs/structprotogen/types"
)

// rootCmd represents the base command when called without any subcommands.
var rootCmd = &cobra.Command{
	Use:     "structprotogen path dest",
	Short:   "This CLI is used to generate proto files from Go structs into one proto file",
	Example: "structprotogen github.com/siderolabs/talos/pkg/machinery/resources/... ./api/resource/definitions",
	Args:    cobra.ExactArgs(2),
	Version: "v1.0.0",
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(args[0], args[1])
	},
	SilenceUsage:      true,
	DisableAutoGenTag: true,
}

func main() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

// TODO(DmitriyMV): get comments for fields

//nolint:gocyclo
func run(pkgPath, dst string) error {
	loadedPkgs, err := loader.LoadPackages(pkgPath)
	if err != nil {
		return err
	}

	constants, err := consts.FindIn(loadedPkgs)
	if err != nil {
		return err
	}

	taggedStructs := ast.FindAllTaggedStructs(loadedPkgs)
	printFoundStructs(taggedStructs)

	sortedPkgs, err := types.FindPkgDecls(taggedStructs, loadedPkgs)
	if err != nil {
		return fmt.Errorf("error finding path '%s' declarations: %w", pkgPath, err)
	}

	pkgsTypes, err := types.ParseDeclsData(sortedPkgs, taggedStructs)
	if err != nil {
		return fmt.Errorf("error parsing path '%s' declarations data: %w", pkgPath, err)
	}

	externalTypes := types.FindExternalTypes(pkgsTypes, taggedStructs)
	for i := 0; i < externalTypes.Len(); i++ {
		externalType := externalTypes.Get(i)

		if constants.HaveType(externalType.Pkg, externalType.Name) {
			continue
		}

		if !proto.IsSupportedExternalType(externalType) {
			return fmt.Errorf("external type '%s.%s' is not supported", externalType.Pkg, externalType.Name)
		}
	}

	data := proto.PrepareProtoData(pkgsTypes, constants)

	for i := 0; i < data.Len(); i++ {
		protoData := data.Get(i)

		fmt.Println("--------")
		protoData.WriteDebug(os.Stdout)
	}

	err = os.MkdirAll(dst, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create directory for proto files: %w", err)
	}

	if len(constants) > 0 {
		err = withFile(filepath.Join(dst, "enums", "enums.proto"), func(f *os.File) error {
			return constants.FormatProtoFile(f)
		})
		if err != nil {
			return fmt.Errorf("failed to write enums proto file: %w", err)
		}
	}

	for i := 0; i < data.Len(); i++ {
		protoData := data.Get(i)

		dstDir, err := filepath.Abs(filepath.Join(dst, protoData.Name))
		if err != nil {
			return fmt.Errorf("failed to get absolute path for pkg '%s': %w", protoData.GoPkg, err)
		}

		err = os.MkdirAll(dstDir, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory for pkg '%s' proto files: %w", protoData.GoPkg, err)
		}

		dstFile, err := filepath.Abs(filepath.Join(dstDir, path.Base(protoData.GoPkg)+".proto"))
		if err != nil {
			return fmt.Errorf("failed to get absolute path for destination file: %w", err)
		}

		fmt.Println("writing file", dstFile)

		err = withFile(dstFile, func(f *os.File) error {
			protoData.Format(f)

			return nil
		})
		if err != nil {
			return fmt.Errorf("failed to write file '%s': %w", dstFile, err)
		}
	}

	return nil
}

func printFoundStructs(structs ast.TaggedStructs) {
	for decl := range structs {
		fmt.Printf("found tagged struct '%s' in pkg '%s'\n", decl.Name, decl.Pkg)
	}
}

func withFile(filename string, fn func(f *os.File) error) error {
	dir := filepath.Dir(filename)

	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return err
	}

	defer func() {
		err := file.Close()
		if err != nil {
			fmt.Println("failed to close file:", err)
		}
	}()

	return fn(file)
}
