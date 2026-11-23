# Setup (English)

Run xmeye-wrapper on your own server to watch XMEye cameras from a browser.

## Requirements

- **Go 1.23+** (to run from source) **or** Docker (ready-to-run).
- Linux, Windows or macOS. A small VPS / Raspberry Pi is enough.
- The camera's **serial number (Cloud ID)**, **username** and **password**.
- Browser: **Chrome / Edge 107+** or **Safari 17+** (for WebCodecs + H.265/HEVC hardware decode).

## 1. Run from source

```bash
git clone <repo>
cd xmeye-wrapper
cp .env.example .env        # fill in values (serial, username, password)
cd core-go
go run ./server
```

The server starts at `http://localhost:8080`. Open it in a browser.

### Configuration

`.env` (in the project root) or environment variables:

```bash
XMEYE_NAME=camera          # device name (shown in UI)
XMEYE_SERIAL=<serial>      # serial (Cloud ID)
XMEYE_USER=<username>      # username
XMEYE_PASS=<password>      # password
XMEYE_CHANNELS=16          # channel count
XMEYE_ADDR=:8080           # listen address (optional)
```

> Security: `.env` holds the camera password and is in `.gitignore` — do not share it.
> You do not supply relay/token manually; the bridge establishes the cloud connection (NAT traversal) itself.

## 2. Run with Docker

```bash
cd xmeye-wrapper
cp .env.example .env        # fill in values
docker compose up -d --build
```

**Important — networking:** The cloud connection uses UDP NAT traversal (P2P), which Docker's bridge
network breaks. The compose file therefore uses **host networking** (`network_mode: host`).

- **Linux server:** host networking is native → works out of the box. Recommended deployment target.
- **Windows / macOS Docker Desktop:** host networking stays inside the WSL2/VM, so the browser cannot
  reach `localhost:8080`. On these platforms run the bridge directly with `go run ./server`.

## 3. Embed in a website (e.g. WordPress)

While the bridge runs, drop a single PHP file (`clients/php/`) into your site to embed the viewer via
an `<iframe>`. Set the bridge address (e.g. `http://server:8080`) in the config block at the top.

## Common issues

- **Black screen / stuck on "connecting":** the device must be online with cloud (P2P/RPS) enabled.
  Opening many channels at once can delay initial load (NAT; each channel opens its own connection).
- **No video, old browser:** H.265/HEVC needs Chrome/Edge 107+ or Safari 17+.
- **UI not reachable in Docker (Windows):** host-networking limitation — use `go run ./server` (see section 2).
