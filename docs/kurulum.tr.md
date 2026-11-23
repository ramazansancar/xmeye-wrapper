# Kurulum (Türkçe)

xmeye-wrapper'ı kendi sunucunda çalıştırıp XMEye kameralarını tarayıcıdan izlemek için.

## Gereksinimler

- **Go 1.23+** (kaynaktan çalıştırmak için) **veya** Docker (hazır çalıştırma).
- Linux, Windows ya da macOS. Küçük bir VPS / Raspberry Pi yeter.
- Kamera **seri numarası (Cloud ID)**, **kullanıcı adı** ve **şifresi**.
- Tarayıcı: **Chrome / Edge 107+** veya **Safari 17+** (WebCodecs + H.265/HEVC donanım çözümü için).

## 1. Kaynaktan çalıştırma

```bash
git clone <repo>
cd xmeye-wrapper
cp .env.example .env        # değerleri doldur (seri no, kullanıcı, şifre)
cd core-go
go run ./server
```

Sunucu `http://localhost:8080` adresinde açılır. Tarayıcıdan gir.

### Ayarlar

`.env` (proje kökünde) ya da ortam değişkenleri:

```bash
XMEYE_NAME=camera          # cihaz adı (arayüzde görünür)
XMEYE_SERIAL=<seri-no>     # seri no (Cloud ID)
XMEYE_USER=<kullanıcı>     # kullanıcı adı
XMEYE_PASS=<şifre>         # şifre
XMEYE_CHANNELS=16          # kanal sayısı
XMEYE_ADDR=:8080           # dinleme adresi (opsiyonel)
```

> Güvenlik: `.env` kamera şifresi içerir, `.gitignore`'dadır — paylaşma.
> Relay/token elle verilmez; köprü bulut bağlantısını (NAT traversal) kendisi kurar.

## 2. Docker ile çalıştırma

```bash
cd xmeye-wrapper
cp .env.example .env        # değerleri doldur
docker compose up -d --build
```

**Önemli — ağ:** Bulut bağlantısı UDP NAT-traversal (P2P) kullanır; Docker bridge ağı bunu bozar.
Bu yüzden compose **host ağı** (`network_mode: host`) kullanır.

- **Linux sunucu:** host ağı yerel → sorunsuz çalışır. Önerilen dağıtım hedefi.
- **Windows / macOS Docker Desktop:** host ağı WSL2/VM içinde kalır, tarayıcı `localhost:8080`'e
  erişemez. Bu platformlarda köprüyü doğrudan `go run ./server` ile çalıştır.

## 3. Köy / WordPress sitesine gömme

Köprü çalışırken, sitene tek bir PHP dosyası (`clients/php/`) koyarak izleyiciyi `<iframe>` ile
gömebilirsin. Config alanına köprü adresini (örn `http://sunucu:8080`) yaz.

## Sık sorunlar

- **Görüntü siyah / "bağlanıyor" takılı:** cihaz internete bağlı ve bulut (P2P/RPS) açık olmalı.
  Çok kanal aynı anda açılınca ilk yükleme gecikebilir (NAT; her kanal ayrı bağlantı açar).
- **Görüntü gelmiyor, tarayıcı eski:** H.265/HEVC için Chrome/Edge 107+ veya Safari 17+ gerekir.
- **Docker'da UI açılmıyor (Windows):** host ağı sınırı — `go run ./server` kullan (bkz. bölüm 2).
