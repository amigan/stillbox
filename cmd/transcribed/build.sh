#!/bin/sh

VPKG=dynatron.me/x/stillbox/internal/version
VER=$1
BUILDDATE=$2

VERSION_LDF="-X '${VPKG}.Version=${VER}' -X '${VPKG}.Built=${BUILDDATE}'"
WHISPER_PACKAGES=whisper
if [ "${GGML_CUDA}" = 1 ] ; then
	WHISPER_PACKAGES="${WHISPER_PACKAGES} cudart cuda cublas"
fi

set -e

export PKG_CONFIG_PATH=/usr/local/lib/pkgconfig
WHISPER_LDFLAGS=`pkg-config --libs ${WHISPER_PACKAGES}`
export CGO_LDFLAGS=${WHISPER_LDFLAGS}
export CGO_CFLAGS=`pkg-config --cflags ${WHISPER_PACKAGES}`
TRANSCRIBED_LDFLAGS=-ldflags="${VERSION_LDF}"

echo building transcribed with "${TRANSCRIBED_LDFLAGS}"
go build -v -o transcribed -tags whisper ${GOFLAGS} "${TRANSCRIBED_LDFLAGS}" ./cmd/transcribed/
