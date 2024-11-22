package common

import (
	"github.com/spf13/cobra"
)

const AppName = "stillbox"

const (
	TimeFormat = "Jan 2 15:04:05"
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

func PtrTo[T any](t T) *T {
	return &t
}

func PtrOrNull[T comparable](val T) *T {
	var zero T
	if val == zero {
		return nil
	}

	return &val
}

func ZeroOr[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}

	return *v
}
