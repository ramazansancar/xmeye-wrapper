# PHP gömme (tek dosya)

Köy / WordPress / herhangi bir PHP sitesine kamerayı gömmek için. **Eklenti gerekmez.**

## Kullanım

1. Çalışan bir `xmeye-wrapper` köprüsü olmalı (bkz. ana README / `docs/kurulum.tr.md`).
2. `xmeye.php` dosyasını siteye koy.
3. İçindeki `$BRIDGE` değerini köprü adresinle değiştir (örn `https://kamera.koyum.com`).
   - Ya da ortam değişkeni: `XMEYE_BRIDGE`.
4. Tarayıcıdan `xmeye.php`'yi aç.

### WordPress

Bir sayfaya **Özel HTML** bloğu ekleyip şunu koy:

```html
<iframe src="/kamera.php" style="width:100%;height:600px;border:0" allowfullscreen></iframe>
```

(ya da doğrudan köprü adresini iframe'e verebilirsin.)

## Güvenlik

- Kamera şifresi **bu dosyada değil**, köprü sunucusunda durur. PHP sadece köprüyü gömer.
- Köprüyü herkese açacaksan HTTPS kullan ve gerekiyorsa köprüye erişimi kısıtla.
