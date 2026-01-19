VPKG=dynatron.me/x/stillbox/internal/version
VER!=git describe --tags --always --dirty
BUILDDATE!=date '+%Y%m%d'
LDFLAGS=-ldflags="-X '${VPKG}.Version=${VER}' -X '${VPKG}.Built=${BUILDDATE}'"
GOFLAGS=-v

all: client/stillbox/dist
	go build -o stillbox ${GOFLAGS} ${LDFLAGS} ./cmd/stillbox/
	go build -o calls ${GOFLAGS} ${LDFLAGS} ./cmd/calls/

pprof:
	go build -o stillbox-pprof ${GOFLAGS} ${LDFLAGS} -tags pprof ./cmd/stillbox

client/stillbox/dist:
	cd client/stillbox && npm install && ng build -c production

web: web-install web-build

web-build:
	cd client/stillbox && ng build -c production

web-install:
	cd client/stillbox && npm install

clean:
	rm -rf client/calls/ client/stillbox/dist/ client/stillbox/node_modules/
	rm -f stillbox calls stillbox-pprof

generate:
	sqlc generate -f sql/sqlc.yaml
	protoc -I=pkg/pb/ --go_out=pkg/ pkg/pb/stillbox.proto
	go generate ./...
	go run ./util/omitempty/omitempty.go

lint:
	golangci-lint run --build-tags integration

coverage-html:
	go tool cover -html=cover.out

coverage-html-file:
	go tool cover -html=cover.out -o coverage.html

coverage:
	go test -tags integration,noclient -coverprofile cover.out ./...

# backup backs up the database without calls
backup:
	sh util/dumpdb.sh

backupplain:
	sh util/dumpdb.sh -p

test:
	go test -race ${GOFLAGS} ${LDFLAGS} -race -v ./...

citest:
	go test -tags integration,noclient -coverprofile cover.out ${GOFLAGS} ${LDFLAGS} ./internal/... ./pkg/... ./util/...

run:
	go run -v ./cmd/stillbox/ serve

transcribe:
	@sh cmd/transcribed/build.sh ${VER} ${BUILDDATE}
