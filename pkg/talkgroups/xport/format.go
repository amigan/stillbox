package xport

import (
	"errors"
)

type Format string

const (
	FormatRadioReference Format = "radioreference"
	FormatSDRTrunk       Format = "sdrtrunk"
)

var (
	ErrBadType = errors.New("unknown format type")
)
