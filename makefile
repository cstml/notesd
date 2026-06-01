.PHONY: build install uninstall enable

DESTDIR ?=
PREFIX  ?= /usr/local
BINDIR   = $(DESTDIR)$(PREFIX)/bin
UNITDIR  = $(DESTDIR)/etc/systemd/system

build: bin/notesd

bin/notesd: go.mod main.go
	go build -o bin/notesd .

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
