package errors

import (
	"errors"
)

var (
	ErrSentToClosed = errors.New("sent to closed connection")
)
