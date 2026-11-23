<?php
/**
 * xmeye-wrapper — tek dosya PHP gömme.
 *
 * Köy / WordPress / herhangi bir PHP sitesine kopyala-yapıştır. Eklenti gerekmez.
 * Çalışan bir xmeye-wrapper köprüsüne bağlanıp kameraları sayfaya gömer.
 *
 * KULLANIM:
 *   1. Aşağıdaki $BRIDGE değerini köprü adresinle değiştir (örn https://kamera.koyum.com).
 *   2. Bu dosyayı siteye koy (ör. kamera.php) ve tarayıcıdan aç.
 *      WordPress'te: bir sayfaya "Özel HTML" bloğu olarak <iframe src="kamera.php"></iframe> de gömebilirsin.
 *
 * NOT: Köprü, HLS akışını ve /api/info'yu sağlar. Bu dosya sadece onu güvenli biçimde gömer;
 *      kamera şifresi burada DEĞİL, köprü tarafında durur.
 */

// ==== YAPILANDIRMA ====
$BRIDGE = getenv('XMEYE_BRIDGE') ?: 'http://localhost:8080'; // köprü adresi
$TITLE  = 'Köy Kameraları';
// ======================

header('Content-Type: text/html; charset=utf-8');
$bridge = rtrim($BRIDGE, '/');
?><!doctype html>
<html lang="tr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title><?php echo htmlspecialchars($TITLE); ?></title>
<style>
  html,body{margin:0;height:100%;background:#0d0f12}
  iframe{border:0;width:100%;height:100%;display:block}
</style></head><body>
<!--
  En basit yol: köprünün kendi arayüzünü iframe ile göm.
  (Köprü zaten çok kanal grid + hls.js içeriyor.)
-->
<iframe src="<?php echo htmlspecialchars($bridge); ?>/" allow="autoplay; fullscreen" allowfullscreen></iframe>
</body></html>
