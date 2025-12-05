.PHONY: install

install: bin/pastebinserve pastebinserve.service
	cp ./bin/pastebinserve /usr/local/bin/
	mkdir -p /etc/pastebinserve
	cp pastebinserve.service /etc/systemd/system/
	systemctl daemon-reload

bin/pastebinserve: go.mod main.go
	go build -o bin/pastebinserve .
