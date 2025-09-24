//go:build client

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/pelicanplatform/pelican/internal/cmd"
)

var PelicanClientRootCmd = &cobra.Command{
	Use:   "pelican",
	Short: "Pelican Client",
}

func init() {
	PelicanClientRootCmd.AddCommand(cmd.ObjectCmd)
	PelicanClientRootCmd.AddCommand(cmd.TokenCmd)
	PelicanClientRootCmd.AddCommand(cmd.RootConfigCmd)
	PelicanClientRootCmd.AddCommand(cmd.KeyCmd)
}

func main() {
	err := cmd.HandleCLI(os.Args, PelicanClientRootCmd)
	if err != nil {
		os.Exit(1)
	}
}
