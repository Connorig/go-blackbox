package monitor

// monitorPageHTML 是内置监控页面(自包含,无外部资源依赖):
// 深色监控风,三张资源卡(CPU/内存/磁盘)+ 负载与主机信息 + 趋势曲线 + 5 秒自动刷新。
const monitorPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>服务器资源监控</title>
<style>
  :root { --bg:#0f1420; --card:#171e2e; --line:#232d42; --text:#d7e0f0; --muted:#7d8aa5;
          --green:#2ecc71; --yellow:#f1c40f; --red:#e74c3c; --blue:#3498db; }
  * { margin:0; padding:0; box-sizing:border-box; }
  body { background:var(--bg); color:var(--text); font-family:"Segoe UI",system-ui,-apple-system,sans-serif; padding:24px; }
  .header { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:12px; margin-bottom:20px; }
  .header h1 { font-size:20px; font-weight:600; }
  .header .meta { color:var(--muted); font-size:13px; }
  .grid { display:grid; grid-template-columns:repeat(auto-fit,minmax(280px,1fr)); gap:16px; margin-bottom:16px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:12px; padding:18px; }
  .card h2 { font-size:14px; color:var(--muted); font-weight:500; margin-bottom:12px; display:flex; justify-content:space-between; }
  .card .value { font-size:28px; font-weight:700; }
  .card .sub { font-size:12px; color:var(--muted); margin-top:6px; }
  .bar { height:8px; background:#0b1019; border-radius:4px; margin-top:14px; overflow:hidden; }
  .bar > div { height:100%; border-radius:4px; transition:width .6s ease; }
  .bar .green { background:var(--green); } .bar .yellow { background:var(--yellow); } .bar .red { background:var(--red); }
  .info { display:grid; grid-template-columns:repeat(auto-fit,minmax(220px,1fr)); gap:16px; }
  .info .card { padding:14px 18px; }
  .info .row { display:flex; justify-content:space-between; font-size:13px; padding:5px 0; border-bottom:1px dashed var(--line); }
  .info .row:last-child { border-bottom:none; }
  .info .row span:first-child { color:var(--muted); }
  .chart { margin-top:16px; height:140px; }
  .chart svg { width:100%; height:100%; }
  .badge { display:inline-block; padding:2px 10px; border-radius:10px; font-size:12px; }
  .badge.ok { background:rgba(46,204,113,.15); color:var(--green); }
  .badge.warn { background:rgba(241,196,15,.15); color:var(--yellow); }
  .badge.err { background:rgba(231,76,60,.15); color:var(--red); }
  .error { color:var(--red); font-size:13px; margin-top:8px; }
  .footer { color:var(--muted); font-size:12px; margin-top:16px; text-align:center; }
</style>
</head>
<body>
<div class="header">
  <h1>🖥️ 服务器资源监控</h1>
  <div class="meta" id="meta">加载中…</div>
</div>

<div class="grid">
  <div class="card">
    <h2><span>CPU 使用率</span><span id="cpuBadge"></span></h2>
    <div class="value" id="cpuValue">--</div>
    <div class="bar"><div id="cpuBar" class="green" style="width:0%"></div></div>
    <div class="sub" id="cpuSub"></div>
  </div>
  <div class="card">
    <h2><span>内存使用率</span><span id="memBadge"></span></h2>
    <div class="value" id="memValue">--</div>
    <div class="bar"><div id="memBar" class="green" style="width:0%"></div></div>
    <div class="sub" id="memSub"></div>
  </div>
  <div class="card">
    <h2><span>磁盘使用率</span><span id="diskBadge"></span></h2>
    <div class="value" id="diskValue">--</div>
    <div class="bar"><div id="diskBar" class="green" style="width:0%"></div></div>
    <div class="sub" id="diskSub"></div>
  </div>
</div>

<div class="chart card">
  <h2><span>使用率趋势(最近 20 个采样点,5s 刷新)</span></h2>
  <svg id="chart" viewBox="0 0 600 120" preserveAspectRatio="none"></svg>
</div>

<div class="info">
  <div class="card"><h2>系统负载</h2>
    <div class="row"><span>1 分钟</span><b id="load1">--</b></div>
    <div class="row"><span>5 分钟</span><b id="load5">--</b></div>
    <div class="row"><span>15 分钟</span><b id="load15">--</b></div>
  </div>
  <div class="card"><h2>运行状态</h2>
    <div class="row"><span>系统运行</span><b id="uptime">--</b></div>
    <div class="row"><span>进程运行</span><b id="pUptime">--</b></div>
    <div class="row"><span>协程数</span><b id="goroutines">--</b></div>
  </div>
  <div class="card"><h2>主机信息</h2>
    <div class="row"><span>主机名</span><b id="hostname">--</b></div>
    <div class="row"><span>平台</span><b id="platform">--</b></div>
    <div class="row"><span>Go 版本</span><b id="goVer">--</b></div>
  </div>
</div>

<div class="error" id="error"></div>
<div class="footer">go-blackbox monitor · <span id="version"></span> · 接口受身份校验与限流保护</div>

<script>
(function(){
  var history = { cpu: [], mem: [], disk: [] };
  var chart = document.getElementById("chart");

  function fmtBytes(n) {
    if (n == null) return "--";
    var units = ["B","KB","MB","GB","TB"];
    var i = 0;
    while (n >= 1024 && i < units.length-1) { n /= 1024; i++; }
    return n.toFixed(1) + " " + units[i];
  }
  function fmtDur(s) {
    if (!s && s !== 0) return "--";
    var d = Math.floor(s/86400), h = Math.floor(s%86400/3600), m = Math.floor(s%3600/60);
    return d + "天 " + h + "时 " + m + "分";
  }
  function badge(p) {
    var cls = p > 90 ? "err" : (p > 70 ? "warn" : "ok");
    return '<span class="badge ' + cls + '">' + (p > 90 ? "告警" : (p > 70 ? "偏高" : "正常")) + '</span>';
  }
  function barColor(p) { return p > 90 ? "red" : (p > 70 ? "yellow" : "green"); }
  function renderChart() {
    var lines = [["cpu","#3498db"],["mem","#2ecc71"],["disk","#f1c40f"]];
    var svg = "";
    lines.forEach(function(l){
      var data = history[l[0]]; if (!data.length) return;
      var min = 0, max = 100, w = 600, h = 120;
      var pts = data.map(function(v,i){
        var x = (data.length === 1 ? 0 : i/(data.length-1)) * (w-20) + 10;
        var y = h - 10 - (v-min)/(max-min) * (h-20);
        return x.toFixed(1) + "," + y.toFixed(1);
      }).join(" ");
      svg += '<polyline points="' + pts + '" fill="none" stroke="' + l[1] + '" stroke-width="2"/>';
    });
    chart.innerHTML = svg;
  }
  function refresh() {
    fetch("api/stats", { headers: { "Accept": "application/json" } })
      .then(function(res){ if (!res.ok) throw new Error("HTTP " + res.status); return res.json(); })
      .then(function(s){
        document.getElementById("error").textContent = "";
        var cpu = (s.cpu && s.cpu.usage_percent != null) ? s.cpu.usage_percent : 0;
        var mem = s.memory ? s.memory.usage_percent : 0;
        var disk = s.disk ? s.disk.usage_percent : 0;
        document.getElementById("cpuValue").textContent = cpu.toFixed(1) + "%";
        document.getElementById("cpuBadge").innerHTML = badge(cpu);
        document.getElementById("cpuBar").style.width = cpu + "%";
        document.getElementById("cpuBar").className = barColor(cpu);
        document.getElementById("memValue").textContent = mem.toFixed(1) + "%";
        document.getElementById("memBadge").innerHTML = badge(mem);
        document.getElementById("memBar").style.width = mem + "%";
        document.getElementById("memBar").className = barColor(mem);
        document.getElementById("diskValue").textContent = disk.toFixed(1) + "%";
        document.getElementById("diskBadge").innerHTML = badge(disk);
        document.getElementById("diskBar").style.width = disk + "%";
        document.getElementById("diskBar").className = barColor(disk);
        document.getElementById("cpuSub").textContent = "采样间隔均值(首帧预热中)";
        document.getElementById("memSub").textContent = "已用 " + fmtBytes(s.memory.used) + " / 共 " + fmtBytes(s.memory.total);
        document.getElementById("diskSub").textContent = "已用 " + fmtBytes(s.disk.used) + " / 共 " + fmtBytes(s.disk.total);
        document.getElementById("load1").textContent = s.load ? s.load.load1.toFixed(2) : "--";
        document.getElementById("load5").textContent = s.load ? s.load.load5.toFixed(2) : "--";
        document.getElementById("load15").textContent = s.load ? s.load.load15.toFixed(2) : "--";
        document.getElementById("uptime").textContent = fmtDur(s.uptime);
        document.getElementById("pUptime").textContent = fmtDur(s.process_uptime_seconds);
        document.getElementById("goroutines").textContent = s.goroutines;
        document.getElementById("hostname").textContent = s.hostname;
        document.getElementById("platform").textContent = s.platform;
        document.getElementById("goVer").textContent = s.go_version;
        document.getElementById("version").textContent = s.version;
        document.getElementById("meta").textContent = "采集时间 " + new Date(s.time * 1000).toLocaleTimeString();
        history.cpu.push(cpu); history.mem.push(mem); history.disk.push(disk);
        if (history.cpu.length > 20) { history.cpu.shift(); history.mem.shift(); history.disk.shift(); }
        renderChart();
      })
      .catch(function(e){
        document.getElementById("error").textContent = "监控数据获取失败: " + e.message + " (请确认已登录或接口已授权)";
      });
  }
  refresh();
  setInterval(refresh, 5000);
})();
</script>
</body>
</html>`
