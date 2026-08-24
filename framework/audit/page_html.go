package oplog

// auditPageHTML 是审计查询页(自包含,无外部资源依赖):
// 深色监控风,审计记录表格 + 关键字过滤 + 10 秒自动刷新。
const auditPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>操作审计查询</title>
<style>
  :root { --bg:#0f1420; --card:#171e2e; --line:#232d42; --text:#d7e0f0; --muted:#7d8aa5;
          --green:#2ecc71; --yellow:#f1c40f; --red:#e74c3c; --blue:#3498db; }
  * { margin:0; padding:0; box-sizing:border-box; }
  body { background:var(--bg); color:var(--text); font-family:"Segoe UI",system-ui,-apple-system,sans-serif; padding:24px; }
  .header { display:flex; align-items:center; justify-content:space-between; flex-wrap:wrap; gap:12px; margin-bottom:16px; }
  .header h1 { font-size:20px; font-weight:600; }
  .header .meta { color:var(--muted); font-size:13px; }
  .toolbar { display:flex; gap:12px; align-items:center; margin-bottom:16px; flex-wrap:wrap; }
  .toolbar input { background:var(--card); border:1px solid var(--line); color:var(--text);
    border-radius:8px; padding:8px 12px; font-size:13px; width:280px; outline:none; }
  .toolbar input:focus { border-color:var(--blue); }
  .toolbar .count { color:var(--muted); font-size:13px; }
  .card { background:var(--card); border:1px solid var(--line); border-radius:12px; overflow:hidden; }
  table { width:100%; border-collapse:collapse; font-size:13px; }
  thead th { text-align:left; color:var(--muted); font-weight:500; padding:10px 14px;
    border-bottom:1px solid var(--line); white-space:nowrap; }
  tbody td { padding:9px 14px; border-bottom:1px solid rgba(35,45,66,.5); white-space:nowrap; }
  tbody tr:last-child td { border-bottom:none; }
  tbody tr:hover { background:rgba(52,152,219,.06); }
  .method { display:inline-block; padding:2px 8px; border-radius:6px; font-size:12px; }
  .method.GET { background:rgba(46,204,113,.15); color:var(--green); }
  .method.POST { background:rgba(241,196,15,.15); color:var(--yellow); }
  .method.PUT { background:rgba(52,152,219,.15); color:var(--blue); }
  .method.DELETE { background:rgba(231,76,60,.15); color:var(--red); }
  .status { padding:2px 8px; border-radius:6px; font-size:12px; }
  .status.ok { background:rgba(46,204,113,.15); color:var(--green); }
  .status.err { background:rgba(231,76,60,.15); color:var(--red); }
  .path { font-family:ui-monospace,Consolas,monospace; font-size:12px; color:var(--text); }
  .muted { color:var(--muted); }
  .empty { padding:40px; text-align:center; color:var(--muted); }
  .error { color:var(--red); font-size:13px; padding:16px; }
  .footer { color:var(--muted); font-size:12px; margin-top:16px; text-align:center; }
</style>
</head>
<body>
<div class="header">
  <h1>操作审计查询</h1>
  <div class="meta" id="meta">加载中…</div>
</div>
<div class="toolbar">
  <input id="filter" type="text" placeholder="过滤:路径 / 方法 / 用户 / 动作…">
  <span class="count" id="count"></span>
</div>
<div class="card">
  <table>
    <thead>
      <tr>
        <th>时间</th><th>用户</th><th>方法</th><th>路径</th><th>状态</th><th>耗时</th><th>动作</th>
      </tr>
    </thead>
    <tbody id="rows"></tbody>
  </table>
  <div class="empty" id="empty" style="display:none;">暂无审计记录</div>
  <div class="error" id="error" style="display:none;"></div>
</div>
<div class="footer">go-blackbox oplog · 10 秒自动刷新</div>
<script>
(function () {
  var prefix = "{{PREFIX}}";
  var rowsEl = document.getElementById("rows");
  var emptyEl = document.getElementById("empty");
  var errorEl = document.getElementById("error");
  var countEl = document.getElementById("count");
  var metaEl = document.getElementById("meta");
  var filterEl = document.getElementById("filter");
  var entries = [];

  function fmtTime(seconds) {
    if (!seconds) return "-";
    var d = new Date(seconds * 1000);
    var p = function (n) { return n < 10 ? "0" + n : "" + n; };
    return d.getFullYear() + "-" + p(d.getMonth() + 1) + "-" + p(d.getDate()) +
      " " + p(d.getHours()) + ":" + p(d.getMinutes()) + ":" + p(d.getSeconds());
  }

  function render() {
    var keyword = filterEl.value.trim().toLowerCase();
    var rows = entries;
    if (keyword) {
      rows = entries.filter(function (e) {
        return [e.path, e.method, e.action, e.user_email, e.user_id, e.request_id]
          .join(" ").toLowerCase().indexOf(keyword) >= 0;
      });
    }
    rowsEl.innerHTML = "";
    countEl.textContent = rows.length + " / " + entries.length + " 条";
    if (rows.length === 0) {
      emptyEl.style.display = "block";
      return;
    }
    emptyEl.style.display = "none";
    rows.forEach(function (e) {
      var tr = document.createElement("tr");
      var statusClass = e.status >= 400 ? "err" : "ok";
      var methodClass = (e.method || "GET").toUpperCase();
      tr.innerHTML =
        '<td class="muted">' + fmtTime(e.time) + "</td>" +
        '<td>' + (e.user_email || e.user_id || "-") + "</td>" +
        '<td><span class="method ' + methodClass + '">' + (e.method || "-") + "</span></td>" +
        '<td class="path">' + (e.path || "-") + "</td>" +
        '<td><span class="status ' + statusClass + '">' + (e.status || "-") + "</span></td>" +
        '<td class="muted">' + (e.duration != null ? e.duration + "ms" : "-") + "</td>" +
        '<td class="muted">' + (e.action || "-") + "</td>";
      rowsEl.appendChild(tr);
    });
  }

  function load() {
    fetch(prefix + "/api/audit?offset=0&count=100")
      .then(function (response) { return response.json(); })
      .then(function (body) {
        if (body.code && body.code !== "B0000" && body.code !== 0) {
          throw new Error(body.message || "加载失败");
        }
        var data = body.data || body;
        entries = data.list || [];
        errorEl.style.display = "none";
        metaEl.textContent = "共 " + (data.total || entries.length) + " 条记录";
        render();
      })
      .catch(function (err) {
        errorEl.textContent = "加载失败: " + err.message;
        errorEl.style.display = "block";
      });
  }

  filterEl.addEventListener("input", render);
  load();
  setInterval(load, 10000);
})();
</script>
</body>
</html>
`
