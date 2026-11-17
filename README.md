# Trove

A self-hosted media library browser for sharing downloads from a NAS. Go serves
the API and embeds the React/TypeScript interface in a single binary. SQLite
stores local users, sessions, library indexes, settings, and activity.

Supports movies, TV, anime, books, and audiobooks; local users with per-library
access and download permissions; resumable file downloads; and streamed ZIPs for
whole titles or seasons. Media is indexed in place and is never modified.

## Run with Docker Compose

Images support `linux/amd64` and `linux/arm64`:
`ghcr.io/ncecere/trove:latest`.

If the package is private, authenticate to GHCR with a GitHub token that has
`read:packages` access before pulling. For a local build, use
`docker compose up -d --build` instead of the pull/start commands below.

```sh
git clone git@github.com:ncecere/trove.git
cd trove
export TROVE_MEDIA_PATH=/absolute/path/to/your/media
docker compose pull
docker compose up -d
```

Open **http://localhost:8080**, create your administrator account, and add
libraries under **Admin → Libraries** using paths inside the container, such as
`/media/Movies`, `/media/TV Shows`, or `/media/Books`. On a NAS, replace `localhost`
with the NAS hostname or IP. New installations have no default credentials or
demo files. Complete initial setup before making the instance available to others.

The named `trove-data` volume persists the database and cached artwork at `/data`.
The host media folder is mounted read-only at `/media`. The process runs as
UID/GID `10001:10001`; ensure this user or a supplemental group can read the NAS
folders. Set `group_add` in Compose if your NAS grants media access through a
specific group. If you replace the named data volume with a host directory,
make that directory writable by UID 10001.

The container has a health check on port 8080. Keep `TROVE_ADDR=:8080` when using
the supplied health check; change the host-side port mapping if needed. Add
HTTPS with your reverse proxy when serving it outside your local network, and
have the proxy set `X-Forwarded-Proto: https` so session cookies are marked Secure.

To mount multiple NAS shares, add additional read-only bind mounts and select
their container paths when adding libraries.

```sh
docker compose logs -f trove
docker compose down
```

`down` keeps the data volume. Back up `/data` with Trove stopped to capture a
consistent database and artwork cache; preserve the media mount paths when restoring.

## Metadata

- Movies, TV, and anime use TMDB when an administrator supplies an API key or
  read-access token in **Admin → Settings**.
- Books and audiobooks use Open Library by default. This sends title/author
  queries to Open Library; disable it in Settings to use local metadata only.
- Local artwork and `.nfo`/`.opf` metadata are also supported.
- TheTVDB is not integrated yet. TVDB IDs in local metadata can already help
  TMDB identify shows.

## Build and develop

Requires Go 1.26+ and Node.js 22.12+.

```sh
npm --prefix web ci
npm --prefix web run build
go test ./...
go build -o bin/trove ./cmd/trove
TROVE_DATA=./data ./bin/trove
```

Build the frontend before the Go binary so the embedded UI is current.
For development, run `go run ./cmd/trove` and `npm --prefix web run dev` in
separate terminals; Vite proxies API requests to port 8080.

```sh
docker build -t trove:local .
python3 scripts/container-smoke.py trove:local
```

GitHub Actions runs frontend checks, Go tests, and a container smoke test before
publishing. The smoke test covers setup, login, scanning, downloads, and data
persistence after recreating the container.
Pushes to `main` publish `latest` and commit-specific `sha-…` tags to GHCR;
version tags such as `v0.1.0` also publish version tags. Pull requests build
without publishing. Images contain no local database, media, or credentials.

See [docs/API.md](docs/API.md) for the API contract. This is an early demo;
real NAS libraries and large archive downloads still need deployment testing.
