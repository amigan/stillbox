VPKG=dynatron.me/x/stillbox/internal/version
VER!=git describe --tags --always --dirty
BUILDDATE!=date '+%Y-%m-%e'
LDFLAGS=-ldflags="-X '${VPKG}.Version=${VER}' -X '${VPKG}.Built=${BUILDDATE}'"

all: checkcalls
	go build -o gordio ${GOFLAGS} ${LDFLAGS} ./cmd/gordio/
	go build -o calls ${GOFLAGS} ${LDFLAGS} ./cmd/calls/

buildpprof:
	go build -o gordio-pprof ${GOFLAGS} ${LDFLAGS} -tags pprof ./cmd/gordio

clean:
	rm -rf client/calls/ && mkdir client/calls && touch client/calls/.gitkeep
	rm -f gordio calls gordio-pprof

checkcalls:
	@test -e client/calls/index.html || make getcalls

getcalls:
	rm -rf client/calls/*
	cd client/calls/ && curl -OL https://nightly.link/amigan/calls/workflows/build-web/trunk/webBuild.zip && unzip -o webBuild.zip && rm webBuild.zip

generate:
	sqlc generate -f sql/sqlc.yaml
	protoc -I=pkg/pb/ --go_out=pkg/ pkg/pb/stillbox.proto

lint:
	golangci-lint run
