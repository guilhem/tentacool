BINDIR := $(DESTDIR)/usr/bin/
GOPATH := ${CURDIR}/build
TENTACOOLPATH := ${GOPATH}/src/github.com/optiflows/tentacool
GOM := env GOPATH="${GOPATH}" ${GOPATH}/bin/gom
GO := env GOPATH="${GOPATH}" go


build:
	# Copy current branch additional packages into GOPATH
	@mkdir -p ${TENTACOOLPATH}
	@cp -rf addresses dns interfaces gateway dhcp ${TENTACOOLPATH}
	# Install gom
	@${GO} get github.com/mattn/gom
	# Build tentacool using gom
	@${GOM} install
	@${GOM} build

install: bindir
	install tentacool $(BINDIR)

bindir:
	mkdir -p $(BINDIR)

clean:
	rm -rf ${GOPATH}
	rm -rf _vendor
	rm -rf tentacool
