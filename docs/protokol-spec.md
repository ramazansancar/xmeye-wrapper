# XMEye / Xiongmai protokol spec (çözülen kısım)

Bu belge, tersine mühendislikle çözülen XMEye bulut protokolünü **detaylı** özetler. Go/Python/Node
implementasyonları buna dayanır. Kaynak: gerçek cihaz trafiği (pcap), APK decompile (Ghidra), Frida.

> **Durum (2026-11): 6611 DVRIP protokolü TAMAMEN çözüldü — şifresiz.** Login, video başlatma, playback,
> video frame hepsi düz JSON + düz H.264. Tek eksik: RPS relay token (8186 TLS signaling'den). Aşağıda detay.
>
> Bir şey değişir/bozulursa: önce bu belgedeki "doğrulama noktaları"nı (örnek JSON/hex) gerçek pcap ile karşılaştır.
> Analiz araçları: `core-go/cmd/pcapanalyze`, `cmd/body1412`, `cmd/xmframe`.

---

## 1. Genel bakış

- Cihaz: Xiongmai NVR/DVR/IPC (örnek: **AHB80N16T-LME-V3**, 16 kanal, firmware `V4.03.R11...`).
- **Ağ modu: RPS** (Xiongmai bulut relay) — cihaz NAT arkası, LAN erişimi yok. İstemci buluta bağlanır,
  bulut cihaza relay yapar.
- Kimlik: cihaz **seri no (Cloud ID)** + kullanıcı adı + parola. Örnek seri: `22e40425f2fc5906shc6`.
- İstemci→relay bağlantısı **TCP 6611** (ana video + kontrol) ya da **TCP 8088** (HTTP/.ts, tutarsız).

## 2. RPS relay bağlantı akışı (6611) — ADIM ADIM

İstemci bir relay sunucusuna (`101.44.0.0/16` bloğu, örn `101.44.43.34:6611`) TCP açar. Sonra:

1. **Relay karşılama (relay → istemci, ilk paket, ~80 byte):**

   ```
   {"ErrorNum":"200","DestIP":"31.223.6.184","MTU":"1400"}XXEE
   ```

   (base64 ile sarılı gelir; decode sonrası bu JSON). `DestIP` = cihazın WAN IP'si. `XXEE` = paket sonu marker.
   `ErrorNum:200` = relay cihaza ulaştı, kanal hazır.

2. **Relay auth (istemci → relay, ilk giden, base64, ~188 byte) — FORMAT:**
   decode → `<prefixId>:<relayToken>:<seriNo>:<32char-nonce>:xmeye:<appSecret-32hex>]q`
   (gerçek token/secret değerleri gizli — burada yazılmaz; yapı aşağıda.)
   - **Alanların sabitliği (6 bağlantı karşılaştırması, sig.pcap):**
     - `prefixId` (`aaaaaaaa-<rakamlar>`): **SABİT** (oturum/istemci).
     - **`relayToken`** (`c_` önekli): **SABİT tüm bağlantılarda.** 8186 TLS signaling'den gelir (§7). Bir kez alınca tekrar kullanılır.
     - `seriNo`: cihaz seri no.
     - `<32char-nonce>`: **her bağlantıda FARKLI** (rastgele, istemci üretir — biz de üretebiliriz).
     - `app` = `"xmeye"` sabit.
     - `appSecret` (32 hex): **SABİT** (muhtemelen appId/secret; APK decompile'da aranabilir).
   - **SONUÇ:** relay auth'un tek değişkeni relayToken (signaling'den) + nonce (rastgele). relayToken bir kez alınırsa
     (yakala ya da 8186 çöz) → gerisi Go'da üretilir. relayToken uzun ömürlüyse **yakala-kullan yarı-otomatik ÇALIŞIR**.

3. **LOGIN (istemci → relay, msgID=1000):** body base64 (imzalı login paketi — Sofia login JSON içerir).

4. **LOGIN YANIT (relay → istemci, msgID=1001) — DÜZ JSON:**

   ```json
   {"AliveInterval":21,"ChannelNum":16,"DataUseAES":false,"DeviceType":"HVR",
    "ExtraChannel":1,"Ret":100,"SessionID":"0x000001E7"}
   ```

   - **`DataUseAES:false`** → **VİDEO AES YOK** (cihazın kendi yanıtı; kesin kanıt).
   - `SessionID` = bundan sonraki tüm paketlerin DVRIP başlığındaki sessionID (burada `0x1E7`).
   - `ChannelNum` = kanal sayısı (16), `AliveInterval` = keepalive periyodu (sn).

5. **Video/playback başlatma** (§4) → cihaz 1412/1422 paketleriyle video akıtır.

> **Not:** Her 6611 akışının başında (2) relay karşılaması + (adım) auth tekrarlanır; çoklu kanal = çoklu 6611 bağlantı
> (yük dengeleme için farklı relay IP'lere). sessionID hepsinde aynı (login'den).

## 3. DVRIP (Sofia) paket biçimi

TCP, little-endian. 20 byte başlık + gövde:

| offset | boyut | alan        | açıklama                               |
|-------:|------:|-------------|----------------------------------------|
| 0      | 1     | head        | `0xFF` (sabit)                         |
| 1      | 1     | version     | `0x01`                                 |
| 2      | 2     | reserved    | 0                                      |
| 4      | 4     | sessionID   | login (1001) yanıtında atanır          |
| 8      | 4     | sequence    | artan                                  |
| 12     | 1     | totalPacket | çok parçalı mesajda toplam             |
| 13     | 1     | curPacket   | çok parçalı mesajda sıra               |
| 14     | 2     | msgID       | mesaj türü (LE)                        |
| 16     | 4     | dataLen     | gövde uzunluğu (LE)                    |
| 20     | …     | body        | JSON (metin) ya da ikili; dataLen kadar|

JSON gövdeler genelde `\n` + `\x00` ile sonlanır. **Doğrulama:** `FF 01 00 00 | <sess LE> | ...` ile başlar.

### Mesaj ID'leri (pcap'te doğrulananlar + standart Sofia)

| msgID | ad                 | yön      | gövde            | not                                        |
|------:|--------------------|----------|------------------|--------------------------------------------|
| 1000  | LOGIN_REQ2         | giden    | base64 (imzalı)  | login isteği                               |
| 1001  | LOGIN_RSP          | gelen    | **düz JSON**     | sessionID + DataUseAES + ChannelNum döner  |
| 1006  | KEEPALIVE_REQ      | giden    | base64           | periyodik (AliveInterval)                  |
| 1007  | KEEPALIVE_RSP      | gelen    | base64           |                                            |
| 1020  | (config/alt oturum)| giden    | base64           | login sonrası                              |
| 1360  | OPString (SupportExtRecord vb.) | giden | JSON | yetenek sorgu                           |
| 1362  | ChannelSystemFunction | giden | JSON           | kanal yetenekleri                          |
| 1410  | OPMonitor (başlat/dur) | giden | JSON            |                                            |
| 1412  | OPMonitor veri     | gelen    | **düz H.264**    | **canlı video frame** (§5)                 |
| 1413  | OPMonitor Claim    | giden    | **düz JSON**     | akış talebi (§4a)                          |
| 1414  | OPMonitor yanıt    | gelen    | düz JSON         | `{"Name":"OPMonitor","Ret":100,...}`       |
| 1422  | OPPlayBack veri    | gelen    | **düz H.264**    | **playback video frame**                   |
| 1424  | OPPlayBack Claim   | giden    | **düz JSON**     | geçmiş oynatma talebi (§4b)                |
| 1425  | OPPlayBack yanıt   | gelen    | düz JSON         | `{"Name":"OPPlayBack","Ret":100,...}`      |
| 1042  | OPSystemInfo vb.   | giden    | JSON             | CameraVersionInfo                          |
| 1452  | SYSTEM_TIME        | giden    | JSON             |                                            |

## 4. Video/playback başlatma JSON'ları (pcap'ten birebir)

### 4a. Canlı — OPMonitor Claim (msgID=1413)

```json
{"Name":"OPMonitor",
 "OPMonitor":{"Action":"Claim","Action1":"Start",
   "Parameter":{"Channel":16,"CombinMode":"NONE","StreamType":"Main","TransMode":"TCP"}},
 "SessionID":"0x00000001e7"}
```

- `Channel`: 0-tabanlı kanal (CAM03 → Channel 2). `StreamType`: **"Main" = HD, "Extra1" = SD**.
- `CombinMode`: tek kanal "NONE"; tüm kanallar tek akışta "CONNECT_ALL".
- `TransMode`: "TCP". Bazı varyantlarda `"DHParameter":{"RandomStrA":"1JJu"}` (DH anahtar değişimi alanı; AES kapalı olunca kullanılmıyor).

### 4b. Geçmiş — OPPlayBack Claim (msgID=1424)

```json
{"Name":"OPPlayBack",
 "OPPlayBack":{"Action":"Claim",
   "Parameter":{"PlayMode":"ByTime",
     "FileName":"/idea0/2015-10-20/001/00.00.00-00.00.09[H][@bff][0].h264",
     "Channel":2,"StreamType":0,"Value":0,"TransMode":"TCP"},
   "StartTime":"2026-11-23 01:32:26","EndTime":"2026-11-23 23:59:59"},
 "SessionID":"0x00000001e7"}
```

- `PlayMode":"ByTime` + StartTime/EndTime → zaman aralığı oynatma (timeline senkron için bu).
- `FileName` XM iç yolu; ByTime'da zaman belirleyici.

## 5. Video frame biçimi (1412 canlı / 1422 playback) — ŞİFRESİZ H.264

1412/1422 body'si ham H.264 **değil**, XM/Sofia video frame header'lı. Ama **AES YOK** (düz).

### XM frame header (body başında)

```
00 00 01 FX   = frame marker (4 byte)
```

| marker | tip      | header boyutu | not                          |
|--------|----------|---------------|------------------------------|
| `FC`   | I-frame  | **16 byte**   | video key (IDR)              |
| `FD`   | P-frame  | 8 byte        | video delta                  |
| `FA`   | devam/B  | 8 byte        | frame devamı                 |
| `FE`   | mux/cfg  | 16 byte       | config (SPS/PPS olası)       |
| `F8`   | extra    | 8 byte        | ek veri                      |
| `F9`   | audio    | 8 byte        | ses (atla)                   |

**I-frame (FC) header açılımı (16 byte):**

```
00 00 01 fc | 03(codec) 06(fps) 2c 24(w/h kod) | ts(4 byte) | dataLen(4 byte LE)
```

Örnek: `...d0 16 00 00` → dataLen=0x16d0=5840; body 5856 - header 16 = 5840 ✓.
**P/FA header (8 byte):** `00 00 01 fX | ... | dataLen(4 byte LE)`.

- Header'dan sonra gelen `dataLen` byte = gerçek H.264 verisi (start-code'suz, ham NAL payload).
- **Doğrulama (entropi):** düz/karanlık bölge paketleri entropi ~0.39 (`d5d5d5...` tekrar); keyframe ~7.98 (sıkıştırma).
  AES olsaydı HEPSİ ~8.0 olurdu. Düşük entropi = şifresiz kanıtı.

### Decode için — SPS/PPS (AÇIK İŞ)

- XM payload start-code'suz, ve SPS/PPS I-frame payload'ının başında DEĞİL.
- SPS/PPS muhtemelen `FE` (mux/config) frame'inde ya da OPMonitor config yanıtında ayrı gelir.
- Çözüm seçenekleri: (a) taze pcap'te I-frame'den önceki FE frame'ini yakala, (b) çözünürlük kodundan (`06 2c 24`) SPS sentezle.
- ffmpeg birleşik 1412 body'lerini `codec_name=h264` tanıdı ama SPS/PPS olmadan `width=0`, decode etmez.

## 6. Kimlik doğrulama (Sofia hash)

- Parola hash = **Sofia "compressed MD5"**: `MD5(parola)` → ardışık ikişer byte toplanır → `[0-9A-Za-z]`
  alfabesinden 8 karakter. **Boş parola → `tlJwpbo6`** (bilinen referans, Go/Python/Node 3 dilde doğrulandı).
- Login JSON: `{"EncryptType":"MD5","LoginType":"DVRIP-Web","PassWord":"<SofiaHash>","UserName":"<user>"}`.
- 8088 token içindeki `{"p":"<8char>"}` = `SofiaHash(parola)` (doğrulandı).

## 7. RPS signaling (8186) — TLS ŞİFRELİ (TEK EKSİK)

- Cihaz seri no → relay IP + **relayToken** çözümlemesi burada.
- Port **8186** (gözlem: `47.245.151.220:8186`, Alibaba). **TLS 1.2** (`16 03 03` handshake, `17 03 03` app data).
- Ham pcap'ten okunamaz (şifreli). Çözmek için: mitmproxy (cert pinning varsa Frida SSL-unpin) ya da Frida SSL_read/write hook.
- **relayToken** (`c_q51oz3...`) bu akıştan gelir; 6611 relay auth'unda (§2 adım 2) kullanılır.
- Not: bootstrap portları oturumdan oturuma değişebilir (gözlem: 8186, 6510, 12132). IP `47.x` (Alibaba) / `101.118.x`.

## 8. Bulut sunucuları (gözlemlenen)

- **Signaling:** `47.245.151.220:8186` (TLS), bazı oturumlarda `:6510`, `:12132`. IP: Alibaba `47.x`, `101.118.x`.
- **Relay (video):** `101.44.0.0/16` bloğu (Xiongmai). Port **6611** (DVRIP, ana) ve/veya **8088** (HTTP/.ts).
- Domainler (APK decompile): `d.xmeye.net`, `app.xmeye.net`, `secu100.net`, `jftech*.com`.

## 9. 8088 HTTP/.ts yolu (alternatif, tutarsız)

- Düz HTTP (openresty). Video **MPEG-TS segmentleri** (HLS gibi), **şifresiz**.
- İstek: `GET /download/<TOKEN>.<md5imza>/<SERI>/<SESSION_HEX>/<N>.ts`  (`User-Agent: Android APP`).
- **Token:** `base64( appId | seriNo | {"p":SofiaHash(pwd),...} | timestamp_ms | "dss" | istemciIP | "md5" ).<md5imza>`.
  appId örn `acbe419b46`. Uzun ömürlü (saatlerce). `.md5imza` + session hex native/signaling'den.
- Segment no ardışık (+2 adım). `.m3u8` yok, istemci numarayı üretir. Segment kısa ömürlü.
- **Tutarsız tetikleniyor** (çoğu oturumda sadece 6611; bazı oturumda playback girişinde 8088 açılır).

## 10. Açık / devam eden işler

1. **RPS relay token (8186 TLS)** — otomasyonun tek eksik parçası (§7). Çözülünce seri+pwd → relay tam otomatik.
2. **SPS/PPS** (§5) — H.264 decode için. FE frame ya da config yanıtı.
3. 6611 relay'e relayToken'sız doğrudan login denenmeli (belki signaling atlanabilir).
4. Go istemci: 6611 bağlan → relay auth → login (1000) → OPMonitor (1413) → 1412 oku → XM frame strip → H.264.

## Değişiklik tespiti (gelecek için)

Bir şey bozulursa sırayla kontrol:

- Relay karşılama JSON'u hâlâ `{"ErrorNum":"200",...}XXEE` mi? (§2.1)
- Login yanıtı (1001) `DataUseAES:false` mi? **true olursa video AES'e geçmiş demektir** → §5 geçersiz, AES gerekir.
- OPMonitor/OPPlayBack JSON alan adları aynı mı? (§4)
- Video frame marker'ları `00 00 01 FC/FD/FA` mı? (§5)
- Signaling portu hâlâ 8186 mi, IP bloğu 47.x / 101.44.x mı? (§7, §8)
