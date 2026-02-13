package common

import (
	"context"
	cryptrand "crypto/rand"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
	"runtime"
	"strconv"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
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

// Keys returns an unsorted slice of the keys in m
func Keys[K comparable, V any](m map[K]V) []K {
	res := make([]K, 0, len(m))

	for k := range m {
		res = append(res, k)
	}

	return res
}

// AtoiU32 is atoi() that supports hex (0x) or dec.
func AtoiU32(s string) (uint32, error) {
	if len(s) > 2 && s[0] == '0' && s[1] == 'x' {
		v, err := strconv.ParseInt(s[2:], 16, 32)
		if err != nil {
			return 0, err
		}

		return uint32(v), err
	}

	v, err := strconv.Atoi(s)
	return uint32(v), err
}

func PGUUID(u *uuid.UUID) pgtype.UUID {
	if u != nil {
		return pgtype.UUID{
			Bytes: *u,
			Valid: true,
		}
	}

	return pgtype.UUID{}
}

// FromPanicValue is used to recover errgroup panics.
func FromPanicValue(i any) error {
	switch value := i.(type) {
	case nil:
		return nil
	case string:
		return fmt.Errorf("panic: %v\n%s", value, CollectStack())
	case error:
		return fmt.Errorf("panic in errgroup goroutine %w\n%s", value, CollectStack())
	default:
		return fmt.Errorf("unknown panic: %+v\n%s", value, CollectStack())
	}
}

func CollectStack() []byte {
	buf := make([]byte, 64<<10)
	buf = buf[:runtime.Stack(buf, false)]
	return buf
}

var letters = []rune("abcdefghijklmnopqrstuvwxyz0123456789")

func RandSeq(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

func CryptRandSeq(n int) string {
	result := ""
	for {
		if len(result) >= n {
			return result
		}
		num, err := cryptrand.Int(cryptrand.Reader, big.NewInt(int64(127)))
		if err != nil {
			panic(err)
		}
		n := num.Int64()
		if n > 32 && n < 127 {
			result += string(rune(n))
		}
	}
}
