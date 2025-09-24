//go:build server

package main

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/pelicanplatform/pelican/internal/cmd"
)

var PelicanServerRootCmd = &cobra.Command{
	Use:   "pelican-server",
	Short: "Pelican Server",
}

func init() {
	PelicanServerRootCmd.AddCommand(cmd.OriginCmd)
	PelicanServerRootCmd.AddCommand(cmd.CacheCmd)
	PelicanServerRootCmd.AddCommand(cmd.DirectorCmd)
	PelicanServerRootCmd.AddCommand(cmd.RegistryCmd)
	PelicanServerRootCmd.AddCommand(cmd.DowntimeCmd)
	PelicanServerRootCmd.AddCommand(cmd.RootConfigCmd)
	PelicanServerRootCmd.AddCommand(cmd.TokenCmd)
	PelicanServerRootCmd.AddCommand(cmd.KeyCmd)
}

func main() {
	err := cmd.HandleCLI(os.Args, PelicanServerRootCmd)
	if err != nil {
		os.Exit(1)
	}
}
