# Pastebinserve

Git-backed pastebin. Every write is a commit; you get history for free.

## API

```sh
URL=http://localhost:3333

curl -X POST   -d "..." "$URL"               # auto id (<unix>-<rand>)
curl -X POST   -d "..." "$URL/$ID"           # create or overwrite
curl    "$URL/$ID"                           # latest content
curl    "$URL/$ID?version=<sha>"             # content at commit (prefix ok)
curl    "$URL/$ID/history"                   # <sha> <unix-time> <preview>
curl -X DELETE "$URL/$ID"                    # delete (history preserved)
curl    "$URL"                               # list: <id>\t<preview>
```

`PUT` is an alias for `POST`. Writes return the commit SHA in `X-Commit`.
Send `Accept: application/json` on `GET /` for JSON output.

Ids are normalized to `[a-z0-9_-]+`: lowercased, other chars collapsed to `-`,
trimmed, capped at 128 chars.

## Config

`PORT` (default `3333`), `STORAGE_PATH` (default `data`, auto-init as git repo).
See `.env.example`.

## Install

```sh
make build
sudo make install   # binary + systemd unit (honors DESTDIR, PREFIX)
sudo make enable
sudo make uninstall
```
