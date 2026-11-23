package main

// playerHTML: çok kanal grid + hls.js canlı izleme arayüzü.
// Bağımlılık yok (hls.js CDN'den); /api/info'dan kanal sayısını alır, grid kurar.
// Köy sitesine gömme için <iframe> ya da PHP sarmalayıcı bunu kullanır.
const playerHTML = `<!doctype html>
<html lang="tr"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1,viewport-fit=cover">
<title>XMEye Wrapper</title>
<style>
:root{color-scheme:dark}
*{box-sizing:border-box}
body{margin:0;background:#0d0f12;color:#e6e8eb;font:14px/1.4 system-ui,Segoe UI,Roboto,sans-serif}
header{display:flex;align-items:center;gap:12px;padding:10px 16px;background:#15181d;border-bottom:1px solid #23272e}
header b{font-size:15px}
header .sub{color:#8b93a1;font-size:12px}
.controls{margin-left:auto;display:flex;gap:8px;align-items:center}
select,button{font:13px system-ui;padding:6px 10px;background:#1d2127;color:#e6e8eb;border:1px solid #333a44;border-radius:6px;cursor:pointer}
button:hover{background:#262c34}
#grid{display:grid;gap:6px;padding:10px}
.cell{position:relative;background:#000;border-radius:6px;overflow:hidden;aspect-ratio:16/9}
.cell video{width:100%;height:100%;object-fit:contain;background:#000}
.cell .lbl{position:absolute;top:6px;left:8px;font-size:12px;color:#cdd3db;text-shadow:0 1px 2px #000;pointer-events:none}
.cell .st{position:absolute;bottom:6px;right:8px;font-size:11px;color:#7f8793}
.err{color:#ff7a7a}
</style></head><body>
<header>
  <b>XMEye Wrapper</b>
  <span class="sub" id="dev">yükleniyor…</span>
  <div class="controls">
    <label>Düzen:
      <select id="layout">
        <option value="1">1</option>
        <option value="4" selected>4</option>
        <option value="9">9</option>
        <option value="16">16</option>
      </select>
    </label>
    <button onclick="startAll()">Tümünü Oynat</button>
  </div>
</header>
<div id="grid"></div>

<script src="https://cdn.jsdelivr.net/npm/hls.js@1.5.17"></script>
<script>
var info={name:'',serial:'',channels:1};
var players={};

async function loadInfo(){
  try{
    var r=await fetch('/api/info'); info=await r.json();
    document.getElementById('dev').textContent=info.name+' · '+info.channels+' kanal';
  }catch(e){ document.getElementById('dev').textContent='bilgi alınamadı'; }
}

function buildGrid(){
  var n=parseInt(document.getElementById('layout').value,10);
  var cols=Math.ceil(Math.sqrt(n));
  var grid=document.getElementById('grid');
  grid.style.gridTemplateColumns='repeat('+cols+',1fr)';
  grid.innerHTML='';
  // kaç hücre: düzen ile kanal sayısının küçüğü
  var count=Math.min(n, info.channels||n);
  for(var ch=0; ch<count; ch++){
    var cell=document.createElement('div'); cell.className='cell';
    var v=document.createElement('video'); v.id='v'+ch; v.muted=true; v.controls=false; v.playsInline=true; v.autoplay=true;
    var lbl=document.createElement('div'); lbl.className='lbl'; lbl.textContent='CAM'+String(ch+1).padStart(2,'0');
    var st=document.createElement('div'); st.className='st'; st.id='st'+ch;
    cell.appendChild(v); cell.appendChild(lbl); cell.appendChild(st); grid.appendChild(cell);
  }
}

function startAll(){
  var n=parseInt(document.getElementById('layout').value,10);
  var count=Math.min(n, info.channels||n);
  for(var ch=0; ch<count; ch++) startChannel(ch);
}

function startChannel(ch){
  var v=document.getElementById('v'+ch), st=document.getElementById('st'+ch);
  if(!v) return;
  var url='/live/'+ch+'.m3u8';
  if(players[ch]){ try{players[ch].destroy();}catch(e){} }
  if(window.Hls && Hls.isSupported()){
    var h=new Hls({liveSyncDurationCount:3,maxBufferLength:10}); players[ch]=h;
    h.loadSource(url); h.attachMedia(v);
    h.on(Hls.Events.ERROR,function(e,d){ if(d.fatal){ st.className='st err'; st.textContent=d.details; } });
    st.textContent='bağlanıyor';
    h.on(Hls.Events.FRAG_LOADED,function(){ st.className='st'; st.textContent='canlı'; });
  } else if(v.canPlayType('application/vnd.apple.mpegurl')){
    v.src=url; // Safari/iOS yerel HLS
  } else { st.className='st err'; st.textContent='HLS desteklenmiyor'; }
}

document.getElementById('layout').addEventListener('change',function(){ buildGrid(); });
(async function(){ await loadInfo(); buildGrid(); startAll(); })();
</script>
</body></html>`
