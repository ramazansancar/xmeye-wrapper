# Node.js istemci

XMEye DVRIP protokolünün Node.js portu. Go çekirdeğiyle (`core-go`) aynı protokol; Node tabanlı
köprü ya da araçlar için. Bağımlılık yok (yerleşik `node:crypto`, `node:test`).

## Modüller

- `src/dvrip.js` — DVRIP paket codec + Sofia parola hash (Go/Python ile birebir).

## Test

```bash
cd clients/node
npm test        # node --test tests/*.test.js
```

(Testler `tests/` altında — teknoloji/dil özelinde ayrı klasör.)

## Gereksinim

Node 18+ (ESM, `node:test`). Geliştirmede Node 22 kullanıldı.
