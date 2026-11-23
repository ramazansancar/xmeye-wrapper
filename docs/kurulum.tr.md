# Kurulum (Türkçe)

xmeye-wrapper'ı kendi sunucunda çalıştırıp XMEye kameralarını tarayıcıdan izlemek için.

> Not: Proje erken geliştirme aşamasında. Bulut login/token akışı tamamlanınca kurulum tek adıma inecek;
> şu an token/relay bilgisi elle veriliyor.

## Gereksinimler

- **Go 1.23+** (kaynaktan çalıştırmak için) **veya** Docker (hazır çalıştırma).
- Linux, Windows ya da macOS sunucu. Küçük bir VPS / Raspberry Pi yeter.
- Kamera **seri numarası (Cloud ID)**, **kullanıcı adı** ve **şifresi**.

## 1. Kaynaktan çalıştırma

```bash
git clone <repo>
cd xmeye-wrapper/core-go
go run ./server
```

Sunucu `http://localhost:8080` adresinde açılır. Tarayıcıdan gir.

### Ayarlar

İki yol: ortam değişkenleri **veya** `core-go/config.json`.

Ortam değişkenleri:

```bash
export XMEYE_NAME=tbdc              # cihaz adı (görünen)
export XMEYE_SERIAL=22e4...shc6     # seri no (Cloud ID)
export XMEYE_USER=koy1              # kullanıcı adı
export XMEYE_PASS=******            # şifre
export XMEYE_RELAY=101.44.x.x:8088  # bulut relay (geçici; ileride otomatik)
export XMEYE_TOKEN=<download-token> # erişim token'ı (geçici; ileride otomatik)
export XMEYE_SESSION=<oturum-hex>   # oturum (geçici)
export XMEYE_ADDR=:8080             # dinleme adresi
go run ./server
```

`config.json` örneği (`core-go/` içinde):

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

> Güvenlik: `config.json` kamera şifresi içerir, `.gitignore`'dadır — paylaşma.

## 2. Docker ile çalıştırma

```bash
cd xmeye-wrapper
docker compose up -d
```

(Docker dosyaları `docker/` altında; ayarlar `docker-compose.yml` içindeki ortam değişkenleriyle.)

## 3. Köy / WordPress sitesine gömme

Köprü çalışırken, sitene tek bir PHP dosyası (`clients/php/`) koyarak izleyiciyi gömebilirsin.
Üstteki küçük config alanına köprü adresini (örn `http://sunucu:8080`) yazman yeterli. Ayrıntı:
`clients/php/README` (hazırlanıyor).

## Sık sorunlar

- **"token/relay ayarlı değil":** `XMEYE_TOKEN` ve `XMEYE_RELAY` verilmemiş. (Bu adım otomatikleşene kadar elle gerekli.)
- **Segment 404:** token süresi dolmuş ya da yanlış. Taze token gerekir.
- **Video gelmiyor:** cihazın internete bağlı ve bulut (P2P/RPS) açık olduğundan emin ol.
