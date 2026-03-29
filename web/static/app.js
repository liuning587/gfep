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
  /** 实时日志合并刷新 rAF，离开页面前须 cancel，避免卡死 */
  let liveLogRedrawRaf = null;
  /** 单屏最多保留行数（防 textContent+= 与巨型节点导致切换标签卡顿） */
  const LIVE_LOG_MAX_LINES = 8000;

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
      { id: "bridges", label: "698 桥接" },
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
    if (liveLogRedrawRaf != null) {
      cancelAnimationFrame(liveLogRedrawRaf);
      liveLogRedrawRaf = null;
    }
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
      else if (id === "bridges") await viewBridges();
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

  /** 导出文件名用：北京时间 yyyymmddHHMMss */
  function fileStampBeijing(date) {
    const d = date instanceof Date ? date : new Date();
    const s = new Intl.DateTimeFormat("sv-SE", {
      timeZone: "Asia/Shanghai",
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
      hour12: false,
    }).format(d);
    return s.replace(/[-: ]/g, "");
  }

  function downloadTextFile(filename, text) {
    const blob = new Blob([text != null ? text : ""], { type: "text/plain;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(a.href);
  }

  function downloadXlsFromTableHtml(filename, innerTableHtml) {
    const doc =
      "<!DOCTYPE html><html><head><meta charset=\"UTF-8\"></head><body>" +
      innerTableHtml +
      "</body></html>";
    const blob = new Blob(["\ufeff" + doc], { type: "application/vnd.ms-excel;charset=utf-8" });
    const a = document.createElement("a");
    a.href = URL.createObjectURL(blob);
    a.download = filename;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(a.href);
  }

  function buildTerminalsExportTable(rows) {
    const heads = [
      "序号",
      "connId",
      "IP:port",
      "协议",
      "addr",
      "在线时长",
      "登录",
      "心跳",
      "最近收",
      "最近发",
      "上报",
      "上行帧数",
      "上行字节",
      "下行次数",
      "下行字节",
    ];
    const hr = "<tr>" + heads.map((h) => "<th>" + escapeHtml(h) + "</th>").join("") + "</tr>";
    const br = rows
      .map((r, i) => {
        return (
          "<tr><td>" +
          (i + 1) +
          "</td><td>" +
          escapeHtml(String(r.connId)) +
          "</td><td>" +
          escapeHtml(r.remoteTcp) +
          "</td><td>" +
          escapeHtml(r.protocol) +
          "</td><td>" +
          escapeHtml(r.addr) +
          "</td><td>" +
          escapeHtml(r.onlineDuration || "") +
          "</td><td>" +
          escapeHtml(r.loginTime != null ? String(r.loginTime) : "—") +
          "</td><td>" +
          escapeHtml(r.heartbeatTime != null ? String(r.heartbeatTime) : "—") +
          "</td><td>" +
          escapeHtml(r.lastRxTime != null ? String(r.lastRxTime) : "—") +
          "</td><td>" +
          escapeHtml(r.lastTxTime != null ? String(r.lastTxTime) : "—") +
          "</td><td>" +
          escapeHtml(r.lastReportTime != null ? String(r.lastReportTime) : "—") +
          "</td><td>" +
          escapeHtml(String(r.uplinkMsgCount ?? "")) +
          "</td><td>" +
          escapeHtml(String(r.uplinkBytes ?? "")) +
          "</td><td>" +
          escapeHtml(String(r.downlinkMsgCount ?? "")) +
          "</td><td>" +
          escapeHtml(String(r.downlinkBytes ?? "")) +
          "</td></tr>"
        );
      })
      .join("");
    return '<table border="1">' + hr + br + "</table>";
  }

  function fmtDurationZh(sec) {
    if (sec == null || Number.isNaN(Number(sec))) return "—";
    let s = Math.floor(Number(sec));
    if (s < 0) s = 0;
    if (s < 60) return s + "秒";
    if (s < 3600) {
      const m = Math.floor(s / 60);
      const r = s % 60;
      if (r === 0) return m + "分钟";
      return m + "分" + r + "秒";
    }
    if (s < 86400) {
      const h = Math.floor(s / 3600);
      const m = Math.floor((s % 3600) / 60);
      const r = s % 60;
      if (m === 0 && r === 0) return h + "小时";
      if (r === 0) return h + "小时" + m + "分";
      return h + "小时" + m + "分" + r + "秒";
    }
    const days = Math.floor(s / 86400);
    const rem = s % 86400;
    const h = Math.floor(rem / 3600);
    const m = Math.floor((rem % 3600) / 60);
    return days + "天" + h + "小时" + m + "分";
  }

  function fmtHostBytes(n) {
    if (n == null || Number.isNaN(Number(n))) return "—";
    let v = Math.max(0, Number(n));
    if (v > Number.MAX_SAFE_INTEGER) v = Number.MAX_SAFE_INTEGER;
    if (v < 1024) return String(Math.round(v));
    const units = ["K", "M", "G", "T", "P"];
    let ui = -1;
    let x = v;
    while (x >= 1024 && ui < units.length - 1) {
      x /= 1024;
      ui++;
    }
    const suf = units[ui];
    if (x >= 100) return x.toFixed(0) + suf;
    if (x >= 10) return x.toFixed(1) + suf;
    return x.toFixed(2) + suf;
  }

  function fmtHostBytesFromDecimalString(s) {
    if (s == null || s === "") return "—";
    let n;
    try {
      n = BigInt(String(s).trim());
    } catch {
      return "—";
    }
    if (n < 0n) n = 0n;
    if (n < 1024n) return n.toString();
    const units = ["K", "M", "G", "T", "P"];
    let ui = -1;
    let x = n;
    while (x >= 1024n && ui < units.length - 1) {
      x = x / 1024n;
      ui++;
    }
    const xv = Number(x);
    if (!Number.isFinite(xv)) return n.toString();
    const suf = units[ui];
    if (xv >= 100) return xv.toFixed(0) + suf;
    if (xv >= 10) return xv.toFixed(1) + suf;
    return xv.toFixed(2) + suf;
  }

  function drawOverviewTrafficChart(canvas, traffic) {
    if (!canvas || !canvas.getContext) return null;
    const rawT = traffic && traffic.terminalBytesPerMin;
    const rawA = traffic && traffic.appBytesPerMin;
    const tArr = Array.isArray(rawT) ? rawT.map((v) => Number(v) || 0) : [];
    const aArr = Array.isArray(rawA) ? rawA.map((v) => Number(v) || 0) : [];
    const pt = 15;
    while (tArr.length < pt) tArr.unshift(0);
    while (aArr.length < pt) aArr.unshift(0);
    if (tArr.length > pt) tArr.splice(0, tArr.length - pt);
    if (aArr.length > pt) aArr.splice(0, aArr.length - pt);
    const maxV = Math.max(1, ...tArr, ...aArr);
    const dpr = window.devicePixelRatio || 1;
    const wrap = canvas.parentElement;
    const cssW = Math.max(320, (wrap && wrap.clientWidth) || canvas.clientWidth || 600);
    const cssH = 200;
    canvas.style.width = cssW + "px";
    canvas.style.height = cssH + "px";
    canvas.width = Math.floor(cssW * dpr);
    canvas.height = Math.floor(cssH * dpr);
    const ctx = canvas.getContext("2d");
    if (!ctx) return null;
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, cssW, cssH);
    const pad = { l: 52, r: 14, t: 18, b: 36 };
    const gw = cssW - pad.l - pad.r;
    const gh = cssH - pad.t - pad.b;
    const cs = getComputedStyle(document.documentElement);
    const colTerm = (cs.getPropertyValue("--accent") || "#4a7eb5").trim();
    const colApp = (cs.getPropertyValue("--warning") || "#b8923a").trim();
    const gridCol = (cs.getPropertyValue("--divider") || "#444").trim();
    const txtCol = (cs.getPropertyValue("--text-tertiary") || "#888").trim();
    ctx.strokeStyle = gridCol;
    ctx.lineWidth = 1;
    for (let g = 0; g <= 4; g++) {
      const y = pad.t + (gh * g) / 4;
      ctx.beginPath();
      ctx.moveTo(pad.l, y);
      ctx.lineTo(pad.l + gw, y);
      ctx.stroke();
    }
    function yFrom(v) {
      return pad.t + gh - (v / maxV) * gh;
    }
    function drawSeries(arr, color) {
      ctx.beginPath();
      for (let i = 0; i < pt; i++) {
        const x = pad.l + (i / (pt - 1)) * gw;
        const y = yFrom(arr[i]);
        if (i === 0) ctx.moveTo(x, y);
        else ctx.lineTo(x, y);
      }
      ctx.strokeStyle = color;
      ctx.lineWidth = 2;
      ctx.lineJoin = "round";
      ctx.lineCap = "round";
      ctx.stroke();
    }
    drawSeries(tArr, colTerm);
    drawSeries(aArr, colApp);
    ctx.fillStyle = txtCol;
    ctx.font = "11px system-ui, Segoe UI, sans-serif";
    ctx.textAlign = "right";
    ctx.textBaseline = "middle";
    ctx.fillText(fmtHostBytes(maxV), pad.l - 6, pad.t + 8);
    ctx.fillText("0", pad.l - 6, pad.t + gh);
    ctx.textAlign = "center";
    ctx.textBaseline = "top";
    const xl = ["-14分", "-7分", "现在"];
    const xi = [0, 7, 14];
    for (let j = 0; j < xi.length; j++) {
      const i = xi[j];
      const x = pad.l + (i / (pt - 1)) * gw;
      ctx.fillText(xl[j], x, pad.t + gh + 6);
    }
    return { pad, gw, gh, pt, cssW, cssH, tArr, aArr, maxV };
  }

  /** @param {HTMLCanvasElement} canvas */
  function bindOverviewTrafficChartTooltip(canvas, layout, snap) {
    const wrap = canvas && canvas.parentElement;
    if (!canvas || !wrap || !layout) return;
    let tip = wrap.querySelector(".ov-traffic-tip");
    if (!tip) {
      tip = document.createElement("div");
      tip.className = "ov-traffic-tip";
      tip.setAttribute("role", "tooltip");
      wrap.appendChild(tip);
    }
    const { pad, gw, gh, pt, tArr, aArr } = layout;
    const byT = snap.terminalsByProtocol || {};
    const byA = snap.appsByProtocol || {};
    const tr = snap.traffic || {};
    const tcpN = snap.tcpConnTotal;

    function protoLine(label, o) {
      const keys = Object.keys(o).sort((a, b) => a.localeCompare(b));
      if (!keys.length) return escapeHtml(label) + "：—";
      const parts = keys.map((k) => escapeHtml(k) + " ×" + Number(o[k]));
      return escapeHtml(label) + "：" + parts.join("，");
    }

    function timeLabel(i) {
      const m = pt - 1 - i;
      if (m <= 0) return "最近 1 分钟桶（最右端）";
      return "约 " + m + " 分钟前";
    }

    function hide() {
      tip.classList.remove("is-visible");
      tip.innerHTML = "";
    }

    function show(html, clientX, clientY) {
      tip.innerHTML = html;
      tip.classList.add("is-visible");
      const wRect = wrap.getBoundingClientRect();
      const margin = 8;
      tip.style.visibility = "hidden";
      tip.style.left = "0";
      tip.style.top = "0";
      const tw = tip.offsetWidth;
      const th = tip.offsetHeight;
      tip.style.visibility = "";
      let lx = clientX - wRect.left + 12;
      let ly = clientY - wRect.top + 12;
      lx = Math.max(margin, Math.min(lx, wrap.clientWidth - tw - margin));
      ly = Math.max(margin, Math.min(ly, wrap.clientHeight - th - margin));
      tip.style.left = lx + "px";
      tip.style.top = ly + "px";
    }

    function indexFromCanvasXY(x, y) {
      if (x < pad.l || x > pad.l + gw || y < pad.t || y > pad.t + gh) return null;
      const rel = (x - pad.l) / gw;
      return Math.min(pt - 1, Math.max(0, Math.round(rel * (pt - 1))));
    }

    function buildHtml(i) {
      const tb = tArr[i] ?? 0;
      const ab = aArr[i] ?? 0;
      const termLine = protoLine("终端（按协议·去重）", byT);
      const appLine = protoLine("主站/APP（按协议）", byA);
      const rxT = fmtHostBytesFromDecimalString(tr.terminalTotalRx);
      const txT = fmtHostBytesFromDecimalString(tr.terminalTotalTx);
      const rxA = fmtHostBytesFromDecimalString(tr.appTotalRx);
      const txA = fmtHostBytesFromDecimalString(tr.appTotalTx);
      return (
        '<div class="ov-traffic-tip-t">' +
        escapeHtml(timeLabel(i)) +
        "</div>" +
        '<div class="ov-traffic-tip-row"><span class="lg-term">●</span> 本分钟终端总流量 <strong>' +
        escapeHtml(fmtHostBytes(tb)) +
        '</strong>　<span class="lg-app">●</span> 本分钟主站/APP总流量 <strong>' +
        escapeHtml(fmtHostBytes(ab)) +
        "</strong></div>" +
        '<div class="ov-traffic-tip-sep"></div>' +
        '<div class="ov-traffic-tip-h">当前在线（与本页刷新同步）</div>' +
        "<div>" +
        termLine +
        "</div><div>" +
        appLine +
        '</div><div class="ov-traffic-tip-muted">TCP 业务连接（含非登记）：<strong>' +
        escapeHtml(tcpN != null ? String(tcpN) : "—") +
        "</strong></div>" +
        '<div class="ov-traffic-tip-sep"></div>' +
        '<div class="ov-traffic-tip-h">当前连接累计收发</div>' +
        '<div class="ov-traffic-tip-muted">终端 收 ' +
        escapeHtml(rxT) +
        " · 发 " +
        escapeHtml(txT) +
        "</div>" +
        '<div class="ov-traffic-tip-muted">主站/APP 收 ' +
        escapeHtml(rxA) +
        " · 发 " +
        escapeHtml(txA) +
        "</div>" +
        '<div class="ov-traffic-tip-foot">各协议人数为当前快照；曲线为对应分钟总字节（整分采样）。</div>'
      );
    }

    const onMove = (ev) => {
      const rect = canvas.getBoundingClientRect();
      const x = ev.clientX - rect.left;
      const y = ev.clientY - rect.top;
      const i = indexFromCanvasXY(x, y);
      if (i == null) {
        hide();
        return;
      }
      show(buildHtml(i), ev.clientX, ev.clientY);
    };
    const onLeave = () => hide();
    canvas.addEventListener("mousemove", onMove);
    canvas.addEventListener("mouseleave", onLeave);
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

  /** @param {Record<string, unknown>} st status JSON */
  function htmlBridge698Compact(st) {
    if (!st || st.bridge698Enabled !== true) return "";
    const host = escapeHtml(String(st.bridge698Host || ""));
    return (
      '<div class="bridge698-compact">' +
      '<h4 class="bridge698-h">698 桥接</h4>' +
      '<p class="muted bridge698-desc">已启用。无匹配主站转发时，终端 698 报文可走桥接对端。</p>' +
      '<div class="stat bridge698-host-stat"><div class="k">BridgeHost698</div><div class="v mono">' +
      host +
      "</div></div></div>"
    );
  }

  /** @param {number | null | undefined} hostMemPct Sys 占主机物理内存 0–100，无采样则省略 */
  function ovHeapBar(label, mibs, maxMib, hostMemPct) {
    const v = mibs != null && !Number.isNaN(Number(mibs)) ? Number(mibs) : 0;
    const cap = maxMib > 0 ? maxMib : 1;
    const w = Math.min(100, (v / cap) * 100);
    let numHtml = v.toFixed(2) + " MiB";
    if (hostMemPct != null && Number.isFinite(hostMemPct)) {
      numHtml +=
        ' <span class="ov-heap-hostpct">· 物理内存 ' + hostMemPct.toFixed(1) + "%</span>";
    }
    return (
      '<div class="ov-meter ov-meter-compact">' +
      '<div class="ov-meter-top">' +
      '<span class="ov-meter-t">' +
      escapeHtml(label) +
      '</span><span class="ov-meter-num">' +
      numHtml +
      "</span></div>" +
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
        const tr = s.traffic || {};
        const procUp = s.processUptimeSec;
        const procAt = s.processStartedAt;
        const hostUp = h.hostUptimeSec;
        const hostBoot = h.hostBootTimeUtc;
        const uptimeBlock =
          '<div class="ov-uptime muted">' +
          "<div><span>操作系统已运行 </span><strong>" +
          fmtDurationZh(hostUp) +
          "</strong>" +
          (hostBoot
            ? '<span class="ov-uptime-at">（开机 北京 ' + escapeHtml(hostBoot) + "）</span>"
            : "") +
          "</div>" +
          "<div><span>GFEP 已运行 </span><strong>" +
          fmtDurationZh(procUp) +
          "</strong>" +
          (procAt
            ? '<span class="ov-uptime-at">（启动 北京 ' + escapeHtml(procAt) + "）</span>"
            : "") +
          "</div></div>";
        const sumProtoMap = (o) =>
          sortProtoKeys(o).reduce((acc, k) => acc + (Number(o[k]) || 0), 0);
        const nTermSum = sumProtoMap(byP);
        const nAppSum = sumProtoMap(byApp);
        const ovQuick =
          '<p class="muted ov-quicklinks">快捷：<button type="button" class="linkish" data-go="terminals">在线终端</button> · <button type="button" class="linkish" data-go="apps">主站/APP</button> · <button type="button" class="linkish" data-go="bridges">698 桥接</button></p>';
        const secRun =
          '<section class="ov-section ov-section--hero" aria-label="运行概览">' +
          '<h3 class="section-title ov-section-focus">运行概览</h3>' +
          '<p class="muted">约每 3 秒自动刷新 · 连接规模与运行时长</p>' +
          uptimeBlock +
          '<div class="ov-hero-grid">' +
          '<div class="ov-hero-tcp"><span class="muted">TCP 业务连接</span> <strong class="ov-tcp-n">' +
          (s.tcpConnTotal ?? "—") +
          "</strong></div>" +
          '<div class="grid2 ov-hero-sub">' +
          '<div class="stat"><div class="k">在线终端（按协议表求和）</div><div class="v">' +
          nTermSum +
          "</div></div>" +
          '<div class="stat"><div class="k">主站/APP（按协议表求和）</div><div class="v">' +
          nAppSum +
          "</div></div></div></div>" +
          ovQuick +
          "</section>";
        const trafficSection =
          '<section class="ov-section" aria-label="业务流量">' +
          '<h3 class="section-title">业务流量</h3><p class="muted">当前<strong>在线连接</strong>累计收/发字节；折线为最近 <strong>15 个整分</strong>钟内<strong>每分钟总流量</strong>（蓝：终端，黄：主站/APP）。连接断开后累计值会减少；采样在整分边界，启动后需经过首个完整分钟才有第一个数据点。</p>' +
          '<div class="grid2 ov-traffic-totals">' +
          '<div class="stat"><div class="k">终端 · 累计收</div><div class="v">' +
          escapeHtml(fmtHostBytesFromDecimalString(tr.terminalTotalRx)) +
          "</div></div>" +
          '<div class="stat"><div class="k">终端 · 累计发</div><div class="v">' +
          escapeHtml(fmtHostBytesFromDecimalString(tr.terminalTotalTx)) +
          "</div></div>" +
          '<div class="stat"><div class="k">主站/APP · 累计收</div><div class="v">' +
          escapeHtml(fmtHostBytesFromDecimalString(tr.appTotalRx)) +
          "</div></div>" +
          '<div class="stat"><div class="k">主站/APP · 累计发</div><div class="v">' +
          escapeHtml(fmtHostBytesFromDecimalString(tr.appTotalTx)) +
          "</div></div></div>" +
          '<div class="ov-traffic-chart-wrap">' +
          '<canvas id="ov-traffic-canvas" class="ov-traffic-canvas" aria-label="最近15分钟每分钟总流量"></canvas>' +
          '<div class="ov-traffic-legend"><span class="lg-term">●</span> 终端 &nbsp; <span class="lg-app">●</span> 主站/APP</div></div></section>';
        const ha = g.heapAllocMiB,
          hi = g.heapInuseMiB,
          hs = g.heapSysMiB,
          sy = g.sysMiB;
        const maxHeap = Math.max(1, Number(ha) || 0, Number(hi) || 0, Number(hs) || 0, Number(sy) || 0);
        const memTotalB = h.memTotalBytes;
        let sysHostMemPct = null;
        if (memTotalB != null && Number(memTotalB) > 0 && sy != null && !Number.isNaN(Number(sy))) {
          sysHostMemPct = Math.min(100, ((Number(sy) * 1048576) / Number(memTotalB)) * 100);
        }
        const secConn =
          '<section class="ov-section" aria-label="连接按协议">' +
          '<h3 class="section-title">连接 · 按协议</h3>' +
          '<p class="muted">终端为在线去重地址数；主站为 TCP 连接数</p>' +
          '<h4 class="ov-subh">在线终端</h4>' +
          '<div class="table-wrap"><table class="data"><thead><tr><th>协议</th><th>在线终端(去重)</th></tr></thead><tbody>' +
          (protoRows || '<tr><td colspan="2" class="empty">无数据</td></tr>') +
          '</tbody></table></div>' +
          '<h4 class="ov-subh">主站 / APP</h4>' +
          (s.bridge698Enabled === true
            ? '<div class="ov-apps-bridge-row">' +
              '<div class="ov-apps-col"><div class="table-wrap"><table class="data"><thead><tr><th>协议</th><th>连接数</th></tr></thead><tbody>' +
              (appRows || '<tr><td colspan="2" class="empty">无数据</td></tr>') +
              "</tbody></table></div></div>" +
              '<div class="ov-bridge-col">' +
              htmlBridge698Compact(s) +
              "</div></div>"
            : '<div class="table-wrap"><table class="data"><thead><tr><th>协议</th><th>连接数</th></tr></thead><tbody>' +
              (appRows || '<tr><td colspan="2" class="empty">无数据</td></tr>') +
              "</tbody></table></div>") +
          "</section>";
        const secHost =
          '<section class="ov-section ov-section--muted" aria-label="主机资源">' +
          '<h3 class="section-title">主机资源 (OS)</h3>' +
          '<p class="muted">gopsutil 采样 · 与下方进程堆内存百分比含义不同</p>' +
          '<div class="ov-meter-grid">' +
          ovMeter("CPU 占用", h.cpuPercent, "瞬时采样") +
          ovMeter(
            "内存占用",
            h.memUsedPercent,
            "已用 <strong>" +
              fmtHostBytes(h.memUsedBytes) +
              "</strong> / 总 <strong>" +
              fmtHostBytes(h.memTotalBytes) +
              "</strong>（物理内存）"
          ) +
          ovMeter(
            "磁盘占用",
            h.diskUsedPercent,
            "已用 <strong>" +
              fmtHostBytes(h.diskUsedBytes) +
              "</strong> / 总 <strong>" +
              fmtHostBytes(h.diskTotalBytes) +
              "</strong> · 日志目录所在分区 · <code class=\"ov-path\">" +
              escapeHtml(diskRel) +
              "</code>"
          ) +
          "</div></section>";
        const secGo =
          '<section class="ov-section ov-section--muted" aria-label="Go运行时">' +
          '<h3 class="section-title">Go 运行时与构建</h3>' +
          '<p class="muted">条形长度为相对本组四项最大值；<strong>Sys 总计</strong>旁「物理内存 %」为 runtime.Sys 约占主机物理内存（需 gopsutil 内存总览）。</p>' +
          '<h4 class="ov-subh">堆与 Sys</h4>' +
          '<div class="ov-meter-grid ov-meter-grid--tight">' +
          ovHeapBar("HeapAlloc", ha, maxHeap) +
          ovHeapBar("HeapInuse", hi, maxHeap) +
          ovHeapBar("HeapSys", hs, maxHeap) +
          ovHeapBar("Sys 总计", sy, maxHeap, sysHostMemPct) +
          "</div>" +
          '<h4 class="ov-subh">GC 与调度相关</h4><div class="grid2">' +
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
          '<h4 class="ov-subh">GFEP 构建信息</h4><p class="muted">产品版本、编译/构建时间与构建所用 Go 工具链（无 vcs 信息且未注入 ldflags 时构建时间可能为空）</p><div class="grid2">' +
          '<div class="stat"><div class="k">产品版本</div><div class="v">' +
          escapeHtml(s.version != null && s.version !== "" ? String(s.version) : "—") +
          "</div></div>" +
          '<div class="stat"><div class="k">编译 / 构建时间</div><div class="v mono">' +
          escapeHtml(s.buildTime != null && String(s.buildTime).trim() !== "" ? String(s.buildTime) : "—") +
          "</div></div>" +
          '<div class="stat"><div class="k">Go 版本</div><div class="v mono">' +
          escapeHtml(s.goVersion != null && String(s.goVersion).trim() !== "" ? String(s.goVersion) : "—") +
          "</div></div></div>" +
          "<p class=\"muted\">Worker池 " +
          (s.workerPoolSize ?? "—") +
          " · 队列 " +
          (s.maxWorkerTaskLen ?? "—") +
          "</p></section>";
        mount.innerHTML =
          '<div class="card overview-page">' +
          secRun +
          trafficSection +
          secConn +
          secHost +
          secGo +
          "</div>";
        mount.querySelectorAll("button[data-go]").forEach((btn) => {
          btn.addEventListener("click", () => selectTab(btn.getAttribute("data-go")));
        });
        requestAnimationFrame(() => {
          const c = mount.querySelector("#ov-traffic-canvas");
          if (!c) return;
          const layout = drawOverviewTrafficChart(c, s.traffic || {});
          bindOverviewTrafficChartTooltip(c, layout, {
            terminalsByProtocol: s.terminalsByProtocol || {},
            appsByProtocol: s.appsByProtocol || {},
            tcpConnTotal: s.tcpConnTotal,
            traffic: s.traffic || {},
          });
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
      '<label class="cb"><input type="checkbox" id="tshowcid" /> 显示 connId</label>' +
      '<span class="toolbar-sep" aria-hidden="true"></span>' +
      '<label class="inline muted">排序</label>' +
      '<select id="tsort"><option value="login">登录时间</option><option value="addr">终端地址</option></select>' +
      '<select id="torder"><option value="desc">降序</option><option value="asc">升序</option></select>' +
      '<label class="inline muted">每页</label>' +
      '<select id="tpsize"><option value="10">10</option><option value="20" selected>20</option><option value="50">50</option><option value="100">100</option><option value="200">200</option><option value="500">500</option><option value="1000">1000</option></select>' +
      '<button class="primary" id="tref">刷新</button>' +
      '<button type="button" id="txls">导出 XLS</button></div>' +
      '<div id="ttable"></div>' +
      '<div class="pager toolbar pager-term" id="tpnav" hidden>' +
      '<button type="button" id="tfirst">首页</button>' +
      '<button type="button" id="tprev">上一页</button>' +
      '<label class="inline pager-goto">' +
      '<span class="muted">第</span> <input type="number" id="tpgoto" min="1" step="1" class="pager-page-input" title="页码" /> ' +
      '<span class="muted">页</span> <button type="button" id="tpgobtn">跳转</button></label>' +
      '<span id="tpinfo" class="muted pager-info"></span>' +
      '<button type="button" id="tnext">下一页</button>' +
      '<button type="button" id="tlast">末页</button></div></div>';

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
      $("#tpinfo").textContent = "共 " + maxPage + " 页 · " + total + " 条";
      const pgIn = $("#tpgoto");
      if (pgIn) {
        pgIn.max = maxPage;
        pgIn.value = String(page);
      }
      const disStart = page <= 1;
      const disEnd = page >= maxPage;
      $("#tfirst").disabled = disStart;
      $("#tprev").disabled = disStart;
      $("#tnext").disabled = disEnd;
      $("#tlast").disabled = disEnd;
      const showDur = $("#tshowdur").checked;
      const showCid = $("#tshowcid").checked;
      const cidTh = showCid ? "<th>connId</th>" : "";
      const durTh = showDur ? "<th>在线时长</th>" : "";
      const cols =
        "<th>#</th>" +
        cidTh +
        "<th>IP:port</th><th>协议</th><th>addr</th>" +
        durTh +
        "<th>登录</th><th>心跳</th><th>最近收</th><th>最近发</th><th>上报</th><th>上行帧/字节</th><th>下行次/字节</th><th></th>";
      const base = (page - 1) * pageSize;
      let i = base;
      const body = rows
        .map((r) => {
          i++;
          const dur = r.onlineDuration ? String(r.onlineDuration) : "";
          const durCell = showDur ? "<td>" + escapeHtml(dur || "—") + "</td>" : "";
          const cidCell = showCid ? "<td>" + r.connId + "</td>" : "";
          return (
            "<tr><td>" +
            i +
            "</td>" +
            cidCell +
            "<td>" +
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
    $("#tshowcid").addEventListener("change", () => paintTerminals());
    $("#tq").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") {
        tPage = 1;
        runSafe();
      }
    });
    const maxPageFromSnap = () => {
      if (!termSnap || !termSnap.total) return 1;
      const ps = termSnap.pageSize || 20;
      return Math.max(1, Math.ceil(termSnap.total / ps));
    };
    $("#tfirst").addEventListener("click", () => {
      tPage = 1;
      runSafe();
    });
    $("#tprev").addEventListener("click", () => {
      if (tPage > 1) {
        tPage--;
        runSafe();
      }
    });
    const doGotoPage = () => {
      const pgIn = $("#tpgoto");
      if (!pgIn) return;
      const mp = maxPageFromSnap();
      let v = parseInt(String(pgIn.value).trim(), 10);
      if (!Number.isFinite(v)) {
        alert("请输入有效页码");
        return;
      }
      if (v < 1) v = 1;
      if (v > mp) v = mp;
      tPage = v;
      runSafe();
    };
    $("#tpgobtn").addEventListener("click", () => doGotoPage());
    $("#tpgoto").addEventListener("keydown", (ev) => {
      if (ev.key === "Enter") {
        ev.preventDefault();
        doGotoPage();
      }
    });
    $("#tnext").addEventListener("click", () => {
      tPage++;
      runSafe();
    });
    $("#tlast").addEventListener("click", () => {
      tPage = maxPageFromSnap();
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
    $("#txls").addEventListener("click", async () => {
      try {
        const q = new URLSearchParams();
        const f = $("#tq").value.trim();
        if (f) q.set("q", f);
        const p = $("#tp").value;
        if (p) q.set("protocol", p);
        if ($("#tex").checked) q.set("expand", "1");
        q.set("sort", $("#tsort").value);
        q.set("order", $("#torder").value);
        q.set("all", "1");
        const data = await api("/api/terminals?" + q.toString());
        const rows = data.rows || [];
        if (!rows.length) {
          alert("暂无数据可导出");
          return;
        }
        const stamp = fileStampBeijing(new Date());
        downloadXlsFromTableHtml("gfep-terminals-" + stamp + ".xls", buildTerminalsExportTable(rows));
      } catch (e) {
        alert(e.message);
      }
    });
    await run();
  }

  async function viewApps() {
    content.innerHTML =
      '<div class="card"><h2>主站 / APP 连接</h2><p class="muted">' +
      "上行 = 主站→FEP 帧数/字节，下行 = FEP→主站（与终端表视角相反）。698 终端桥接至主站的<strong>独立 TCP</strong>见菜单 <strong>698 桥接</strong>。</p>" +
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

  async function viewBridges() {
    const st = await api("/api/status").catch(() => ({}));
    const en = st.bridge698Enabled === true;
    const initial = await api("/api/bridges").catch(() => ({ rows: [], help: "" }));
    const helpHint = escapeHtml(String(initial.help || ""));
    content.innerHTML =
      '<div class="card"><h2>698 桥接连接</h2>' +
      (en
        ? '<p class="muted">每条记录对应<strong>一个在线终端 TCP</strong>上挂起的、至配置 <code>BridgeHost698</code> 的桥接链路。' +
          "<strong>自主站收</strong>（rx）/ <strong>至主站发</strong>（tx）为<strong>桥接 socket</strong>上的帧数与字节；登录/心跳/透传均计入。</p>"
        : '<p class="muted bridge698-desc">当前配置下 <strong>BridgeHost698</strong> 未启用（空或以 0 开头）。无桥接链路时下列为空属正常。</p>') +
      '<p class="muted">' +
      helpHint +
      "</p>" +
      '<div class="toolbar"><input type="search" id="bq" placeholder="主站地址 / 终端 addr / IP / 地址hex" /><button class="primary" id="bref">刷新</button></div><div id="btable"></div></div>';
    const run = async () => {
      const q = $("#bq").value.trim();
      const data = await api("/api/bridges" + (q ? "?q=" + encodeURIComponent(q) : ""));
      const rows = data.rows || [];
      if (!rows.length) {
        $("#btable").innerHTML =
          '<p class="empty">当前无桥接对象（无终端挂起桥接，或未启用 698 桥接）</p>';
        return;
      }
      const cols =
        "<th>#</th><th>终端connId</th><th>终端addr</th><th>终端IP:port</th><th>桥地址(hex)</th><th>主站</th><th>规约</th><th>状态</th><th>TCP起</th><th>桥登录</th><th>最近心跳</th><th>在线时长</th><th>最近收</th><th>最近发</th><th>rx帧/字节</th><th>tx帧/字节</th><th>heartUnAck</th>";
      let i = 0;
      const body = rows
        .map((r) => {
          i++;
          return (
            "<tr><td>" +
            i +
            "</td><td>" +
            r.terminalConnId +
            "</td><td>" +
            escapeHtml(r.terminalAddr || "—") +
            "</td><td>" +
            escapeHtml(r.terminalRemoteTcp || "—") +
            "</td><td class=\"mono\">" +
            escapeHtml(r.addrHex || "—") +
            "</td><td class=\"mono\">" +
            escapeHtml(r.bridgeHost || "—") +
            "</td><td>" +
            escapeHtml(r.protocol || "—") +
            "</td><td title=\"" +
            escapeHtml(r.status || "") +
            "\">" +
            escapeHtml(r.statusText || r.status || "—") +
            "</td><td>" +
            (r.tcpSince || "—") +
            "</td><td>" +
            (r.loginTime || "—") +
            "</td><td>" +
            (r.heartbeatTime || "—") +
            "</td><td>" +
            escapeHtml(r.onlineDuration || "—") +
            "</td><td>" +
            (r.lastRxTime || "—") +
            "</td><td>" +
            (r.lastTxTime || "—") +
            "</td><td>" +
            r.rxPkts +
            " / " +
            r.rxBytes +
            "</td><td>" +
            r.txPkts +
            " / " +
            r.txBytes +
            "</td><td>" +
            (r.heartUnAck != null ? r.heartUnAck : "—") +
            "</td></tr>"
          );
        })
        .join("");
      $("#btable").innerHTML =
        '<div class="table-wrap"><table class="data"><thead><tr>' + cols + "</tr></thead><tbody>" + body + "</tbody></table></div>";
    };
    $("#bref").addEventListener("click", () => run().catch((e) => alert(e.message)));
    await run();
  }

  function viewLive() {
    const liveProtoOptions = [
      "376.1",
      "698.45",
      "NW",
      "376-主站",
      "698-主站",
      "Nw-主站",
    ];
    const protoChipsHtml = liveProtoOptions
      .map(
        (p) =>
          '<label class="lfp-chip"><input type="checkbox" name="lfp-proto" value="' +
          escapeHtml(p) +
          '" checked /> <span>' +
          escapeHtml(p) +
          "</span></label>"
      )
      .join("");
    content.innerHTML =
      '<div class="card card-live"><div class="live-card-head">' +
      "<h2>实时通信日志</h2>" +
      '<p class="muted live-intro">需开启 LogPacketHex / LogLinkLayer 等；SSE 推送。<strong>全部勾选</strong>或<strong>全部不勾选</strong>均不按协议过滤；仅<strong>部分勾选</strong>时按所选过滤。改条件后会自动重连（亦可点应用过滤）。界面最多保留约 <strong>' +
      LIVE_LOG_MAX_LINES +
      "</strong> 行，超出丢弃最旧，减轻卡顿。</p></div>" +
      '<div class="toolbar toolbar-live">' +
      '<div class="lfp-inline" role="group" aria-label="按协议过滤">' +
      '<span class="muted lfp-inline-label">协议</span>' +
      '<div id="lfp-chips" class="lfp-chips">' +
      protoChipsHtml +
      '</div><span class="lfp-quick">' +
      '<button type="button" class="linkish" id="lfp-all">全选</button>' +
      '<span class="muted lfp-dot" aria-hidden="true">·</span>' +
      '<button type="button" class="linkish" id="lfp-none">全不选</button>' +
      "</span></div>" +
      '<input type="search" id="lf" placeholder="addr / IP:port" />' +
      '<button type="button" class="primary" id="lapply">应用过滤</button>' +
      '<button type="button" id="lclr">清空</button>' +
      '<button type="button" id="ldl-txt">下载 .txt</button>' +
      '<button type="button" id="ldl-log">下载 .log</button></div><div class="log-view" id="logbox"></div></div>';
    const box = $("#logbox");
    const liveLogLines = [];
    const pendingLiveLines = [];
    const pushLiveLine = (line) => {
      pendingLiveLines.push(line);
    };
    const trimLiveLogFromHead = (removeCount) => {
      if (removeCount <= 0) return;
      liveLogLines.splice(0, removeCount);
      let n = 0;
      while (n < removeCount && box.firstChild) {
        box.removeChild(box.firstChild);
        n++;
      }
    };
    const flushLiveLogDom = () => {
      liveLogRedrawRaf = null;
      if (!box || !box.isConnected) return;
      const stick = box.scrollHeight - box.scrollTop - box.clientHeight < 800;
      const batch = pendingLiveLines.splice(0, pendingLiveLines.length);
      for (let i = 0; i < batch.length; i++) {
        const line = batch[i];
        liveLogLines.push(line);
        const row = document.createElement("div");
        row.className = "log-line";
        row.textContent = line;
        box.appendChild(row);
      }
      const over = liveLogLines.length - LIVE_LOG_MAX_LINES;
      if (over > 0) trimLiveLogFromHead(over);
      if (stick) box.scrollTop = box.scrollHeight;
    };
    const scheduleLiveLogRedraw = () => {
      if (liveLogRedrawRaf != null) return;
      liveLogRedrawRaf = requestAnimationFrame(flushLiveLogDom);
    };
    const protoCheckboxes = () => Array.from(document.querySelectorAll("#lfp-chips input[name=lfp-proto]"));
    $("#lfp-all").addEventListener("click", () => {
      protoCheckboxes().forEach((el) => {
        el.checked = true;
      });
    });
    $("#lfp-none").addEventListener("click", () => {
      protoCheckboxes().forEach((el) => {
        el.checked = false;
      });
    });
    const streamURL = () => {
      const q = new URLSearchParams();
      const a = $("#lf").value.trim();
      if (a) q.set("addr", a);
      const all = protoCheckboxes();
      const nAll = all.length;
      const checked = all.filter((inp) => inp.checked);
      const nCh = checked.length;
      if (nCh > 0 && nCh < nAll) {
        checked.forEach((inp) => {
          if (inp.value) q.append("protocol", inp.value);
        });
      }
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
          pushLiveLine((o.ts || "") + " " + (o.line || ""));
        } catch {
          pushLiveLine(String(ev.data || ""));
        }
        scheduleLiveLogRedraw();
      };
      liveES.onerror = () => {
        pushLiveLine("[连接中断，请切换页面重试]");
        scheduleLiveLogRedraw();
      };
    };
    let liveReconnectTimer = null;
    const scheduleLiveReconnect = () => {
      if (liveReconnectTimer != null) clearTimeout(liveReconnectTimer);
      liveReconnectTimer = setTimeout(() => {
        liveReconnectTimer = null;
        connect();
      }, 320);
    };
    connect();
    $("#lapply").addEventListener("click", () => connect());
    $("#lclr").addEventListener("click", () => {
      liveLogLines.length = 0;
      pendingLiveLines.length = 0;
      if (liveLogRedrawRaf != null) {
        cancelAnimationFrame(liveLogRedrawRaf);
        liveLogRedrawRaf = null;
      }
      box.textContent = "";
    });
    $("#ldl-txt").addEventListener("click", () => {
      const stamp = fileStampBeijing(new Date());
      const t = liveLogLines.length ? liveLogLines.join("\n") + "\n" : "";
      downloadTextFile("gfep-live-" + stamp + ".txt", t);
    });
    $("#ldl-log").addEventListener("click", () => {
      const stamp = fileStampBeijing(new Date());
      const t = liveLogLines.length ? liveLogLines.join("\n") + "\n" : "";
      downloadTextFile("gfep-live-" + stamp + ".log", t);
    });
    $("#lfp-chips").addEventListener("change", scheduleLiveReconnect);
    $("#lf").addEventListener("input", scheduleLiveReconnect);
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
    const totalHum =
      data.totalSizeHuman != null && String(data.totalSizeHuman).trim() !== ""
        ? String(data.totalSizeHuman)
        : data.totalSize != null
          ? fmtHostBytes(Number(data.totalSize))
          : "—";
    content.innerHTML =
      '<div class="card"><h2>历史日志</h2><p class="muted log-files-head">' +
      '<span class="log-root-path">' +
      escapeHtml(data.root || "") +
      '</span><span class="log-root-total">目录内文件合计 <strong>' +
      escapeHtml(totalHum) +
      "</strong></span></p><div class=\"table-wrap\"><table class=\"data\"><thead><tr><th>文件</th><th>大小</th><th>修改时间</th><th></th></tr></thead><tbody>" +
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
