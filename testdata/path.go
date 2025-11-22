// Package testdata is merely a way to get the path to the testdata directory.
package testdata

import (
	"path/filepath"
	"runtime"
)

var (
    _, b, _, _ = runtime.Caller(0)

    Path = filepath.Dir(b)
)
