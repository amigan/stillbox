all:
	go build -o gordio ./cmd/gordio/
	go build -o calls ./cmd/calls/

generate:
	sqlc generate -f sql/sqlc.yaml
	protoc -I=pkg/pb/ --go_out=pkg/ pkg/pb/stillbox.proto

