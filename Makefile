current_dir = $(shell pwd)

GOPATH := ${current_dir}/go

bindir := $(DESTDIR)/usr/bin/
gopm := $(GOPATH)/bin/gopm

export GOPATH


build:
	go get -u github.com/gpmgo/gopm
	${gopm} get
	${gopm} build || ${gopm} build

install: bindir
	install tentacool $(DESTDIR)/usr/bin/

bindir:
	mkdir -p $(bindir)

clean:
	rm -rf .vendor
	rm -rf ${GOPATH}
	rm -rf tentacool
