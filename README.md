# notesd

Git-backed notes (micro) service. Every write is a commit; you get history for
free. Aims to be the simplest note taking app service.

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

## Web interface

Browsers (`Accept: text/html`) get a small editor UI — create, view, and edit
notes without curl. `GET /` lists all notes with previews, `GET /$ID` opens
an editor (empty for a new id), and `GET /$ID/history` lists revisions.
See screenshots at <https://cstml.github.io/notesd/>.

## Storage

Notes are stored as plain text files in `STORAGE_PATH` (one file per note,
filename = id), and the directory is a normal git repo. You can `cat`, `grep`,
edit, or `git log` them directly on disk — no database, no binary blobs.

## Ids & Names

Ids are normalized to `[a-z0-9_-]+`: lowercased, other chars collapsed to `-`,
trimmed, capped at 128 chars.

## Config

- `PORT` (default `3333`)
- `STORAGE_PATH` (default `data`, auto-init as git repo)
- `GIT_REMOTE` (optional) — pull `--ff-only` before each request (5s timeout)
  and push asynchronously after every commit.

See `.env.example`.

## Install

```sh
make build
sudo make install   # binary + systemd unit (honors DESTDIR, PREFIX)
sudo make enable
sudo make uninstall
```

Operator config lives at `/etc/notesd/.env` (see `notesd.service`).
