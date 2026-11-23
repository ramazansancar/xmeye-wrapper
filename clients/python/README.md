# Python istemci / araçlar

XMEye DVRIP protokolünün Python portu + araçlar. Go çekirdeğiyle (`core-go`) aynı protokol,
Python tarafında protokol araştırma, pcap analizi ve alternatif köprü için.

## Modüller

- `xmeye_wrapper/dvrip.py` — DVRIP paket codec + Sofia parola hash (Go `core-go/dvrip` ile birebir).

## Test

```bash
cd clients/python
python -m pytest tests/ -q
```

(Testler `tests/` altında — teknoloji/dil özelinde ayrı klasör.)

## Durum

Erken. DVRIP codec + auth hazır ve testli. Bulut login/token akışı + köprü (Go'da referans) eklenecek.
