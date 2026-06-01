.PHONY: build install uninstall enable

DESTDIR ?=
PREFIX  ?= /usr/local
BINDIR   = $(DESTDIR)$(PREFIX)/bin
UNITDIR  = $(DESTDIR)/etc/systemd/system

build: bin/pastebinserve

bin/pastebinserve: go.mod main.go
	go build -o bin/pastebinserve .

install: bin/pastebinserve pastebinserve.service
	install -d $(BINDIR) $(UNITDIR) $(DESTDIR)/etc/pastebinserve
	install -m 0755 bin/pastebinserve $(BINDIR)/pastebinserve
	install -m 0644 pastebinserve.service $(UNITDIR)/pastebinserve.service
	[ -n "$(DESTDIR)" ] || systemctl daemon-reload

enable:
	systemctl enable --now pastebinserve

uninstall:
	-systemctl disable --now pastebinserve
	rm -f $(BINDIR)/pastebinserve
	rm -f $(UNITDIR)/pastebinserve.service
	[ -n "$(DESTDIR)" ] || systemctl daemon-reload
