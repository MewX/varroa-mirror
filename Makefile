GO = GO111MODULE=on go
VERSION=`git describe --tags`

all: fmt check test-coverage build

prepare:
	${GO} get -u github.com/divan/depscheck
	${GO} get github.com/warmans/golocc
	${GO} install github.com/golangci/golangci-lint/cmd/golangci-lint

deps:
	${GO} mod download

fmt:
	${GO} fmt ./...

check: fmt
	golangci-lint run

info: fmt
	depscheck -totalonly -tests .
	golocc .

test-coverage:
	${GO} test -race -coverprofile=coverage.txt -covermode=atomic ./...

clean:
	rm -f coverage.txt
	rm -f varroa
	rm -f varroa-fuse
	rm -f varroa_bash_completion
	rm -f send-to-varroa.js

build:
	${GO} build -v ./...
build-bin:
	cd cmd/varroa;${GO} build -ldflags "-X gitlab.com/passelecasque/varroa.Version=${VERSION}" -o ../../varroa;cd ../..
	cd cmd/varroa-fuse;${GO} build -ldflags "-X gitlab.com/passelecasque/varroa.Version=${VERSION}" -o ../../varroa-fuse;cd ../..
	cp cmd/varroa/bash_completion varroa_bash_completion
	cp script/send-to-varroa.js send-to-varroa.js

install:
	${GO} install -ldflags "-X main.Version=${VERSION}" -v ./...





