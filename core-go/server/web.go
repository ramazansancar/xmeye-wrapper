package main

// playerHTML: NVR-duzeni izleme arayuzu — acik/fonksiyonel "guvenlik paneli" temasi.
// Duzen: ust header + buyuk ana ekran (sol) + kontrolcu paneli (sag) + alt yatay galeri serit.
// Tasarim ilkeleri: basit, responsive (mobil: sag panel + serit alta iner), SIFIR gereksiz animasyon,
// kaynak-tasarruflu (gradient/golge/transition YOK, sistem fontu, tek dis bagimlilik hls.js).
//   - Galeri serit: yatay kaydirilir, tik = ana ekrana al, cift tik = tam ekran.
//   - Kontrolcu: PTZ (yon+zoom), akis kalite (ana/alt), snapshot/kayit, playback, genel komut.
//     Komutlar POST /api/cmd'e gider (backend 6611 istemci hazir olunca islenir).
// Koy sitesine gomme: PHP sarmalayici bunu <iframe> ile gomer.
const playerHTML = `<!doctype html>
<html lang="tr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>Kamera Izleme</title>
<style>
*{box-sizing:border-box}
html,body{margin:0;height:100%}
body{background:#eceef0;color:#14181c;font:14px system-ui,Segoe UI,Roboto,Arial,sans-serif;
  height:100vh;display:flex;flex-direction:column;overflow:hidden}

header{flex:0 0 auto;display:flex;flex-wrap:wrap;align-items:center;gap:8px 14px;
  padding:8px 12px;background:#fff;border-bottom:2px solid #c3c8cd}
header .dev{font-weight:600;font-size:15px}
header .meta{color:#5a636c;font-size:12px}
.sp{margin-left:auto}

/* orta: ana ekran + sag panel */
.mid{flex:1 1 auto;display:flex;min-height:0}
.stage{flex:1 1 auto;position:relative;background:#000;min-width:0;min-height:0}
.stage video{width:100%;height:100%;object-fit:contain;background:#000;display:block}
.stage .bar{position:absolute;top:0;left:0;right:0;display:flex;justify-content:space-between;
  padding:4px 8px;background:#fff;border-bottom:1px solid #b4bbc1;font-size:13px}
.stage .bar .n{font-weight:600}
.stage .bar .s{color:#5a636c}
.stage .bar .s.err{color:#b00020}
.stage .bar .s.live{color:#0a7d2c}

/* sag kontrolcu */
.ctrl{flex:0 0 190px;background:#f6f7f8;border-left:1px solid #b4bbc1;
  overflow-y:auto;padding:8px;display:flex;flex-direction:column;gap:10px}
.ctrl h3{margin:0 0 4px;font-size:11px;letter-spacing:.04em;text-transform:uppercase;color:#5a636c;font-weight:600}
.grp{border:1px solid #c3c8cd;background:#fff;padding:7px}
.ptz{display:grid;grid-template-columns:repeat(3,1fr);gap:3px}
.ptz button{aspect-ratio:1;font-size:15px}
.row{display:flex;gap:4px;flex-wrap:wrap}
.ctrl button,.ctrl select{font:13px system-ui;padding:5px 8px;background:#fff;color:#14181c;
  border:1px solid #b4bbc1;cursor:pointer}
.ctrl button:active{background:#dfe3e6}
.ctrl button.wide{flex:1}
.ctrl .seg button.on{background:#14181c;color:#fff}
.ctrl input[type=datetime-local]{font:12px system-ui;padding:4px;border:1px solid #b4bbc1;width:100%}

/* alt galeri serit */
.strip{flex:0 0 auto;display:flex;gap:4px;padding:4px;background:#d9dde1;border-top:2px solid #c3c8cd;
  overflow-x:auto;overflow-y:hidden}
.thumb{flex:0 0 auto;width:150px;height:92px;position:relative;background:#000;
  border:2px solid #b4bbc1;cursor:pointer}
.thumb.on{border-color:#14181c}
.thumb video{width:100%;height:100%;object-fit:cover;background:#000;display:block}
.thumb .t{position:absolute;top:0;left:0;right:0;padding:1px 4px;font-size:11px;
  background:rgba(255,255,255,.88);color:#14181c}
.thumb .d{position:absolute;bottom:2px;right:3px;font-size:10px;color:#9aa2ab}
.thumb .d.live{color:#43d16b}

/* duzen secici (header) */
.layout{display:flex;border:1px solid #b4bbc1}
.layout button{font:13px system-ui;padding:5px 11px;background:#fff;color:#14181c;
  border:0;border-right:1px solid #d5dadf;cursor:pointer}
.layout button:last-child{border-right:0}
.layout button.on{background:#14181c;color:#fff}
.go{font:13px system-ui;padding:5px 12px;background:#fff;color:#14181c;border:1px solid #b4bbc1;cursor:pointer}

/* grid modu: stage icinde cok kamera */
.stage #main{display:block}
.stage.grid #main{display:none}
.stage .gwrap{display:none;position:absolute;inset:26px 0 0 0}
.stage.grid .gwrap{display:grid;gap:3px;padding:3px}
.stage .gwrap .gc{position:relative;background:#000;border:1px solid #2a2f35;min-height:0;overflow:hidden}
.stage .gwrap .gc video{width:100%;height:100%;object-fit:contain;background:#000}
.stage .gwrap .gc .gl{position:absolute;top:0;left:0;padding:0 4px;font-size:11px;
  background:rgba(255,255,255,.85);color:#14181c}

/* alt zaman cizgisi (senkron playback) */
.tl{flex:0 0 auto;background:#e4e7ea;border-top:2px solid #c3c8cd;padding:4px 6px 6px}
.tlhead{display:flex;align-items:center;gap:8px;font-size:12px;margin-bottom:4px}
.tltitle{font-weight:600}
.tldate{color:#14181c}
.tlmode{padding:1px 6px;background:#0a7d2c;color:#fff;font-size:11px}
.tlmode.pb{background:#b06a00}
.tlsp{margin-left:auto}
.tlzoom{display:flex;border:1px solid #b4bbc1}
.tlzoom button{font:12px system-ui;padding:3px 9px;background:#fff;color:#14181c;border:0;border-right:1px solid #d5dadf;cursor:pointer}
.tlzoom button:last-child{border-right:0}
.tlzoom button.on{background:#14181c;color:#fff}
.tlgo{font:12px system-ui;padding:3px 10px;background:#fff;border:1px solid #b4bbc1;cursor:pointer}
.tltrack{position:relative;height:34px;background:#fff;border:1px solid #b4bbc1;
  overflow:hidden;cursor:ew-resize;user-select:none;touch-action:none}
.tlfill{position:absolute;inset:0}
/* saat cizgileri + kayit bolgeleri JS ile canvas yerine div; basit tutuldu */
.tltick{position:absolute;top:0;bottom:0;width:1px;background:#d5dadf}
.tllabel{position:absolute;top:1px;font-size:10px;color:#5a636c;transform:translateX(2px)}
.tlrec{position:absolute;top:18px;bottom:3px;background:#2d6cdf;opacity:.55}
.tlrec.alarm{background:#c0392b;opacity:.6}
.tlhair{position:absolute;top:0;bottom:0;width:2px;background:#14181c;left:50%}
.tlhair::after{content:'';position:absolute;top:-1px;left:-3px;width:8px;height:8px;background:#14181c}

@media(max-width:720px){
  .mid{flex-direction:column}
  .ctrl{flex:0 0 auto;border-left:0;border-top:1px solid #b4bbc1;
    flex-direction:column}
  .ctrl .grp{flex:0 0 auto}
}
</style></head><body>
<header>
  <span class="dev" id="dev">Kamera Izleme</span>
  <span class="meta" id="meta"></span>
  <span class="sp"></span>
  <span class="layout" id="layout">
    <button data-n="1" class="on">Tekli</button>
    <button data-n="4">4</button>
    <button data-n="9">9</button>
    <button data-n="16">16</button>
  </span>
  <button class="go" onclick="toggleFs()">Tam Ekran</button>
</header>

<div class="mid">
  <div class="stage" id="stage">
    <video id="main" muted playsinline autoplay></video>
    <div class="gwrap" id="gwrap"></div>
    <div class="bar"><span class="n" id="mainName">—</span><span class="s" id="mainState">—</span></div>
  </div>

  <div class="ctrl">
    <div class="grp">
      <h3>Yon / Zoom (PTZ)</h3>
      <div class="ptz">
        <button data-ptz="LeftUp">&#8598;</button>
        <button data-ptz="Up">&#8593;</button>
        <button data-ptz="RightUp">&#8599;</button>
        <button data-ptz="Left">&#8592;</button>
        <button data-ptz="" disabled>&#9679;</button>
        <button data-ptz="Right">&#8594;</button>
        <button data-ptz="LeftDown">&#8601;</button>
        <button data-ptz="Down">&#8595;</button>
        <button data-ptz="RightDown">&#8600;</button>
      </div>
      <div class="row" style="margin-top:4px">
        <button class="wide" data-ptz="ZoomTile">Zoom +</button>
        <button class="wide" data-ptz="ZoomWide">Zoom &minus;</button>
      </div>
    </div>

    <div class="grp seg">
      <h3>Akis Kalitesi</h3>
      <div class="row" id="qual">
        <button class="wide on" data-stream="0">HD</button>
        <button class="wide" data-stream="1">SD</button>
      </div>
    </div>

    <div class="grp">
      <h3>Goruntu / Kayit</h3>
      <div class="row">
        <button class="wide" onclick="snap()">Foto</button>
        <button class="wide" id="recBtn" onclick="toggleRec()">Kayit</button>
      </div>
    </div>

    <div class="grp">
      <h3>Gecmis (Playback)</h3>
      <input type="datetime-local" id="pbTime">
      <div class="row" style="margin-top:4px">
        <button class="wide" onclick="playback()">Oynat</button>
        <button class="wide" onclick="goLive()">Canli</button>
      </div>
    </div>

    <div class="grp">
      <h3>Komut Gonder</h3>
      <select id="cmdSel"></select>
      <button class="wide" style="margin-top:4px;width:100%" onclick="sendSelCmd()">Gonder</button>
    </div>
  </div>
</div>

<div class="strip" id="strip"></div>

<div class="tl" id="tl">
  <div class="tlhead">
    <span class="tltitle">Zaman çizgisi</span>
    <span class="tldate" id="tlDate">—</span>
    <span class="tlmode" id="tlMode">CANLI</span>
    <span class="tlsp"></span>
    <span class="tlzoom" id="tlZoom">
      <button data-z="24">24s</button>
      <button data-z="6">6s</button>
      <button data-z="1">1s</button>
    </span>
    <button class="tlgo" onclick="tlGoLive()">Canlıya dön</button>
  </div>
  <div class="tltrack" id="tlTrack">
    <div class="tlfill" id="tlFill"></div>
    <div class="tlhair" id="tlHair"></div>
  </div>
</div>

<script src="https://cdn.jsdelivr.net/npm/hls.js@1.5.17"></script>
<script src="https://cdn.jsdelivr.net/npm/jmuxer@2.0.5/dist/jmuxer.min.js"></script>
<script>
var info={name:'',serial:'',channels:1};
var players={};      // kanal -> {ws, jmuxer} (thumbnail)
var gplayers={};     // kanal -> {ws, jmuxer} (grid modu)
var mainHls=null;    // ana ekran {ws, jmuxer}
var active=0;        // ana ekrandaki kanal
var stream=0;        // 0=ana,1=alt
var layout=1;        // 1=tekli (ana ekran+kontrol), 4/9/16=grid
// DVRIP komut seti (cihazdan toplanan + standart). Backend /api/cmd isleyecek.
var CMDS=[
  {id:1020,ad:'Sistem Bilgisi'},{id:1040,ad:'Config Oku'},{id:1042,ad:'Config Yaz'},
  {id:1360,ad:'Kanal Adi'},{id:1362,ad:'OSD Ayar'},{id:1400,ad:'Kayit Ayar'},
  {id:1430,ad:'Sesli Konus (OPTalk)'},{id:1452,ad:'Saat Oku'},{id:1650,ad:'Ag Ayar'},
  {id:2350,ad:'Alarm Durum'}
];

function api(path,body){
  return fetch(path,{method:'POST',headers:{'content-type':'application/json'},
    body:JSON.stringify(body||{})}).catch(function(){});
}

function loadInfo(cb){
  fetch('/api/info').then(function(r){return r.json();}).then(function(j){
    info=j;
    document.getElementById('dev').textContent=info.name||'Kamera Izleme';
    document.getElementById('meta').textContent=(info.channels||0)+' kanal';
    cb();
  }).catch(function(){ document.getElementById('meta').textContent='bilgi alinamadi'; cb(); });
}

// --- TEK WebSocket çoğullama ---
// Tek WS üstünden tüm kanallar. play/stop mesajı; her frame [ch][H.264]. Kanal→{video,jmuxer,onLive} map.
var hub={ws:null,subs:{},ready:false,queue:[]};
function hubConnect(){
  if(hub.ws && (hub.ws.readyState===0||hub.ws.readyState===1)) return;
  var scheme=(location.protocol==='https:')?'wss':'ws';
  hub.ws=new WebSocket(scheme+'://'+location.host+'/ws'); hub.ws.binaryType='arraybuffer';
  hub.ws.onopen=function(){ hub.ready=true; hub.queue.forEach(function(m){ hub.ws.send(m); }); hub.queue=[]; };
  hub.ws.onmessage=function(ev){
    var b=new Uint8Array(ev.data); if(b.length<2) return;
    var ch=b[0]; var s=hub.subs[ch]; if(!s) return;
    try{ s.jmuxer.feed({video:b.subarray(1)});
      if(!s.live){ s.live=true; if(s.onLive) s.onLive(); } }catch(e){}
  };
  hub.ws.onclose=function(){ hub.ready=false;
    for(var ch in hub.subs){ if(hub.subs[ch].onErr) hub.subs[ch].onErr(); } };
  hub.ws.onerror=function(){ };
}
function hubSend(obj){ var m=JSON.stringify(obj);
  if(hub.ready && hub.ws.readyState===1) hub.ws.send(m); else hub.queue.push(m); }

// attachVideo: kanalı tek-WS hub'a abone eder, jmuxer+MSE ile video'ya oynatır. {destroy} döner.
function attachVideo(video,ch,strm,onLive,onErr){
  hubConnect();
  // önceki abonelik varsa temizle
  if(hub.subs[ch]){ try{hub.subs[ch].jmuxer.destroy();}catch(e){} }
  if(window.JMuxer && 'MediaSource' in window){
    // flushingTime düşük = düşük gecikme. fps sabit vermiyoruz (değişken frame).
    var jm=new JMuxer({node:video,mode:'video',flushingTime:50,maxDelay:500,clearBuffer:true,debug:false,
      onReady:function(){ try{video.play();}catch(e){} },
      onError:function(){ if(onErr) onErr(); }});
    hub.subs[ch]={jmuxer:jm,onLive:onLive,onErr:onErr,live:false};
    hubSend({type:'play',ch:ch,stream:strm});
    return {destroy:function(){ hubSend({type:'stop',ch:ch});
      if(hub.subs[ch]){ try{hub.subs[ch].jmuxer.destroy();}catch(e){} delete hub.subs[ch]; } }};
  }
  // fallback: HLS (REST alternatif, ayrı bağlantı)
  if(window.Hls && Hls.isSupported()){
    var h=new Hls({liveSyncDurationCount:3,maxBufferLength:10});
    h.loadSource('/live/'+ch+'.m3u8'); h.attachMedia(video);
    if(onLive) h.on(Hls.Events.FRAG_LOADED,onLive);
    if(onErr) h.on(Hls.Events.ERROR,function(e,d){ if(d.fatal) onErr(); });
    return {destroy:function(){ try{h.destroy();}catch(e){} }};
  }
  if(onErr) onErr(); return null;
}

function buildStrip(){
  var strip=document.getElementById('strip');
  strip.innerHTML='';
  var n=info.channels||1;
  for(var ch=0;ch<n;ch++){
    (function(ch){
      var t=document.createElement('div'); t.className='thumb'+(ch===active?' on':''); t.id='t'+ch;
      var v=document.createElement('video'); v.id='tv'+ch; v.muted=true; v.playsInline=true; v.autoplay=true;
      var lb=document.createElement('div'); lb.className='t'; lb.textContent='CAM'+String(ch+1).padStart(2,'0');
      var d=document.createElement('div'); d.className='d'; d.id='td'+ch; d.textContent='-';
      t.appendChild(v); t.appendChild(lb); t.appendChild(d);
      t.onclick=function(){ setActive(ch); };
      t.ondblclick=function(){ setActive(ch); fsEl(document.querySelector('.stage')); };
      strip.appendChild(t);
      // NOT: thumbnail canlı akışı şimdilik kapalı — her kanal 2 cloud bağlantısı açıyor,
      // 16 kanal paralel NAT'ı boğuyor. Tek-WS çoğullama + bağlantı havuzu gelince açılacak.
      d.textContent='';
    })(ch);
  }
}

function startThumb(ch){
  // thumbnail canlı akış KAPALI (NAT: her kanal 2 bağlantı; 16 paralel boğuyor).
  // Tek-WS bağlantı havuzu gelince açılacak. Şimdilik sadece ana ekran oynar.
}

function setActive(ch){
  active=ch;
  var th=document.querySelectorAll('.thumb');
  for(var i=0;i<th.length;i++) th[i].className='thumb'+(i===ch?' on':'');
  document.getElementById('mainName').textContent='CAM'+String(ch+1).padStart(2,'0');
  var mv=document.getElementById('main'), st=document.getElementById('mainState');
  st.className='s'; st.textContent='baglaniyor';
  if(mainHls){ try{mainHls.destroy();}catch(e){} }
  mainHls=attachVideo(mv,ch,stream,
    function(){ st.className='s live'; st.textContent='canli'; },
    function(){ st.className='s err'; st.textContent='hata'; });
}

// --- kontrolcu ---
function sendPtz(dir){ if(dir) api('/api/cmd',{channel:active,type:'ptz',dir:dir}); }
function snap(){ api('/api/cmd',{channel:active,type:'snapshot'}); }
var recording=false;
function toggleRec(){ recording=!recording;
  document.getElementById('recBtn').textContent=recording?'Durdur':'Kayit';
  api('/api/cmd',{channel:active,type:'record',on:recording}); }
function playback(){ var t=document.getElementById('pbTime').value;
  if(t) api('/api/cmd',{channel:active,type:'playback',time:t}); }
function goLive(){ setActive(active); }
function setStream(s){ stream=s;
  var bs=document.querySelectorAll('#qual button');
  for(var i=0;i<bs.length;i++) bs[i].className='wide'+(parseInt(bs[i].getAttribute('data-stream'),10)===s?' on':'');
  setActive(active); }
function sendSelCmd(){ var id=parseInt(document.getElementById('cmdSel').value,10);
  api('/api/cmd',{channel:active,type:'raw',cmdId:id}); }

// --- duzen (tekli vs grid) ---
function setLayout(n){
  layout=n;
  var bs=document.querySelectorAll('#layout button');
  for(var i=0;i<bs.length;i++) bs[i].className=(parseInt(bs[i].getAttribute('data-n'),10)===n?'on':'');
  var stage=document.getElementById('stage');
  // grid instancelari temizle
  for(var k in gplayers){ try{gplayers[k].destroy();}catch(e){} } gplayers={};
  if(n===1){
    stage.className='stage';
    setActive(active);
  } else {
    stage.className='stage grid';
    if(mainHls){ try{mainHls.destroy();}catch(e){} mainHls=null; }
    buildGrid(n);
  }
}

function buildGrid(n){
  var gw=document.getElementById('gwrap');
  var count=Math.min(n, info.channels||n);
  var cols=Math.ceil(Math.sqrt(n)), rows=Math.ceil(count/cols);
  gw.style.gridTemplateColumns='repeat('+cols+',1fr)';
  gw.style.gridTemplateRows='repeat('+rows+',1fr)';
  gw.innerHTML='';
  for(var k=0;k<count;k++){
    (function(ch){
      var c=document.createElement('div'); c.className='gc';
      var v=document.createElement('video'); v.id='gv'+ch; v.muted=true; v.playsInline=true; v.autoplay=true;
      var l=document.createElement('div'); l.className='gl'; l.textContent='CAM'+String(ch+1).padStart(2,'0');
      c.appendChild(v); c.appendChild(l);
      c.onclick=function(){ setLayout(1); setActive(ch); };
      c.ondblclick=function(){ fsEl(c); };
      gw.appendChild(c);
      gplayers[ch]=attachVideo(v,ch,stream,null,null);
    })(k);
  }
}

// tam ekran
function fsEl(el){ if(el.requestFullscreen) el.requestFullscreen(); }
function toggleFs(){ if(document.fullscreenElement) document.exitFullscreen();
  else fsEl(document.querySelector('.stage')); }

// event baglama
document.getElementById('layout').addEventListener('click',function(e){
  var b=e.target.closest('button'); if(!b) return; setLayout(parseInt(b.getAttribute('data-n'),10)); });
document.querySelector('.ptz').addEventListener('click',function(e){
  var b=e.target.closest('button'); if(!b) return; sendPtz(b.getAttribute('data-ptz')); });
document.getElementById('qual').addEventListener('click',function(e){
  var b=e.target.closest('button'); if(!b) return; setStream(parseInt(b.getAttribute('data-stream'),10)); });

(function initCmds(){
  var sel=document.getElementById('cmdSel');
  CMDS.forEach(function(c){ var o=document.createElement('option');
    o.value=c.id; o.textContent=c.id+' · '+c.ad; sel.appendChild(o); });
})();

// --- senkron zaman cizgisi ---
// Merkez an (centerMs) = hairline. Zoom = gorunen pencere (saat). Sag kenar = su an (canli sinir).
// Kaydir: centerMs degisir; gun sinirini asinca otomatik onceki/sonraki gune gecer.
// centerMs < now => PLAYBACK modu (tum gorunen kameralar senkron). Sag uca gelince CANLI.
var tlZoomHours=24;       // 24 / 6 / 1
var tlCenter=Date.now();  // hairline zamani (ms)
var tlNow=Date.now();
var tlRecs={};            // ch -> [{s,e,alarm}] kayit araliklari (backend /api/records; simdilik bos)
var tlDragging=false, tlDragX=0, tlDragCenter=0;
var tlApplyTimer=null;

function tlWindowMs(){ return tlZoomHours*3600*1000; }
function tlPad(n){ return String(n).padStart(2,'0'); }
function tlFmtDate(ms){ var d=new Date(ms);
  return d.getFullYear()+'-'+tlPad(d.getMonth()+1)+'-'+tlPad(d.getDate()); }
function tlFmtClock(ms){ var d=new Date(ms); return tlPad(d.getHours())+':'+tlPad(d.getMinutes()); }

function tlRender(){
  tlNow=Date.now();
  if(tlCenter>tlNow) tlCenter=tlNow;           // gelecege gidilemez
  var fill=document.getElementById('tlFill'); if(!fill) return;
  var win=tlWindowMs();
  var left=tlCenter-win/2, right=tlCenter+win/2;
  fill.innerHTML='';
  // saat/dakika cizgileri
  var stepMs = tlZoomHours>=24 ? 2*3600*1000 : (tlZoomHours>=6 ? 3600*1000 : 15*60*1000);
  var t=Math.ceil(left/stepMs)*stepMs;
  for(; t<=right; t+=stepMs){
    var x=((t-left)/win)*100;
    var tick=document.createElement('div'); tick.className='tltick'; tick.style.left=x+'%';
    fill.appendChild(tick);
    var lb=document.createElement('div'); lb.className='tllabel'; lb.style.left=x+'%'; lb.textContent=tlFmtClock(t);
    fill.appendChild(lb);
  }
  // kayit bolgeleri (aktif kanal; grid'de tum gorunenlerin birlesimi gosterilebilir)
  var recs=tlRecs[active]||[];
  recs.forEach(function(r){
    var a=Math.max(r.s,left), b=Math.min(r.e,right); if(b<=a) return;
    var x=((a-left)/win)*100, w=((b-a)/win)*100;
    var el=document.createElement('div'); el.className='tlrec'+(r.alarm?' alarm':'');
    el.style.left=x+'%'; el.style.width=w+'%'; fill.appendChild(el);
  });
  // ust bilgi
  document.getElementById('tlDate').textContent=tlFmtDate(tlCenter)+' '+tlFmtClock(tlCenter);
  var live=(tlNow-tlCenter)<30000; // son 30sn = canli sinir
  var m=document.getElementById('tlMode');
  m.textContent=live?'CANLI':'GEÇMİŞ'; m.className='tlmode'+(live?'':' pb');
}

function tlZoomSet(h){ tlZoomHours=h;
  var bs=document.querySelectorAll('#tlZoom button');
  for(var i=0;i<bs.length;i++) bs[i].className=(parseInt(bs[i].getAttribute('data-z'),10)===h?'on':'');
  tlRender();
}
function tlGoLive(){ tlCenter=Date.now(); tlRender(); tlApply(); }

// kaydirma bitince: canli sinirdaysa canli, degilse senkron playback (tum gorunen kameralar)
function tlApply(){
  var live=(Date.now()-tlCenter)<30000;
  // canlı modda video zaten açık (setActive/setLayout yönetir) — tlApply DOKUNMAZ (çift play'i önler).
  if(live) return;
  // playback: backend'e zaman damgali istek (6611 OPPlayBack). senkron tüm görünen kanallar.
  var chans = (layout===1)?[active]:(function(){var a=[];var c=Math.min(layout,info.channels||layout);for(var k=0;k<c;k++)a.push(k);return a;})();
  var d=new Date(tlCenter);
  var ts=d.getFullYear()+'-'+tlPad(d.getMonth()+1)+'-'+tlPad(d.getDate())+' '+tlPad(d.getHours())+':'+tlPad(d.getMinutes())+':'+tlPad(d.getSeconds());
  chans.forEach(function(ch){ api('/api/cmd',{channel:ch,type:'playback',time:ts,sync:true}); });
}
function tlApplyDebounced(){ clearTimeout(tlApplyTimer); tlApplyTimer=setTimeout(tlApply,350); }

// surukle = zamanda pan
(function tlDrag(){
  var tr=document.getElementById('tlTrack'); if(!tr) return;
  function down(x){ tlDragging=true; tlDragX=x; tlDragCenter=tlCenter; }
  function move(x){ if(!tlDragging) return;
    var w=tr.clientWidth||1; var dx=x-tlDragX;
    tlCenter=tlDragCenter - (dx/w)*tlWindowMs();  // saga suruk = geriye (gecmis)
    tlRender(); tlApplyDebounced();
  }
  function up(){ if(!tlDragging) return; tlDragging=false; tlApply(); }
  tr.addEventListener('mousedown',function(e){ down(e.clientX); });
  window.addEventListener('mousemove',function(e){ move(e.clientX); });
  window.addEventListener('mouseup',up);
  tr.addEventListener('touchstart',function(e){ down(e.touches[0].clientX); },{passive:true});
  window.addEventListener('touchmove',function(e){ if(tlDragging){ move(e.touches[0].clientX); } },{passive:true});
  window.addEventListener('touchend',up);
  // tekerlek = zoom
  tr.addEventListener('wheel',function(e){ e.preventDefault();
    var opts=[24,6,1]; var i=opts.indexOf(tlZoomHours);
    if(e.deltaY<0 && i<opts.length-1) tlZoomSet(opts[i+1]);
    else if(e.deltaY>0 && i>0) tlZoomSet(opts[i-1]);
  },{passive:false});
})();
document.getElementById('tlZoom').addEventListener('click',function(e){
  var b=e.target.closest('button'); if(!b) return; tlZoomSet(parseInt(b.getAttribute('data-z'),10)); });

// kayit araliklari backend'den (OPQueryRecord). Yoksa bos (cizgi bos gorunur).
function tlLoadRecords(){
  fetch('/api/records?channel='+active+'&date='+tlFmtDate(tlCenter))
    .then(function(r){return r.json();})
    .then(function(j){ tlRecs[active]=j.records||[]; tlRender(); })
    .catch(function(){ /* backend hazir degil; cizgi kayit bolgesiz */ });
}

setInterval(function(){ if(!tlDragging) tlRender(); },5000); // canli kenar kaymasi

loadInfo(function(){
  buildStrip();
  var h=parseInt((location.hash||'').replace('#',''),10);
  setLayout((h===4||h===9||h===16)?h:1);
  tlZoomSet(24); tlGoLive();
});
</script>
</body></html>`
