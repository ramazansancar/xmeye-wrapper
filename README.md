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

Tarayıcı ham TCP/UDP açamaz; bu yüzden küçük bir **köprü sunucu** XMEye bulutuna bağlanır, videoyu
tarayıcı dostu formata (HLS / ileride WebRTC, MSE) çevirir.

```
Tarayıcı  ──HLS──►  Köprü sunucu  ──bulut relay──►  Xiongmai bulut  ──►  NVR/kamera
```

## Durum

Erken geliştirme. Çalışan parçalar:

- `core-go/dvrip` — DVRIP (Sofia) protokol çekirdeği: paket codec, Sofia parola hash, komutlar (testli).
- `core-go/transport` — HLS/MSE proxy (relay segmentlerini tarayıcıya sunar).
- `core-go/server` — HTTP köprü sunucu + **NVR-düzeni web arayüzü** (büyük ekran + PTZ/kalite/playback kontrolcü + galeri şeridi + düzen grid).
- `core-go/cmd/pcapanalyze`, `cmd/body1412`, `cmd/xmframe` — protokol analiz araçları.

**Protokol durumu (detay: [docs/protokol-spec.md](docs/protokol-spec.md)):**

- **6611 DVRIP yolu tamamen çözüldü, şifresiz.** Login, video başlatma (OPMonitor), playback (OPPlayBack) düz JSON;
  video (msgID 1412/1422) düz H.264. Cihazın login yanıtı `DataUseAES:false` — video AES yok (kesin).
- Video frame'leri XM frame header'lı (`00 00 01 FC/FD/FA`), header strip edilerek H.264 elde edilir.
- 8088 HTTP/`.ts` yolu alternatif (şifresiz MPEG-TS) ama tutarsız tetikleniyor; ana yol 6611.
- **Açık tek iş:** RPS relay token (seri no → relay) `8186` portunda TLS şifreli signaling'den geliyor — otomasyonun
  son parçası. Ayrıca H.264 decode için SPS/PPS çözümü. Bunlar çözülünce köprü seri+kullanıcı+şifre ile tam otomatik akıtır.

## Hızlı başlangıç (geliştirici)

```bash
cd core-go
go test ./...          # testler
go run ./server        # köprü sunucu → http://localhost:8080
```

Ayarlar ortam değişkenleri ile (ya da `config.json`):

```bash
XMEYE_NAME=tbdc XMEYE_SERIAL=<seri-no> XMEYE_USER=<kullanıcı> XMEYE_PASS=<şifre> \
XMEYE_RELAY=<host:port> XMEYE_TOKEN=<token> XMEYE_SESSION=<hex> \
go run ./server
```

Ayrıntılı kurulum: [docs/kurulum.tr.md](docs/kurulum.tr.md).

## Yapı

```
core-go/     Go köprü sunucu + protokol çekirdeği (referans implementasyon)
  dvrip/     DVRIP protokol (codec, auth, komutlar)
  transport/ tarayıcı transport (HLS; ileride WebRTC, MSE)
  server/    HTTP köprü + web arayüz
clients/
  php/       köy sitesi için tek-dosya PHP gömme (WordPress/Linux)
  python/    alternatif istemci + araçlar
  node/      alternatif istemci
docker/      Docker + docker-compose (kolay host)
docs/        kurulum (TR + EN)
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

Cloud/NAT protokol implementasyonu, aşağıdaki MIT lisanslı çalışmadan uyarlanmıştır (çevrim sırasında eklenecek):

- `irlanfazryana23/cctv-backup` (MIT) — XMEye cloud NAT traversal + DVRIP istemci referansı.
