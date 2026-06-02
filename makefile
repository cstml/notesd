.PHONY: build install uninstall enable

DESTDIR ?=
PREFIX  ?= /usr/local
BINDIR   = $(DESTDIR)$(PREFIX)/bin
UNITDIR  = $(DESTDIR)/etc/systemd/system

build: bin/notesd

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo devel)
LDFLAGS  = -ldflags "-X main.version=$(VERSION)"

bin/notesd: go.mod main.go
	go build $(LDFLAGS) -o bin/notesd .

install: bin/notesd notesd.service
	install -d $(BINDIR) $(UNITDIR) $(DESTDIR)/etc/notesd
	install -m 0755 bin/notesd $(BINDIR)/notesd
	install -m 0644 notesd.service $(UNITDIR)/notesd.service
	[ -n "$(DESTDIR)" ] || systemctl daemon-reload

enable:
	systemctl enable --now notesd

uninstall:
	-systemctl disable --now notesd
	rm -f $(BINDIR)/notesd
	rm -f $(UNITDIR)/notesd.service
	[ -n "$(DESTDIR)" ] || systemctl daemon-reload
