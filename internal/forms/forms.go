package forms

import (
	"errors"
	"time"
)

var (
	ErrNotStruct   = errors.New("destination is not a struct")
	ErrNotPointer  = errors.New("destination is not a pointer")
	ErrContentType = errors.New("bad content type")
)

const (
	MaxMultipartMemory int64 = 1024 * 1024 // 1MB
)

type options struct {
	tagOverride        *string
	parseTimeIn        *time.Location
	parseLocal         bool
	acceptBlank        bool
	maxMultipartMemory int64
	defaultOmitEmpty   bool
}

type Option func(*options)

func WithParseTimeInTZ(l *time.Location) Option {
	return func(o *options) {
		o.parseTimeIn = l
	}
}

func WithParseLocalTime() Option {
	return func(o *options) {
		o.parseLocal = true
	}
}

func WithAcceptBlank() Option {
	return func(o *options) {
		o.acceptBlank = true
	}
}

func WithTag(t string) Option {
	return func(o *options) {
		o.tagOverride = &t
	}
}

func WithMaxMultipartSize(s int64) Option {
	return func(o *options) {
		o.maxMultipartMemory = s
	}
}

func WithOmitEmpty() Option {
	return func(o *options) {
		o.defaultOmitEmpty = true
	}
}

func (o *options) Tag() string {
	if o.tagOverride != nil {
		return *o.tagOverride
	}

	return "form"
}
