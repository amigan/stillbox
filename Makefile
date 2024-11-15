VPKG=dynatron.me/x/stillbox/internal/version
VER!=git describe --tags --always --dirty
BUILDDATE!=date '+%Y%m%d'
LDFLAGS=-ldflags="-X '${VPKG}.Version=${VER}' -X '${VPKG}.Built=${BUILDDATE}'"
GOFLAGS=-v

all: checkcalls
	go build -o stillbox ${GOFLAGS} ${LDFLAGS} ./cmd/stillbox/
	go build -o calls ${GOFLAGS} ${LDFLAGS} ./cmd/calls/

buildpprof:
	go build -o stillbox-pprof ${GOFLAGS} ${LDFLAGS} -tags pprof ./cmd/stillbox

clean:
	rm -rf client/calls/ && mkdir client/calls && touch client/calls/.gitkeep
	rm -f stillbox calls stillbox-pprof

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

coverage-html:
	go tool cover -html=cover.out

coverage:
	go test -coverprofile cover.out

# backup backs up the database without calls
backup:
	sh util/dumpdb.sh

backupplain:
	sh util/dumpdb.sh -p

test:
	go test -v ./...

run:
	go run -v ./cmd/stillbox/ serve
