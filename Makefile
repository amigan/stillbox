all: checkcalls
	go build -o gordio ./cmd/gordio/
	go build -o calls ./cmd/calls/

clean:
	rm -rf client/calls/ && mkdir client/calls && touch client/calls/.gitkeep
	rm -f gordio calls

checkcalls:
	test ! -e client/calls/index.html && make getcalls

getcalls:
	rm -rf client/calls/*
	cd client/calls/ && curl -OL https://nightly.link/amigan/calls/workflows/build-web/trunk/webBuild.zip && unzip -o webBuild.zip && rm webBuild.zip

generate:
	sqlc generate -f sql/sqlc.yaml
	protoc -I=pkg/pb/ --go_out=pkg/ pkg/pb/stillbox.proto

