all:
	go build -o gordio ./cmd/gordio/

generate:
	sqlc generate -f sql/sqlc.yaml
