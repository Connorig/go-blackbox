package apidoc

// renderPage 渲染 API 浏览页面(内嵌,无外部资源)。
func renderPage(prefix string) string {
	return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>接口文档</title>
<style>
  :root { --bg:#0f1420; --card:#171e2e; --line:#232d42; --text:#d7e0f0; --muted:#7d8aa5;
          --green:#2ecc71; --yellow:#f1c40f; --red:#e74c3c; --blue:#3498db; }
  * { margin:0; padding:0; box-sizing:border-box; }
  body { background:var(--bg); color:var(--text); font-family:"Segoe UI",system-ui,sans-serif; padding:24px; }
  h1 { font-size:20px; margin-bottom:6px; }
  .sub { color:var(--muted); font-size:13px; margin-bottom:20px; }
  .group { margin-bottom:18px; }
  .group h2 { font-size:15px; color:var(--blue); margin-bottom:8px; border-left:3px solid var(--blue); padding-left:8px; }
  .op { background:var(--card); border:1px solid var(--line); border-radius:8px; padding:12px 14px; margin-bottom:8px; }
  .op-head { display:flex; align-items:center; gap:10px; cursor:pointer; }
  .method { display:inline-block; padding:3px 10px; border-radius:4px; font-size:12px; font-weight:600; color:#fff; min-width:64px; text-align:center; }
  .m-GET{background:#2ecc71}.m-POST{background:#3498db}.m-PUT{background:#f39c12}.m-DELETE{background:#e74c3c}
  .path { font-family:Consolas,monospace; font-size:13px; }
  .summary { color:var(--muted); font-size:13px; flex:1; }
  .op-body { display:none; margin-top:10px; font-size:13px; }
  .op.open .op-body { display:block; }
  .op-body table { width:100%; border-collapse:collapse; margin-top:6px; }
  .op-body th { background:#1c2438; color:var(--muted); text-align:left; padding:5px 8px; border:1px solid var(--line); font-size:12px; }
  .op-body td { padding:5px 8px; border:1px solid var(--line); font-size:12px; }
  .desc { color:var(--muted); font-size:12px; margin:6px 0; }
  .dep { color:var(--red); font-size:11px; margin-left:6px; }
</style>
</head>
<body>
<h1>📚 接口文档</h1>
<div class="sub" id="meta">加载中…</div>
<div id="content"></div>
<script>
fetch("api.json").then(function(res){ return res.json(); }).then(function(spec){
  document.getElementById("meta").textContent = spec.info.title + " · v" + spec.info.version + " · OpenAPI " + spec.openapi;
  var content = document.getElementById("content");
  var groups = {};
  var methods = {get:"GET",post:"POST",put:"PUT",delete:"DELETE"};
  Object.keys(spec.paths).forEach(function(path){
    var item = spec.paths[path];
    Object.keys(methods).forEach(function(key){
      var op = item[methods[key]];
      if (!op) return;
      var tag = (op.tags && op.tags[0]) || "default";
      if (!groups[tag]) groups[tag] = [];
      groups[tag].push({path:path, op:op, method:methods[key]});
    });
  });
  var html = "";
  Object.keys(groups).forEach(function(tag){
    html += '<div class="group"><h2>' + tag + '</h2>';
    groups[tag].forEach(function(entry){
      html += '<div class="op" onclick="this.classList.toggle(\'open\')"><div class="op-head">' +
        '<span class="method m-' + entry.method + '">' + entry.method + '</span>' +
        '<span class="path">' + entry.path + '</span>' +
        '<span class="summary">' + entry.op.summary + (entry.op.deprecated ? '<span class="dep">弃用</span>' : '') + '</span></div>';
      html += '<div class="op-body">';
      if (entry.op.description) html += '<div class="desc">' + entry.op.description + '</div>';
      if (entry.op.parameters && entry.op.parameters.length) {
        html += '<table><tr><th>参数</th><th>位置</th><th>类型</th><th>必填</th><th>说明</th></tr>';
        entry.op.parameters.forEach(function(p){
          html += '<tr><td>' + p.name + '</td><td>' + p.in + '</td><td>' + (p.schema.type || '') + '</td><td>' + (p.required ? '是' : '否') + '</td><td>' + (p.description || '') + '</td></tr>';
        });
        html += '</table>';
      }
      if (entry.op.requestBody) html += '<div class="desc">请求体:' + (entry.op.requestBody.description || '') + '</div>';
      var responses = entry.op.responses || {};
      html += '<table><tr><th>状态码</th><th>说明</th></tr>';
      Object.keys(responses).forEach(function(code){
        html += '<tr><td>' + code + '</td><td>' + (responses[code].description || '') + '</td></tr>';
      });
      html += '</table></div></div>';
    });
    html += '</div>';
  });
  content.innerHTML = html || '<div class="desc">暂无接口文档(使用 apidoc.GET/POST/PUT/DELETE 或 apidoc.CRUD 注册)</div>';
}).catch(function(e){
  document.getElementById("content").innerHTML = '<div class="desc">加载失败:' + e.message + '</div>';
});
</script>
</body>
</html>`
}
