# Setup (English)

Run xmeye-wrapper on your own server to watch XMEye cameras from a browser.

> Note: The project is in early development. Once the cloud login/token flow is complete, setup will become a
> single step; for now the token/relay info is supplied manually.

## Requirements

- **Go 1.23+** (to run from source) **or** Docker (ready-to-run).
- A Linux, Windows or macOS server. A small VPS / Raspberry Pi is enough.
- The camera's **serial number (Cloud ID)**, **username** and **password**.

## 1. Run from source

```bash
git clone <repo>
cd xmeye-wrapper/core-go
go run ./server
```

The server starts at `http://localhost:8080`. Open it in a browser.

### Configuration

Two options: environment variables **or** `core-go/config.json`.

Environment variables:

```bash
export XMEYE_NAME=tbdc              # device name (display)
export XMEYE_SERIAL=22e4...shc6     # serial (Cloud ID)
export XMEYE_USER=koy1              # username
export XMEYE_PASS=******            # password
export XMEYE_RELAY=101.44.x.x:8088  # cloud relay (temporary; will be automatic)
export XMEYE_TOKEN=<download-token> # access token (temporary; will be automatic)
export XMEYE_SESSION=<session-hex>  # session (temporary)
export XMEYE_ADDR=:8080             # listen address
go run ./server
```

`config.json` example (inside `core-go/`):

```json
{
  "addr": ":8080",
  "device": {
    "name": "tbdc",
    "serial": "22e4...shc6",
    "username": "koy1",
    "password": "******",
    "channels": 16
  }
}
```

> Security: `config.json` holds the camera password and is in `.gitignore` — do not share it.

## 2. Run with Docker

```bash
cd xmeye-wrapper
docker compose up -d
```

(Docker files live under `docker/`; configure via environment variables in `docker-compose.yml`.)

## 3. Embed in a website (e.g. WordPress)

While the bridge runs, drop a single PHP file (`clients/php/`) into your site to embed the viewer.
Just set the bridge address (e.g. `http://server:8080`) in the small config block at the top. See
`clients/php/README` (in progress).

## Common issues

- **"token/relay not set":** `XMEYE_TOKEN` and `XMEYE_RELAY` were not provided. (Required manually until this step is automated.)
- **Segment 404:** token expired or wrong. A fresh token is needed.
- **No video:** make sure the device is online and cloud (P2P/RPS) is enabled.
