package gencode

// adminPageHTML 低代码生成管理页面(自包含,无外部资源):
// 主区表列表(已生成标记)+ 字段编辑器 + 右侧操作菜单(生成/预览/同步)。
const adminPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>低代码生成平台</title>
<style>
  :root { --bg:#0f1420; --card:#171e2e; --line:#232d42; --text:#d7e0f0; --muted:#7d8aa5;
          --green:#2ecc71; --yellow:#f1c40f; --red:#e74c3c; --blue:#3498db; }
  * { margin:0; padding:0; box-sizing:border-box; }
  body { background:var(--bg); color:var(--text); font-family:"Segoe UI",system-ui,sans-serif; }
  .layout { display:flex; min-height:100vh; }
  .sidebar { width:260px; background:#111827; border-right:1px solid var(--line); padding:16px; flex-shrink:0; }
  .sidebar h1 { font-size:16px; margin-bottom:14px; }
  .sidebar .search { width:100%; background:#0b1019; border:1px solid var(--line); color:var(--text);
                     border-radius:6px; padding:7px 10px; margin-bottom:12px; font-size:13px; }
  .table-item { padding:9px 10px; border-radius:6px; cursor:pointer; font-size:13px;
                display:flex; justify-content:space-between; align-items:center; }
  .table-item:hover { background:#1c2438; }
  .table-item.active { background:rgba(52,152,219,.15); border-left:3px solid var(--blue); }
  .table-item .badge { font-size:11px; padding:1px 8px; border-radius:8px; }
  .badge.gen { background:rgba(46,204,113,.15); color:var(--green); }
  .badge.new { background:rgba(241,196,15,.15); color:var(--yellow); }
  .main { flex:1; padding:20px; }
  .main h2 { font-size:17px; margin-bottom:14px; }
  .toolbar { display:flex; gap:8px; margin-bottom:14px; flex-wrap:wrap; }
  .btn { background:#1c2438; border:1px solid var(--line); color:var(--text); border-radius:6px;
         padding:7px 14px; font-size:13px; cursor:pointer; }
  .btn:hover { background:#232d42; }
  .btn.primary { background:var(--blue); border-color:var(--blue); color:#fff; }
  .btn.danger { color:var(--red); }
  .btn:disabled { opacity:.5; cursor:not-allowed; }
  table { width:100%; border-collapse:collapse; font-size:13px; }
  th { background:#1c2438; color:var(--muted); text-align:left; padding:8px 10px; border:1px solid var(--line); }
  td { padding:6px 10px; border:1px solid var(--line); }
  td input, td select { background:#0b1019; border:1px solid var(--line); color:var(--text);
                        border-radius:4px; padding:4px 6px; width:100%; font-size:12px; }
  .empty { color:var(--muted); text-align:center; padding:40px; }
  .modal { position:fixed; inset:0; background:rgba(0,0,0,.6); display:none; align-items:center; justify-content:center; z-index:10; }
  .modal.show { display:flex; }
  .modal-box { background:var(--card); border:1px solid var(--line); border-radius:10px; padding:20px; max-width:560px; width:90%; max-height:70vh; overflow:auto; }
  .modal-box h3 { margin-bottom:10px; font-size:15px; }
  .modal-box pre { background:#0b1019; border:1px solid var(--line); border-radius:6px; padding:10px;
                   font-size:11px; overflow:auto; max-height:300px; white-space:pre-wrap; }
  .modal-actions { display:flex; justify-content:flex-end; gap:8px; margin-top:14px; }
  .notice { position:fixed; top:16px; right:16px; background:var(--card); border:1px solid var(--line);
            border-radius:8px; padding:10px 16px; font-size:13px; display:none; z-index:20; }
  .notice.show { display:block; }
  .notice.ok { border-color:var(--green); color:var(--green); }
  .notice.err { border-color:var(--red); color:var(--red); }
  .row-add { display:flex; gap:6px; margin-top:10px; }
  .row-add input, .row-add select { background:#0b1019; border:1px solid var(--line); color:var(--text);
                                    border-radius:4px; padding:5px 8px; font-size:12px; }
</style>
</head>
<body>
<div class="layout">
  <div class="sidebar">
    <h1>🛠️ 低代码生成平台</h1>
    <input class="search" id="search" placeholder="搜索表名..." oninput="renderTables()">
    <div id="tableList"></div>
  </div>
  <div class="main">
    <h2 id="tableTitle">选择左侧数据表</h2>
    <div class="toolbar" id="toolbar" style="display:none">
      <button class="btn" onclick="previewCode()">👁 预览代码</button>
      <button class="btn primary" onclick="generateCode(false)">⚡ 生成 DDD 代码</button>
      <button class="btn" onclick="generateCode(true)" id="btnForce" style="display:none">⚠ 覆盖生成</button>
      <button class="btn" onclick="syncTable()">🔄 同步表结构</button>
      <button class="btn" onclick="addColumnRow()">➕ 新增字段</button>
    </div>
    <div id="columnsArea"><div class="empty">从左侧选择一张表,查看并管理字段</div></div>
    <div id="addRowArea" style="display:none"></div>
  </div>
</div>

<div class="modal" id="modal">
  <div class="modal-box">
    <h3 id="modalTitle"></h3>
    <div id="modalBody"></div>
    <div class="modal-actions">
      <button class="btn" onclick="closeModal()">关闭</button>
      <button class="btn primary" id="modalConfirm" style="display:none" onclick="modalAction()">确认</button>
    </div>
  </div>
</div>
<div class="notice" id="notice"></div>

<script>
(function(){
  var current = null;
  var tables = [];
  var pendingAction = null;

  function notice(text, ok) {
    var el = document.getElementById("notice");
    el.textContent = text;
    el.className = "notice show " + (ok ? "ok" : "err");
    setTimeout(function(){ el.className = "notice"; }, 3500);
  }
  function api(path, options) {
    return fetch(path, options).then(function(res){ return res.json().then(function(body){
      if (body.code !== "00000") throw new Error(body.message || ("HTTP " + res.status));
      return body.data;
    }); });
  }

  window.loadTables = function() {
    api("api/tables").then(function(list){
      tables = list;
      renderTables();
    }).catch(function(e){ notice("加载表列表失败: " + e.message, false); });
  };

  window.renderTables = function() {
    var keyword = document.getElementById("search").value.toLowerCase();
    var el = document.getElementById("tableList");
    el.innerHTML = "";
    tables.filter(function(t){ return t.name.toLowerCase().indexOf(keyword) >= 0; }).forEach(function(t){
      var item = document.createElement("div");
      item.className = "table-item" + (current && current.name === t.name ? " active" : "");
      item.innerHTML = '<span>' + t.name + (t.comment ? ' <small style="color:var(--muted)">' + t.comment + '</small>' : '') + '</span>' +
        (t.generated ? '<span class="badge gen">已生成</span>' : '<span class="badge new">未生成</span>');
      item.onclick = function(){ selectTable(t.name); };
      el.appendChild(item);
    });
  };

  window.selectTable = function(name) {
    api("api/tables/" + encodeURIComponent(name)).then(function(table){
      current = table;
      renderTables();
      document.getElementById("tableTitle").textContent = "表 " + table.name + (table.comment ? " · " + table.comment : "");
      document.getElementById("toolbar").style.display = "flex";
      document.getElementById("btnForce").style.display = "none";
      renderColumns();
    }).catch(function(e){ notice("加载表详情失败: " + e.message, false); });
  };

  window.renderColumns = function() {
    if (!current) return;
    var area = document.getElementById("columnsArea");
    var html = '<table><tr><th>字段名</th><th>类型</th><th>长度</th><th>可空</th><th>默认值</th><th>注释</th><th>主键</th><th>操作</th></tr>';
    current.columns.forEach(function(col){
      html += '<tr><td>' + col.name + '</td><td>' + col.data_type + '</td><td>' + (col.length || '') + '</td>' +
        '<td>' + (col.nullable ? '是' : '否') + '</td><td>' + (col.default || '') + '</td>' +
        '<td>' + (col.comment || '') + '</td><td>' + (col.primary_key ? '✅' : '') + '</td>' +
        '<td><button class="btn danger" onclick="dropColumn(\'' + col.name + '\')">删除</button></td></tr>';
    });
    html += '</table>';
    area.innerHTML = html;
    document.getElementById("addRowArea").innerHTML =
      '<div class="row-add">' +
      '<input id="newColName" placeholder="字段名(如 age)">' +
      '<select id="newColType"><option>INTEGER</option><option>TEXT</option><option>VARCHAR</option><option>BIGINT</option><option>REAL</option><option>BOOLEAN</option><option>DATETIME</option></select>' +
      '<input id="newColLen" placeholder="长度" style="width:70px">' +
      '<input id="newColDefault" placeholder="默认值" style="width:90px">' +
      '<input id="newColComment" placeholder="注释" style="flex:1">' +
      '<button class="btn primary" onclick="submitAddColumn()">添加</button></div>';
  };

  window.addColumnRow = function() {
    document.getElementById("addRowArea").style.display = "block";
    document.getElementById("addRowArea").scrollIntoView({behavior:"smooth"});
  };

  window.submitAddColumn = function() {
    var name = document.getElementById("newColName").value.trim();
    if (!name) { notice("字段名必填", false); return; }
    var column = {
      name: name,
      data_type: document.getElementById("newColType").value,
      length: parseInt(document.getElementById("newColLen").value) || 0,
      default: document.getElementById("newColDefault").value,
      comment: document.getElementById("newColComment").value,
      nullable: true
    };
    api("api/tables/" + encodeURIComponent(current.name) + "/columns", {
      method: "POST",
      headers: {"Content-Type": "application/json"},
      body: JSON.stringify(column)
    }).then(function(){
      notice("字段 " + name + " 已添加", true);
      return selectTable(current.name);
    }).catch(function(e){ notice("添加失败: " + e.message, false); });
  };

  window.dropColumn = function(name) {
    if (!confirm("确认删除字段 " + name + " ?此操作会修改表结构")) return;
    api("api/tables/" + encodeURIComponent(current.name) + "/columns/" + encodeURIComponent(name), {
      method: "DELETE"
    }).then(function(){
      notice("字段 " + name + " 已删除", true);
      return selectTable(current.name);
    }).catch(function(e){ notice("删除失败: " + e.message, false); });
  };

  window.syncTable = function() {
    api("api/tables/" + encodeURIComponent(current.name) + "/sync", {method: "POST"}).then(function(table){
      current = table;
      renderColumns();
      notice("表结构已同步(" + table.columns.length + " 个字段)", true);
    }).catch(function(e){ notice("同步失败: " + e.message, false); });
  };

  window.previewCode = function() {
    api("api/tables/" + encodeURIComponent(current.name) + "/preview").then(function(result){
      var html = "<p>共 " + result.files.length + " 个文件</p>";
      result.files.forEach(function(file){
        html += '<h4 style="margin-top:10px;color:var(--blue)">' + file.path + '</h4><pre>' + escapeHtml(file.content) + '</pre>';
      });
      openModal("代码预览 - " + current.name, html);
    }).catch(function(e){ notice("预览失败: " + e.message, false); });
  };

  window.generateCode = function(force) {
    api("api/tables/" + encodeURIComponent(current.name) + "/generate?force=" + force, {method: "POST"}).then(function(result){
      if (result.need_confirm) {
        var html = '<p style="color:var(--yellow)">⚠️ 该表已生成过代码,再次生成将覆盖以下文件:</p><pre>' +
          result.overwritten.join("\n") + '</pre>';
        pendingAction = function(){ generateCode(true); };
        openModal("覆盖确认", html, "确认覆盖");
        return;
      }
      var html = '<p>✅ 已生成 ' + result.files + ' 个文件' +
        (result.overwritten && result.overwritten.length ? '，覆盖 ' + result.overwritten.length + ' 个' : '') + '</p>' +
        '<h4 style="margin-top:10px;color:var(--blue)">路由注册代码(复制到 main.go):</h4><pre>' + escapeHtml(result.route_code) + '</pre>';
      openModal("生成完成", html);
      document.getElementById("btnForce").style.display = "none";
      loadTables();
    }).catch(function(e){ notice("生成失败: " + e.message, false); });
  };

  function escapeHtml(text) {
    var div = document.createElement("div");
    div.textContent = text;
    return div.innerHTML;
  }

  window.openModal = function(title, body, confirmText) {
    document.getElementById("modalTitle").textContent = title;
    document.getElementById("modalBody").innerHTML = body;
    var confirm = document.getElementById("modalConfirm");
    confirm.style.display = confirmText ? "inline-block" : "none";
    confirm.textContent = confirmText || "确认";
    document.getElementById("modal").className = "modal show";
  };
  window.closeModal = function() {
    document.getElementById("modal").className = "modal";
    pendingAction = null;
  };
  window.modalAction = function() {
    if (pendingAction) { var action = pendingAction; pendingAction = null; action(); }
    closeModal();
  };

  loadTables();
})();
</script>
</body>
</html>`
