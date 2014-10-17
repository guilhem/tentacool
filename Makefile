current_dir = $(shell pwd)

GOPATH := ${current_dir}/go

bindir := $(DESTDIR)/usr/bin/
gopm := $(GOPATH)/bin/gopm

export GOPATH


build:
	go get -d -v ./...
	go build

install: bindir
	install tentacool $(DESTDIR)/usr/bin/

bindir:
	mkdir -p $(bindir)

clean:
	rm -rf ${GOPATH}
	rm -rf tentacool
