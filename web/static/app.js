(function () {
  const $ = (sel) => document.querySelector(sel);
  const loginPanel = $("#login-panel");
  const mainPanel = $("#main-panel");
  const content = $("#content");
  const nav = $("#nav");
  const userLabel = $("#user-label");

  const THEME_STORAGE_KEY = "gfep-theme";

  function applyTheme(light) {
    const root = document.documentElement;
    if (light) {
      root.setAttribute("data-theme", "light");
    } else {
      root.removeAttribute("data-theme");
    }
    try {
      localStorage.setItem(THEME_STORAGE_KEY, light ? "light" : "dark");
    } catch (_) {}
    document.querySelectorAll(".btn-theme").forEach((btn) => {
      btn.setAttribute("aria-pressed", light ? "true" : "false");
      btn.title = light ? "切换为深色主题" : "切换为浅色主题";
      btn.textContent = light ? "\u263e" : "\u2600";
    });
  }

  function initTheme() {
    let light = false;
    try {
      light = localStorage.getItem(THEME_STORAGE_KEY) === "light";
    } catch (_) {}
    applyTheme(light);
  }

  initTheme();
  document.querySelectorAll(".btn-theme").forEach((btn) => {
    btn.addEventListener("click", () => {
      const isLight = document.documentElement.getAttribute("data-theme") === "light";
      applyTheme(!isLight);
    });
  });

  let me = null;
  let liveES = null;
  let overviewTimer = null;

  async function api(path, opts = {}) {
    const { headers: optHeaders, ...rest } = opts || {};
    const headers = { ...(optHeaders || {}) };
    if (
      rest.body != null &&
      headers["Content-Type"] === undefined &&
      headers["content-type"] === undefined
    ) {
      headers["Content-Type"] = "application/json";
    }
    const r = await fetch(path, {
      credentials: "same-origin",
      ...rest,
      headers,
    });
    const text = await r.text();
    let data = null;
    try {
      data = text ? JSON.parse(text) : null;
    } catch {
      data = { error: text || r.statusText };
    }
    if (!r.ok) throw new Error(data.error || r.statusText);
    return data;
  }

  function showLogin() {
    mainPanel.classList.add("hidden");
    loginPanel.classList.remove("hidden");
    me = null;
    if (liveES) {
      liveES.close();
      liveES = null;
    }
  }

  function showMain() {
    loginPanel.classList.add("hidden");
    mainPanel.classList.remove("hidden");
  }

  async function refreshMe() {
    try {
      me = await api("/api/auth/me");
      userLabel.textContent = me.username + " (" + me.role + ")";
      buildNav();
      return true;
    } catch {
      me = null;
      showLogin();
      return false;
    }
  }

  function buildNav() {
    const tabs = [
      { id: "overview", label: "总览" },
      { id: "terminals", label: "在线终端" },
      { id: "apps", label: "主站/APP" },
      { id: "live", label: "实时日志" },
      { id: "files", label: "历史日志" },
      { id: "config", label: "配置" },
    ];
    if (me && me.role === "admin") {
      tabs.push({ id: "users", label: "用户" });
      tabs.push({ id: "blacklist", label: "黑名单" });
      tabs.push({ id: "loglevel", label: "日志级别" });
    }
    nav.innerHTML = "";
    let first = true;
    for (const t of tabs) {
      const b = document.createElement("button");
      b.textContent = t.label;
      b.dataset.tab = t.id;
      if (first) {
        b.classList.add("active");
        first = false;
      }
      b.addEventListener("click", () => selectTab(t.id));
      nav.appendChild(b);
    }
    const active = nav.querySelector(".active");
    if (active) renderView(active.dataset.tab);
    else selectTab("overview");
  }

  function selectTab(id) {
    nav.querySelectorAll("button").forEach((b) => b.classList.toggle("active", b.dataset.tab === id));
    renderView(id);
  }

  async function renderView(id) {
    if (liveES) {
      liveES.close();
      liveES = null;
    }
    if (overviewTimer) {
      clearInterval(overviewTimer);
      overviewTimer = null;
    }
    content.innerHTML = '<p class="muted">加载中…</p>';
    try {
      if (id === "overview") await viewOverview();
      else if (id === "terminals") await viewTerminals();
      else if (id === "apps") await viewApps();
      else if (id === "live") viewLive();
      else if (id === "files") await viewFiles();
      else if (id === "config") await viewConfig();
      else if (id === "users") await viewUsers();
      else if (id === "blacklist") await viewBlacklist();
      else if (id === "loglevel") await viewLogLevel();
      else content.innerHTML = "<p>未知页面</p>";
    } catch (e) {
      content.innerHTML = '<p class="err">' + escapeHtml(e.message) + "</p>";
    }
  }

  function escapeHtml(s) {
    return String(s)
      .replace(/&/g, "&amp;")
      .replace(/</g, "&lt;")
      .replace(/>/g, "&gt;")
      .replace(/"/g, "&quot;");
  }

  function ovPct(x) {
    if (x == null || Number.isNaN(Number(x))) return null;
    return Math.min(100, Math.max(0, Number(x)));
  }

  function ovMeter(title, pct, hintHtml) {
    const p = ovPct(pct);
    const label = p == null ? "—" : p.toFixed(1) + "%";
    const w = p == null ? 0 : p;
    let fillMod = "";
    if (p != null) {
      if (p >= 90) fillMod = " is-high";
      else if (p >= 75) fillMod = " is-warn";
    }
    const hint = hintHtml ? '<p class="ov-meter-hint muted">' + hintHtml + "</p>" : "";
    return (
      '<div class="ov-meter">' +
      '<div class="ov-meter-top">' +
      '<span class="ov-meter-t">' +
      escapeHtml(title) +
      '</span><span class="ov-meter-num">' +
      label +
      "</span></div>" +
      '<div class="ov-meter-bar" role="img" aria-label="' +
      escapeHtml(title + " " + label) +
      '"><span class="ov-meter-fill' +
      fillMod +
      '" style="width:' +
      w +
      '%"></span></div>' +
      hint +
      "</div>"
    );
  }

  function ovHeapBar(label, mibs, maxMib) {
    const v = mibs != null && !Number.isNaN(Number(mibs)) ? Number(mibs) : 0;
    const cap = maxMib > 0 ? maxMib : 1;
    const w = Math.min(100, (v / cap) * 100);
    return (
      '<div class="ov-meter ov-meter-compact">' +
      '<div class="ov-meter-top">' +
      '<span class="ov-meter-t">' +
      escapeHtml(label) +
      '</span><span class="ov-meter-num">' +
      v.toFixed(2) +
      " MiB</span></div>" +
      '<div class="ov-meter-bar ov-meter-bar-heap"><span class="ov-meter-fill ov-fill-heap" style="width:' +
      w +
      '%"></span></div></div>'
    );
  }

  async function viewOverview() {
    const mount = document.createElement("div");
    mount.id = "overview-root";
    content.innerHTML = "";
    content.appendChild(mount);
    const paint = async () => {
      try {
        const s = await api("/api/status");
        const h = s.host || {};
        const g = h.goRuntime || {};
        const byP = s.terminalsByProtocol || {};
        const byApp = s.appsByProtocol || {};
        const sortProtoKeys = (o) => Object.keys(o).sort((a, b) => a.localeCompare(b));
        const protoRows = sortProtoKeys(byP)
          .map((k) => "<tr><td>" + escapeHtml(k) + "</td><td>" + byP[k] + "</td></tr>")
          .join("");
        const appRows = sortProtoKeys(byApp)
          .map((k) => "<tr><td>" + escapeHtml(k) + "</td><td>" + byApp[k] + "</td></tr>")
          .join("");
        const fmtMiB = (x) => (x != null && !Number.isNaN(x) ? Number(x).toFixed(2) : "—");
        const fmtPct01 = (x) =>
          x != null && !Number.isNaN(x) ? (Number(x) * 100).toFixed(3) + "%" : "—";
        const diskRel = h.diskPath || ".";
        const ha = g.heapAllocMiB,
          hi = g.heapInuseMiB,
          hs = g.heapSysMiB,
          sy = g.sysMiB;
        const maxHeap = Math.max(1, Number(ha) || 0, Number(hi) || 0, Number(hs) || 0, Number(sy) || 0);
        mount.innerHTML =
          '<div class="card overview-page"><h2>主机（OS）</h2><p class="muted">约每 3 秒自动刷新 · gopsutil 采样</p>' +
          '<div class="ov-meter-grid">' +
          ovMeter("CPU 占用", h.cpuPercent, "瞬时采样") +
          ovMeter("内存占用", h.memUsedPercent, "整机物理内存已用比例") +
          ovMeter(
            "磁盘占用",
            h.diskUsedPercent,
            "日志目录所在分区 · <code class=\"ov-path\">" + escapeHtml(diskRel) + "</code>"
          ) +
          "</div>" +
          '<div class="ov-tcp"><span class="muted">TCP 业务连接</span> <strong class="ov-tcp-n">' +
          (s.tcpConnTotal ?? "—") +
          "</strong></div>" +
          '<h3 class="section-title">Go 堆与运行时内存</h3><p class="muted">条形长度相对下列四项中的最大值；与主机「内存占用%」含义不同</p>' +
          '<div class="ov-meter-grid ov-meter-grid--tight">' +
          ovHeapBar("HeapAlloc", ha, maxHeap) +
          ovHeapBar("HeapInuse", hi, maxHeap) +
          ovHeapBar("HeapSys", hs, maxHeap) +
          ovHeapBar("Sys 总计", sy, maxHeap) +
          "</div>" +
          '<h3 class="section-title">其他运行时指标</h3><p class="muted">GC CPU 占比为进程启动以来累计，适合长期观察</p><div class="grid2">' +
          '<div class="stat"><div class="k">协程数</div><div class="v">' +
          (g.goroutines != null ? g.goroutines : "—") +
          "</div></div>" +
          '<div class="stat"><div class="k">栈使用 StackInuse</div><div class="v">' +
          fmtMiB(g.stackInuseMiB) +
          " MiB</div></div>" +
          '<div class="stat"><div class="k">累计 GC 次数</div><div class="v">' +
          (g.numGC != null ? g.numGC : "—") +
          "</div></div>" +
          '<div class="stat"><div class="k">上次 GC 暂停</div><div class="v">' +
          (g.lastGCPauseMs != null ? g.lastGCPauseMs.toFixed(3) : "—") +
          " ms</div></div>" +
          '<div class="stat"><div class="k">GC CPU 占比</div><div class="v">' +
          fmtPct01(g.gcCPUFraction) +
          "</div></div>" +
          '<div class="stat"><div class="k">下次 GC 目标 NextGC</div><div class="v">' +
          fmtMiB(g.nextGCMiB) +
          " MiB</div></div>" +
          '<div class="stat"><div class="k">MSpan Inuse</div><div class="v">' +
          fmtMiB(g.mspanInuseMiB) +
          " MiB</div></div>" +
          '<div class="stat"><div class="k">MSpan Sys</div><div class="v">' +
          fmtMiB(g.mspanSysMiB) +
          " MiB</div></div>" +
          '<div class="stat"><div class="k">MCache Inuse</div><div class="v">' +
          fmtMiB(g.mcacheInuseMiB) +
          " MiB</div></div>" +
          '<div class="stat"><div class="k">MCache Sys</div><div class="v">' +
          fmtMiB(g.mcacheSysMiB) +
          " MiB</div></div>" +
          "</div>" +
          "<p class=\"muted\">版本 " +
          escapeHtml(s.version || "") +
          " · Worker池 " +
          (s.workerPoolSize ?? "") +
          " · 队列 " +
          (s.maxWorkerTaskLen ?? "") +
          '</p><p class="muted">快捷：<button type="button" class="linkish" data-go="terminals">在线终端</button> · <button type="button" class="linkish" data-go="apps">主站/APP</button></p>' +
          '<h3 class="section-title">在线终端 · 按协议（去重）</h3>' +
          '<div class="table-wrap"><table class="data"><thead><tr><th>协议</th><th>在线终端(去重)</th></tr></thead><tbody>' +
          (protoRows || '<tr><td colspan="2" class="empty">无数据</td></tr>') +
          '</tbody></table></div>' +
          '<h3 class="section-title">主站 / APP · 按协议</h3>' +
          '<div class="table-wrap"><table class="data"><thead><tr><th>协议</th><th>连接数</th></tr></thead><tbody>' +
          (appRows || '<tr><td colspan="2" class="empty">无数据</td></tr>') +
          "</tbody></table></div></div>";
        mount.querySelectorAll("button[data-go]").forEach((btn) => {
          btn.addEventListener("click", () => selectTab(btn.getAttribute("data-go")));
        });
      } catch (e) {
        mount.innerHTML = '<p class="err">' + escapeHtml(e.message) + "</p>";
      }
    };
    await paint();
    overviewTimer = setInterval(paint, 3000);
  }

  async function viewTerminals() {
    let tPage = 1;
    content.innerHTML =
      '<div class="card"><h2>在线终端</h2><div class="toolbar">' +
      '<input type="search" id="tq" placeholder="addr / IP 过滤（回车查询）" />' +
      '<select id="tp"><option value="">全部协议</option><option>376.1</option><option>698.45</option><option>NW</option></select>' +
      '<label class="cb"><input type="checkbox" id="tex" /> 展开同址多连接</label>' +
      '<label class="cb"><input type="checkbox" id="tshowdur" /> 显示在线时长</label>' +
      '<span class="toolbar-sep" aria-hidden="true"></span>' +
      '<label class="inline muted">排序</label>' +
      '<select id="tsort"><option value="login">登录时间</option><option value="addr">终端地址</option></select>' +
      '<select id="torder"><option value="desc">降序</option><option value="asc">升序</option></select>' +
      '<label class="inline muted">每页</label>' +
      '<select id="tpsize"><option value="10">10</option><option value="20" selected>20</option><option value="50">50</option><option value="100">100</option></select>' +
      '<button class="primary" id="tref">刷新</button></div>' +
      '<div id="ttable"></div>' +
      '<div class="pager toolbar" id="tpnav" hidden>' +
      '<button type="button" id="tprev">上一页</button>' +
      '<span id="tpinfo" class="muted"></span>' +
      '<button type="button" id="tnext">下一页</button></div></div>';

    /** @type {{ rows: any[], total: number, page: number, pageSize: number } | null} */
    let termSnap = null;

    const paintTerminals = () => {
      const pnav = $("#tpnav");
      if (!termSnap) {
        pnav.hidden = true;
        $("#ttable").innerHTML = "";
        return;
      }
      const { rows, total, page, pageSize } = termSnap;
      const maxPage = total === 0 ? 1 : Math.ceil(total / pageSize);
      if (total === 0) {
        pnav.hidden = true;
        $("#ttable").innerHTML = '<p class="empty">暂无终端连接</p>';
        return;
      }
      pnav.hidden = false;
      $("#tpinfo").textContent = "第 " + page + " / " + maxPage + " 页（共 " + total + " 条）";
      $("#tprev").disabled = page <= 1;
      $("#tnext").disabled = page >= maxPage;
      const showDur = $("#tshowdur").checked;
      const durTh = showDur ? "<th>在线时长</th>" : "";
      const cols =
        "<th>#</th><th>connId</th><th>IP:port</th><th>协议</th><th>addr</th>" +
        durTh +
        "<th>登录</th><th>心跳</th><th>最近收</th><th>最近发</th><th>上报</th><th>上行帧/字节</th><th>下行次/字节</th><th></th>";
      const base = (page - 1) * pageSize;
      let i = base;
      const body = rows
        .map((r) => {
          i++;
          const dur = r.onlineDuration ? String(r.onlineDuration) : "";
          const durCell = showDur ? "<td>" + escapeHtml(dur || "—") + "</td>" : "";
          return (
            "<tr><td>" +
            i +
            "</td><td>" +
            r.connId +
            "</td><td>" +
            escapeHtml(r.remoteTcp) +
            "</td><td>" +
            escapeHtml(r.protocol) +
            "</td><td>" +
            escapeHtml(r.addr) +
            "</td>" +
            durCell +
            "<td>" +
            (r.loginTime || "—") +
            "</td><td>" +
            (r.heartbeatTime || "—") +
            "</td><td>" +
            (r.lastRxTime || "—") +
            "</td><td>" +
            (r.lastTxTime || "—") +
            "</td><td>" +
            (r.lastReportTime || "—") +
            "</td><td>" +
            r.uplinkMsgCount +
            " / " +
            escapeHtml(r.uplinkBytes) +
            "</td><td>" +
            r.downlinkMsgCount +
            " / " +
            escapeHtml(r.downlinkBytes) +
            "</td><td>" +
            '<button type="button" class="t-kick" data-conn="' +
            r.connId +
            '">踢</button></td></tr>'
          );
        })
        .join("");
      $("#ttable").innerHTML =
        '<div class="table-wrap"><table class="data"><thead><tr>' + cols + "</tr></thead><tbody>" + body + "</tbody></table></div>";
    };

    const run = async () => {
      const q = new URLSearchParams();
      const f = $("#tq").value.trim();
      if (f) q.set("q", f);
      const p = $("#tp").value;
      if (p) q.set("protocol", p);
      if ($("#tex").checked) q.set("expand", "1");
      q.set("sort", $("#tsort").value);
      q.set("order", $("#torder").value);
      q.set("page", String(tPage));
      q.set("pageSize", $("#tpsize").value);
      const data = await api("/api/terminals?" + q.toString());
      const rows = data.rows || [];
      const total = data.total != null ? data.total : rows.length;
      const page = data.page != null ? data.page : tPage;
      const pageSize = data.pageSize != null ? data.pageSize : Number($("#tpsize").value) || 20;
      tPage = page;
      termSnap = { rows, total, page, pageSize };
      paintTerminals();
    };

    const runSafe = () => run().catch((e) => alert(e.message));

    $("#tref").addEventListener("click", runSafe);
    $("#tsort").addEventListener("change", () => {
      tPage = 1;
      runSafe();
    });
    $("#torder").addEventListener("change", () => {
      tPage = 1;
      runSafe();
    });
    $("#tpsize").addEventListener("change", () => {
      tPage = 1;
      runSafe();
    });
    $("#tp").addEventListener("change", () => {
      tPage = 1;
      runSafe();
    });
    $("#tex").addEventListener("change", () => {
      tPage = 1;
      runSafe();
    });
    $("#tshowdur").addEventListener("change", () => paintTerminals());
    $("#tq").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") {
        tPage = 1;
        runSafe();
      }
    });
    $("#tprev").addEventListener("click", () => {
      if (tPage > 1) {
        tPage--;
        runSafe();
      }
    });
    $("#tnext").addEventListener("click", () => {
      tPage++;
      runSafe();
    });
    $("#ttable").addEventListener("click", async (ev) => {
      const btn = ev.target.closest(".t-kick");
      if (!btn) return;
      const id = Number(btn.getAttribute("data-conn"), 10);
      if (!id || !window.confirm("确定踢掉该终端连接（关闭 TCP）？")) return;
      try {
        await api("/api/terminals/kick", { method: "POST", body: JSON.stringify({ connId: id }) });
        await run();
      } catch (e) {
        alert(e.message);
      }
    });
    await run();
  }

  async function viewApps() {
    content.innerHTML =
      '<div class="card"><h2>主站 / APP 连接</h2><p class="muted">' +
      "上行 = 主站→FEP 帧数/字节，下行 = FEP→主站（与终端表视角相反）</p>" +
      '<div class="toolbar"><input type="search" id="aq" placeholder="MSA / IP 过滤" /><button class="primary" id="aref">刷新</button></div><div id="atable"></div></div>';
    const run = async () => {
      const q = $("#aq").value.trim();
      const data = await api("/api/apps" + (q ? "?q=" + encodeURIComponent(q) : ""));
      const rows = data.rows || [];
      if (!rows.length) {
        $("#atable").innerHTML = '<p class="empty">当前无此类连接或未启用相关功能</p>';
        return;
      }
      const cols =
        "<th>#</th><th>connId</th><th>IP:port</th><th>协议</th><th>主站摘要</th><th>连接</th><th>在线时长</th><th>最近收</th><th>最近发</th><th>上报</th><th>上行帧/字节</th><th>下行次/字节</th>";
      let i = 0;
      const body = rows
        .map((r) => {
          i++;
          return (
            "<tr><td>" +
            i +
            "</td><td>" +
            r.connId +
            "</td><td>" +
            escapeHtml(r.remoteTcp) +
            "</td><td>" +
            escapeHtml(r.protocol) +
            "</td><td>" +
            escapeHtml(r.masterSummary) +
            "</td><td>" +
            (r.connTime || "—") +
            "</td><td>" +
            escapeHtml(r.onlineDuration || "—") +
            "</td><td>" +
            (r.lastRxTime || "—") +
            "</td><td>" +
            (r.lastTxTime || "—") +
            "</td><td>" +
            (r.lastReportTime || "—") +
            "</td><td>" +
            r.uplinkMsgCount +
            " / " +
            r.uplinkBytes +
            "</td><td>" +
            r.downlinkMsgCount +
            " / " +
            r.downlinkBytes +
            "</td></tr>"
          );
        })
        .join("");
      $("#atable").innerHTML =
        '<div class="table-wrap"><table class="data"><thead><tr>' + cols + "</tr></thead><tbody>" + body + "</tbody></table></div>";
    };
    $("#aref").addEventListener("click", () => run().catch((e) => alert(e.message)));
    await run();
  }

  function viewLive() {
    content.innerHTML =
      '<div class="card card-live"><h2>实时通信日志</h2><p class="muted">依赖 LogPacketHex / LogLinkLayer 等开关；SSE 推送。报文行可按 <strong>协议</strong>、<strong>终端地址 addr</strong> 或 <strong>对端 IP:port</strong> 过滤。</p>' +
      '<div class="toolbar">' +
      '<label class="inline"><span class="muted">协议</span> <select id="lfp"><option value="">全部</option><option>376.1</option><option>698.45</option><option>NW</option><option>376-主站</option><option>698-主站</option><option>Nw-主站</option></select></label>' +
      '<input type="search" id="lf" placeholder="addr 或 IP:port（子串）" />' +
      '<button type="button" class="primary" id="lapply">应用过滤</button>' +
      '<button type="button" id="lclr">清空</button></div><div class="log-view" id="logbox"></div></div>';
    const box = $("#logbox");
    const streamURL = () => {
      const q = new URLSearchParams();
      const a = $("#lf").value.trim();
      if (a) q.set("addr", a);
      const p = $("#lfp").value.trim();
      if (p) q.set("protocol", p);
      const qs = q.toString();
      return "/api/logs/stream" + (qs ? "?" + qs : "");
    };
    const connect = () => {
      if (liveES) {
        liveES.close();
        liveES = null;
      }
      liveES = new EventSource(streamURL(), { withCredentials: true });
      liveES.onmessage = (ev) => {
        try {
          const o = JSON.parse(ev.data);
          box.textContent += (o.ts || "") + " " + (o.line || "") + "\n";
          if (box.scrollHeight < box.scrollTop + box.clientHeight + 800) box.scrollTop = box.scrollHeight;
        } catch {
          box.textContent += ev.data + "\n";
        }
      };
      liveES.onerror = () => {
        box.textContent += "\n[连接中断，请切换页面重试]\n";
      };
    };
    connect();
    $("#lapply").addEventListener("click", () => connect());
    $("#lclr").addEventListener("click", () => {
      box.textContent = "";
    });
  }

  async function viewFiles() {
    const data = await api("/api/logs/files");
    const files = (data.files || []).filter((f) => !f.isDir);
    let body = "";
    for (const f of files) {
      const href = "/api/logs/download?name=" + encodeURIComponent(f.name);
      body +=
        "<tr><td>" +
        escapeHtml(f.name) +
        "</td><td>" +
        escapeHtml(f.sizeHuman != null && f.sizeHuman !== "" ? f.sizeHuman : String(f.size)) +
        "</td><td>" +
        escapeHtml(f.modTime) +
        '</td><td><a href="' +
        href +
        '">下载</a></td></tr>';
    }
    content.innerHTML =
      '<div class="card"><h2>历史日志</h2><p class="muted">' +
      escapeHtml(data.root || "") +
      '</p><div class="table-wrap"><table class="data"><thead><tr><th>文件</th><th>大小</th><th>修改时间</th><th></th></tr></thead><tbody>' +
      (body || '<tr><td colspan=4 class="empty">无文件</td></tr>') +
      "</tbody></table></div></div>";
  }

  async function viewUsers() {
    const data = await api("/api/users");
    const users = data.users || [];
    const rowHtml = users
      .map((u) => {
        const enc = encodeURIComponent(u.username);
        const disp = escapeHtml(u.username);
        const sel =
          '<select class="role-sel"><option value="admin"' +
          (u.role === "admin" ? " selected" : "") +
          '>admin</option><option value="user"' +
          (u.role === "user" ? " selected" : "") +
          ">user</option></select>";
        return (
          '<tr data-user="' +
          enc +
          '"><td class="u-name">' +
          disp +
          "</td><td>" +
          sel +
          '</td><td><button type="button" class="u-role-save">保存角色</button> ' +
          '<button type="button" class="u-pw">改密</button> ' +
          '<button type="button" class="u-del danger">删除</button></td></tr>'
        );
      })
      .join("");
    content.innerHTML =
      '<div class="card"><h2>控制台用户</h2><p class="muted">用户与密码保存在 conf/web_users.json；密码 bcrypt 存储。</p>' +
      '<div class="table-wrap"><table class="data"><thead><tr><th>用户名</th><th>角色</th><th>操作</th></tr></thead><tbody>' +
      (rowHtml || '<tr><td colspan="3" class="empty">暂无用户</td></tr>') +
      "</tbody></table></div>" +
      '<h3 class="muted" style="margin-top:1rem;font-size:0.95rem">新增用户</h3>' +
      '<div class="toolbar" style="margin-top:0.35rem">' +
      '<input type="text" id="nu-name" placeholder="用户名" />' +
      '<input type="password" id="nu-pw" placeholder="密码（≥6位）" />' +
      '<select id="nu-role"><option value="user">user</option><option value="admin">admin</option></select>' +
      '<button type="button" class="primary" id="nu-add">添加</button></div>' +
      '<p id="nu-msg" class="muted"></p></div>';

    const runList = async () => {
      await renderView("users");
    };

    content.querySelectorAll(".u-role-save").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const tr = btn.closest("tr");
        const name = decodeURIComponent(tr.getAttribute("data-user") || "");
        const role = tr.querySelector(".role-sel").value;
        const msg = $("#nu-msg");
        try {
          await api("/api/users", { method: "PUT", body: JSON.stringify({ username: name, role }) });
          msg.textContent = "已更新 " + name;
          await runList();
        } catch (e) {
          msg.textContent = e.message;
        }
      });
    });
    content.querySelectorAll(".u-pw").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const tr = btn.closest("tr");
        const name = decodeURIComponent(tr.getAttribute("data-user") || "");
        const pw = window.prompt("新密码（至少 6 位）", "");
        if (pw == null) return;
        const msg = $("#nu-msg");
        try {
          await api("/api/users", { method: "PUT", body: JSON.stringify({ username: name, password: pw }) });
          msg.textContent = "已修改 " + name + " 的密码";
        } catch (e) {
          msg.textContent = e.message;
        }
      });
    });
    content.querySelectorAll(".u-del").forEach((btn) => {
      btn.addEventListener("click", async () => {
        const tr = btn.closest("tr");
        const name = decodeURIComponent(tr.getAttribute("data-user") || "");
        if (!window.confirm("确定删除用户 " + name + "？")) return;
        const msg = $("#nu-msg");
        try {
          await api("/api/users?username=" + encodeURIComponent(name), { method: "DELETE" });
          msg.textContent = "已删除";
          await runList();
        } catch (e) {
          msg.textContent = e.message;
        }
      });
    });
    $("#nu-add").addEventListener("click", async () => {
      const name = $("#nu-name").value.trim();
      const password = $("#nu-pw").value;
      const role = $("#nu-role").value;
      const msg = $("#nu-msg");
      msg.textContent = "";
      try {
        await api("/api/users", { method: "POST", body: JSON.stringify({ username: name, password, role }) });
        msg.textContent = "已添加";
        await runList();
      } catch (e) {
        msg.textContent = e.message;
      }
    });
  }

  async function viewConfig() {
    const data = await api("/api/config");
    const e = data.effective || {};
    const keys = Object.keys(e).sort();
    let html =
      '<div class="card"><h2>当前有效配置</h2><p class="muted">普通用户只读。含 password/secret/token 的键及非空 BridgeHost698 等已脱敏显示。</p><div class="table-wrap"><table class="data">';
    for (const k of keys) {
      let v = e[k];
      if (typeof v === "object") v = JSON.stringify(v);
      html += "<tr><th>" + escapeHtml(k) + "</th><td>" + escapeHtml(String(v)) + "</td></tr>";
    }
    html += "</table></div></div>";
    if (me && me.role === "admin") {
      html +=
        '<div class="card"><h2>管理员：快捷开关</h2><p class="muted">写入 conf 并 Reload；请谨慎操作</p>' +
        '<div id="cfgform"></div><button class="primary" id="cfgsave">保存可写项</button><p id="cfgmsg" class="muted"></p></div>';
    }
    content.innerHTML = html;
    if (me && me.role === "admin") {
      const boolKeys = [
        "LogPacketHex",
        "LogLinkLayer",
        "LogForwardEgressHex",
        "LogDebugClose",
        "LogConnTrace",
        "LogNetVerbose",
      ];
      const numKeys = ["Timeout", "FirstFrameTimeoutMin", "PostLoginRxIdleMinutes", "ForwardWorkers", "ForwardQueueLen"];
      const form = $("#cfgform");
      for (const k of boolKeys) {
        const id = "cfg_" + k;
        const row = document.createElement("label");
        row.className = "cb";
        row.innerHTML = '<input type="checkbox" id="' + id + '" data-k="' + k + '" /> ' + k;
        form.appendChild(row);
        const inp = $("#" + id);
        inp.checked = !!e[k];
      }
      for (const k of numKeys) {
        const id = "cfg_" + k;
        const row = document.createElement("div");
        row.innerHTML =
          '<label>' +
          k +
          ' <input type="number" id="' +
          id +
          '" data-k="' +
          k +
          '" value="' +
          escapeHtml(String(e[k] ?? "")) +
          '" /></label>';
        form.appendChild(row);
      }
      $("#cfgsave").addEventListener("click", async () => {
        const patch = {};
        form.querySelectorAll('input[type="checkbox"]').forEach((inp) => {
          patch[inp.dataset.k] = inp.checked;
        });
        form.querySelectorAll('input[type="number"]').forEach((inp) => {
          const n = parseInt(inp.value, 10);
          if (!Number.isNaN(n)) patch[inp.dataset.k] = n;
        });
        try {
          await api("/api/config", { method: "PUT", body: JSON.stringify(patch) });
          $("#cfgmsg").textContent = "已保存";
        } catch (err) {
          $("#cfgmsg").textContent = err.message;
        }
      });
    }
  }

  async function viewBlacklist() {
    const data = await api("/api/blacklist");
    const addrs = (data.addrs || []).join("\n");
    content.innerHTML =
      '<div class="card danger-zone"><h2>终端地址黑名单</h2><p class="muted">一行一个地址；保存后拒绝新登录（LINK_LOGIN）</p>' +
      '<textarea id="blta" class="bl-textarea" rows="14">' +
      escapeHtml(addrs) +
      '</textarea><p><button class="primary" id="blsave">保存</button> <span id="blmsg" class="muted"></span></p></div>';
    $("#blsave").addEventListener("click", async () => {
      if (!window.confirm("确定保存黑名单？将影响后续终端登录。")) return;
      const lines = $("#blta")
        .value.split(/\r?\n/)
        .map((s) => s.trim())
        .filter(Boolean);
      try {
        await api("/api/blacklist", { method: "PUT", body: JSON.stringify({ addrs: lines }) });
        $("#blmsg").textContent = "已保存";
      } catch (e) {
        $("#blmsg").textContent = e.message;
      }
    });
  }

  async function viewLogLevel() {
    const data = await api("/api/log-level");
    content.innerHTML =
      '<div class="card"><h2>全局 Debug 日志</h2><p class="muted">' +
      escapeHtml(data.hint || "") +
      '</p><label class="cb"><input type="checkbox" id="ldc" /> LogDebugClose（关闭 Debug）</label>' +
      '<p><button class="primary" id="lsave">应用</button> <span id="lmsg" class="muted"></span></p></div>';
    $("#ldc").checked = !!data.logDebugClose;
    $("#lsave").addEventListener("click", async () => {
      try {
        await api("/api/log-level", {
          method: "PUT",
          body: JSON.stringify({ logDebugClose: $("#ldc").checked }),
        });
        $("#lmsg").textContent = "已更新";
      } catch (e) {
        $("#lmsg").textContent = e.message;
      }
    });
  }

  const loginFormEl = document.getElementById("login-form");
  const loginSubmitBtn = document.getElementById("login-submit");
  if (loginFormEl) {
    loginFormEl.addEventListener("submit", async (ev) => {
      ev.preventDefault();
      const form = loginFormEl;
      const fd = new FormData(form);
      const username = String(fd.get("username") || "").trim();
      const password = String(fd.get("password") || "");
      const errEl = document.getElementById("login-err");
      errEl.textContent = "";
      if (loginSubmitBtn) {
        loginSubmitBtn.disabled = true;
        loginSubmitBtn.textContent = "登录中…";
      }
      try {
        await api("/api/auth/login", { method: "POST", body: JSON.stringify({ username, password }) });
        const ok = await refreshMe();
        if (!ok) {
          errEl.textContent = "登录接口成功，但会话校验失败（请检查浏览器是否拦截 Cookie，或强制刷新后重试）";
          return;
        }
        showMain();
      } catch (e) {
        errEl.textContent = e.message || "登录请求失败，请检查网络或是否访问正确端口（Web 端口非 TCP 业务端口）";
      } finally {
        if (loginSubmitBtn) {
          loginSubmitBtn.disabled = false;
          loginSubmitBtn.textContent = "登录";
        }
      }
    });
  }

  $("#btn-logout").addEventListener("click", async () => {
    try {
      await api("/api/auth/logout", { method: "POST" });
    } catch (_) {}
    showLogin();
  });

  refreshMe().then((ok) => {
    if (ok) showMain();
    else showLogin();
  });
})();
