# xmeye-wrapper

XMEye / Xiongmai kameralarını **tarayıcıda, reklamsız, eklentisiz** izlemek için açık kaynak köprü.
Sadece bulut **seri no + kullanıcı/şifre** ile çalışır — yerel ağ (LAN) gerekmez.

> Köy kameralarını (İsahocalılar) reklamsız izlemek için doğdu; her Xiongmai cihaz sahibi kullanabilsin diye
> genel, çok dilli ve Docker ile kolay host edilebilir tasarlandı.

**Diller / Languages:** [Türkçe kurulum](docs/kurulum.tr.md) · [English setup](docs/setup.en.md)

---

## Neden?

- Resmî XMEye mobil uygulaması reklam dolu; bilgisayardan (tarayıcı) izleme yolu yok.
- Resmî web portalı bile tarayıcıya eklenti (`VideoPlayTool.exe`) kurduruyor.
- Bu proje: küçük bir köprü sunucu + tarayıcı arayüzü. Eklenti yok, reklam yok.

## Nasıl çalışır?

Tarayıcı ham TCP/UDP açamaz; bu yüzden küçük bir **köprü sunucu** XMEye bulutuna bağlanır, video
frame'lerini tek WebSocket üzerinden tarayıcıya iletir; tarayıcı **WebCodecs** ile donanımda çözüp
canvas'a çizer. Eklenti yok, transcode yok (düşük gecikme, düşük CPU).

```
Tarayıcı  ◄─WebSocket (H.264/H.265 NAL)─►  Köprü sunucu  ◄─bulut P2P/relay─►  Xiongmai bulut  ──►  NVR/kamera
   │
   └─ WebCodecs VideoDecoder → <canvas>
```

## Durum

**Canlı izleme çalışıyor.** Seri no + kullanıcı/şifre ile köy kameraları tarayıcıda canlı görüntüleniyor
(tekli + 4/9/16 grid, HD/SD, PTZ, snapshot — gerçek cihazda doğrulandı).

Çalışan parçalar:

- `core-go/dvrip` — DVRIP (Sofia) protokol çekirdeği: paket codec, Sofia parola hash, komutlar (testli).
- `core-go/cloud` — XMEye bulut bağlantısı: NAT traversal (P2P + proxy relay), login, OPMonitor, canlı H.264/H.265 akış.
- `core-go/server` — HTTP köprü + tek-WS çok kanal video çoğullama (`hub.go`) + **NVR-düzeni web arayüzü**
  (büyük ekran + grid + PTZ/kalite/playback kontrolcü + senkron zaman çizgisi).
- `core-go/cmd/*` — protokol analiz/tanı araçları (`pcapanalyze`, `body1412`, `xmframe`, `cloudtest`).

**Protokol durumu (detay: [docs/protokol-spec.md](docs/protokol-spec.md)):**

- **6611 DVRIP yolu tamamen çözüldü, şifresiz.** Login, video başlatma (OPMonitor), playback (OPPlayBack) düz JSON;
  video (msgID 1412/1422) düz H.264/H.265. Cihazın login yanıtı `DataUseAES:false` — video AES yok (kesin).
- Video frame'leri XM frame header'lı (`00 00 01 FX`), header strip edilerek ham H.264/H.265 NAL elde edilir.
- **Codec:** cihaz **H.265/HEVC** de yollar; tarayıcı WebCodecs ile ikisini de (H.264 + H.265) çözer. jmuxer yok.
- 8088 HTTP/`.ts` yolu alternatif (şifresiz MPEG-TS) ama tutarsız tetikleniyor; ana yol 6611.

**Bilinen sınırlar / yol haritası:**

- **Tam otomatik relay token:** şu an relay token/relay IP elle (`.env`) veriliyor. Seri no → relay token
  `8186` TLS signaling'den geliyor; tam otomasyon için bu çözülmeli.
- **Bağlantı havuzu:** her kanal cloud'a ayrı 2 bağlantı açıyor → çok kanalda NAT yoruluyor, yükleme gecikir.
  Planlanan: tek login + çok OPMonitor claim.
- **Senkron playback:** timeline UI hazır; backend OPPlayBack (6611) implementasyonu bekliyor.

## Hızlı başlangıç (geliştirici)

```bash
cd core-go
go test ./...          # testler
go run ./server        # köprü sunucu → http://localhost:8080
```

Ayarlar `.env` ya da ortam değişkenleri ile (`.env.example` kopyala):

```bash
XMEYE_NAME=tbdc XMEYE_SERIAL=<seri-no> XMEYE_USER=<kullanıcı> XMEYE_PASS=<şifre> \
XMEYE_CHANNELS=16 \
go run ./server
```

Docker ile: `docker compose up -d --build` (Linux sunucu host ağında çalışır — UDP NAT için şart;
bkz. `docker-compose.yml`). **Windows Docker Desktop'ta** host ağı WSL2 VM'de kalır, tarayıcı
localhost'tan erişemez → Windows'ta `go run ./server` kullanın.

Ayrıntılı kurulum: [docs/kurulum.tr.md](docs/kurulum.tr.md).

## Yapı

```
core-go/     Go köprü sunucu + protokol çekirdeği (referans implementasyon)
  dvrip/     DVRIP protokol (codec, auth, komutlar)
  cloud/     bulut NAT traversal (P2P + proxy relay) + canlı akış
  server/    HTTP köprü + tek-WS çoğullama (hub.go) + web arayüz (web.go)
  cmd/       protokol analiz/tanı araçları
clients/
  php/       köy sitesi için tek-dosya PHP gömme (WordPress/Linux)
  python/    alternatif istemci + araçlar
  node/      alternatif istemci
docker/      Docker + docker-compose (kolay host)
docs/        kurulum (TR + EN) + protokol spec
```

## Güvenlik notu

Xiongmai/XMEye cihazlarının bulut ve yerel protokollerinde bilinen güvenlik açıkları var. Kamera şifresini
ve portunu halka açmak yerine, yayını bu köprü üzerinden salt-okunur dağıtmak daha güvenli.

## Lisans

**[PolyForm Noncommercial License 1.0.0](LICENSE)** — ticari olmayan kullanım serbesttir.

- Köy, dernek, kurum, kişisel ve diğer **ticari olmayan** amaçlarla serbestçe kullanılabilir, değiştirilebilir, dağıtılabilir.
- **Ticari kullanım yasaktır.** Ticari kullanım için ayrı izin/iletişim gerekir.
- Ayrıntı: [LICENSE](LICENSE) dosyası ve <https://polyformproject.org/licenses/noncommercial/1.0.0>.

### Üçüncü taraf atıfları

Cloud/NAT protokol implementasyonu, aşağıdaki MIT lisanslı çalışmadan uyarlanmıştır:

- `irlanfazryana23/cctv-backup` (MIT) — XMEye cloud NAT traversal + DVRIP istemci referansı.
