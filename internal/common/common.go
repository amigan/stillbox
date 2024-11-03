package common

import (
	"fmt"
	"strconv"

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

func FmtFloat(v float64, places ...int) string {
	if len(places) > 0 {
		return fmt.Sprintf("%."+strconv.Itoa(places[0])+"f", v)
	}
	return fmt.Sprintf("%.4f", v)
}
