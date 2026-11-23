# XMEye / Xiongmai protokol spec (çözülen kısım)

Bu belge, tersine mühendislikle çözülen XMEye bulut protokolünü özetler. Go/Python/Node
implementasyonları buna dayanır. Kaynak: gerçek cihaz trafiği (pcap), APK decompile, Frida.

> Durum: kısmen çözüldü. Paket formatı + login + video yolları netleşti; bulut signaling (token üretimi)
> native kodda, devam ediyor.

## 1. Genel bakış

- Cihaz: Xiongmai NVR/DVR/IPC (örnek: AHB80N16T, 16 kanal). **RPS** (bulut relay) modunda, NAT arkası.
- İstemci cihaza iki yoldan erişir: **bulut relay** (seri no + bulut sunucuları) — LAN gerekmez.
- Kimlik: cihaz **seri no (Cloud ID)** + kullanıcı adı + parola.

## 2. DVRIP (Sofia) paket biçimi

TCP, little-endian. 20 byte başlık + gövde:

| offset | boyut | alan        | açıklama                               |
|-------:|------:|-------------|----------------------------------------|
| 0      | 1     | head        | `0xFF` (sabit)                         |
| 1      | 1     | version     | `0x01`                                 |
| 2      | 2     | reserved    | 0                                      |
| 4      | 4     | sessionID   | login'de atanır (örn `0x084e`)         |
| 8      | 4     | sequence    | artan                                  |
| 12     | 1     | totalPacket | çok parçalı mesajda toplam             |
| 13     | 1     | curPacket   | çok parçalı mesajda sıra               |
| 14     | 2     | msgID       | mesaj türü                             |
| 16     | 4     | dataLen     | gövde uzunluğu                         |
| 20     | …     | body        | JSON (metin) ya da ikili; dataLen kadar|

JSON gövdeler genelde `\n` + `\x00` ile sonlanır.

### Mesaj ID'leri (doğrulananlar + standart Sofia)

| msgID | ad                | not                                   |
|------:|-------------------|---------------------------------------|
| 1000  | LOGIN_REQ2        | JSON login isteği                     |
| 1001  | LOGIN_RSP         | login yanıtı (sessionID döner)        |
| 1006  | KEEPALIVE_REQ     | pcap'te doğrulandı (base64 gövde)     |
| 1007  | KEEPALIVE_RSP     | pcap'te doğrulandı                    |
| 1410  | OPMonitor başlat  | akış talebi                           |
| 1412  | OPMonitor veri    | **video frame** (pcap'te doğrulandı)  |
| 1413  | OPMonitor Claim   |                                       |
| 1420  | OPPlayBack        | geçmiş kayıt                          |
| 1430  | OPTalk            | ses                                   |
| 1452  | SYSTEM_TIME       |                                       |

## 3. Kimlik doğrulama

- Parola hash = **Sofia "compressed MD5"**: `MD5(parola)` → ardışık ikişer byte toplanır → `[0-9A-Za-z]`
  alfabesinden 8 karakter. Boş parola → `tlJwpbo6` (bilinen referans, 3 dilde doğrulandı).
- Login JSON: `{"EncryptType":"MD5","LoginType":"DVRIP-Web","PassWord":"<hash>","UserName":"<user>"}`.
- Bulut modunda `login_type=2` (P2P/RPS).

## 4. Video yolları (iki alternatif)

Cihaz/bulut, oturuma göre iki yoldan birini seçer:

### 4a. Port 6611 — ham DVRIP (ANA, hep kullanılıyor)

- `FF 01` DVRIP paketleri, `msgID=1412`, sabit `sessionID`.
- Gövde **ikili/şifreli** (AES; XAES sınıfı, native `libFunSDK.so`). Anahtar türetimi çözülüyor (Ghidra).
- Canlı izleme güvenilir biçimde bu yoldan gelir.

### 4b. Port 8088 — HTTP / .ts (KOLAY ama tutarsız)

- Düz HTTP (openresty). Video **MPEG-TS segmentleri** (HLS gibi), **şifresiz**.
- İstek: `GET /download/<TOKEN>/<SERI_NO>/<SESSION_HEX>/<N>.ts`  (`User-Agent: Android APP`).
- Segment no ardışık artar (gözlem: ana akış 1,3,5,…; adım +2). `.m3u8` playlist YOK — istemci numarayı kendi üretir.
- Segmentler kısa ömürlü (canlı kenar); üretilir üretilmez çekilmeli.
- Daha çok playback/download'da tetiklenir; canlı çoğunlukla 6611.

### Token yapısı (8088 yolu için)

`base64( appId | seriNo | <iç-JSON-b64> | timestamp_ms | "dss" | istemciIP | "md5" ).<md5imza>`

- İç JSON: `{"p":"<8char>", …}` — `<8char>` = **`SofiaHash(parola)`** (çözüldü).
- `appId`, `.md5imza` native üretiliyor (henüz Go'da sıfırdan üretilemiyor; yakala-kullan ya da Ghidra).
- Token **uzun ömürlü** (saatlerce geçerli). `.ts` bir kez gerçek token ile HTTP 200 çekildi (kanıt).

## 5. Bulut sunucuları (gözlemlenen)

- Signaling/bootstrap portları oturumdan oturuma değişir: `12132`, `6510`, `6611`.
- Relay (video): `:8088` (HTTP) ve/veya `:6611` (DVRIP). IP bloğu `101.44.0.0/16` (Xiongmai), ayrıca Alibaba/`47.245.x`, `213.250.x`.
- Domainler (APK): `d.xmeye.net`, `app.xmeye.net`, `secu100.net`, `jftech*.com`.

## 6. Açık / devam eden

- Bulut signaling handshake'in tam byte formatı (12132/6611 login → sessionID + token).
- 6611 video AES anahtar türetimi (XAES, native).
- Token'ı sıfırdan üretme (appId kaynağı + md5imza salt).
