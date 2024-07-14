package common

import (
	"github.com/spf13/cobra"
)

type cmdOptions interface {
	Options(*cobra.Command, []string) error
	Execute() error
}

func RunE(c cmdOptions) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := c.Options(cmd, args)
		if err != nil {
			cmd.SilenceUsage = true
			return err
		}

		err = c.Execute()
		if err != nil {
			cmd.SilenceUsage = true
		}

		return err
	}
}
