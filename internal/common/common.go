package common

import (
	"context"
	"reflect"
	"strconv"

	"github.com/urfave/cli/v3"
)

const (
	AppName   = "stillbox"
	EnvPrefix = "STILLBOX_"
)

const (
	TimeFormat = "Jan 2 15:04:05"
)

type cmdOptions interface {
	Options(context.Context, *cli.Command) error
	Execute() error
}

func Action(c cmdOptions) cli.ActionFunc {
	return func(ctx context.Context, cmd *cli.Command) error {
		err := c.Options(ctx, cmd)
		if err != nil {
			return err
		}

		return c.Execute()
	}
}

func PtrTo[T any](t T) *T {
	return &t
}

func NilIfZero[T comparable](val T) *T {
	var zero T
	if val == zero {
		return nil
	}

	return &val
}

// ZeroFields takes a struct or a pointer to struct and if any pointer fields point to a zero value for the pointed type, they are set to nil. It also recurses into embedded fields.
func ZeroFields(s any) {
	zeroFields(reflect.ValueOf(s))
}

func zeroFields(v reflect.Value) {
	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		panic("must be pointer to struct")
	}

	vt := v.Type()

	for fi := range v.NumField() {
		fv := v.Field(fi)
		ft := fv.Type()

		if ft.Kind() == reflect.Struct && vt.Field(fi).Anonymous {
			zeroFields(fv)
		}
		if fv.Kind() == reflect.Pointer && !fv.IsNil() && fv.Elem().Kind() == reflect.String {
			fe := fv.Elem()
			if fe.IsZero() {
				fv.Set(reflect.Zero(ft))
			}
		}
	}
}

func ZeroIfNil[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}

	return *v
}

func DefaultIfNilOrZero[T comparable](v *T, def T) T {
	if v == nil {
		return def
	}

	var zero T
	if *v == zero {
		return def
	}

	return *v
}

// AtoiU32 is atoi() that supports hex (0x) or dec.
func AtoiU32(s string) (uint32, error) {
	if len(s) > 2 && s[0] == '0' && s[1] == 'x' {
		v, err := strconv.ParseUint(s, 16, 32)
		if err != nil {
			return 0, err
		}

		return uint32(v), err
	}

	v, err := strconv.Atoi(s)
	return uint32(v), err
}
