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
- `core-go/transport` — HLS proxy (relay `.ts` segmentlerini tarayıcıya HLS olarak sunar).
- `core-go/server` — HTTP köprü sunucu + çok kanal web arayüzü (hls.js).
- `core-go/cmd/pcapanalyze` — protokol analiz aracı (geliştirme).

Protokol: cihaz video'yu iki yoldan sunar — HTTP/`.ts` (HLS) ve ham DVRIP. Köprü HTTP/`.ts` yolunu
hedefler (en basit). Bulut login/token akışı devam ediyor.

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

Belirlenecek (açık kaynak, ticari olmayan düşünülüyor).
