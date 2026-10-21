/* PkgHub 前端应用：单页结构 + REST API */
'use strict';

const $ = (sel, root) => (root || document).querySelector(sel);
const $$ = (sel, root) => Array.from((root || document).querySelectorAll(sel));

const app = $('#app');
const APP_NAME = document.body.dataset.app || 'PkgHub';
const VERSION = document.body.dataset.version || '';
const MAX_MB = parseInt(document.body.dataset.maxMb || '0', 10);

// storedPkgView 读取记住的展示方式（localStorage 不可用时回退为卡片）。
function storedPkgView() {
  try {
    return localStorage.getItem('pkghub-pkg-view') === 'list' ? 'list' : 'card';
  } catch (e) {
    return 'card';
  }
}

const state = {
  me: null,
  meta: null,
  categories: [],
  pkgs: {
    keyword: '', category: 0, sort: 'updated', page: 1, pageSize: 12, total: 0,
    view: storedPkgView(), // card | list
    cache: null, // 当前页数据，供视图切换时本地重渲染
  },
  uploadFile: null,
};

/* ---------------- 工具函数 ---------------- */

// isAdmin 当前登录用户是否为管理员（新增/编辑软件包、分类与用户管理仅管理员可用）。
function isAdmin() {
  return !!(state.me && state.me.role === 'admin');
}

// requireAdmin 非管理员时给出提示并返回 false，用于动作层二次防护。
function requireAdmin(action) {
  if (isAdmin()) return true;
  toast('没有权限', '仅管理员可以' + action, 'error');
  return false;
}

function esc(s) {
  return String(s == null ? '' : s).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  })[c]);
}

function fmtBytes(n) {
  const v = Number(n || 0);
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  let i = 0, x = v;
  while (x >= 1024 && i < units.length - 1) { x /= 1024; i++; }
  return (i === 0 ? x : x.toFixed(x < 10 ? 2 : 1)) + ' ' + units[i];
}

function fmtTime(s) {
  if (!s) return '—';
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  return d.toLocaleString('zh-CN', { year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' });
}

function fmtAgo(s) {
  if (!s) return '—';
  const d = new Date(s).getTime();
  if (isNaN(d)) return s;
  const diff = (Date.now() - d) / 1000;
  if (diff < 60) return '刚刚';
  if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前';
  if (diff < 86400) return Math.floor(diff / 3600) + ' 小时前';
  if (diff < 2592000) return Math.floor(diff / 86400) + ' 天前';
  return fmtTime(s);
}

// brandMark 由平台名称生成品牌图标文字：英文取首字母组合，中文取首字。
function brandMark(name) {
  const s = String(name || APP_NAME || 'PkgHub').trim();
  if (!s) return 'PH';
  if (/^[\x00-\x7F]/.test(s)) {
    const words = s.split(/[\s._-]+/).filter(Boolean);
    if (words.length >= 2) return (words[0][0] + words[1][0]).toUpperCase();
    return Array.from(s.slice(0, 2)).join('').toUpperCase();
  }
  return Array.from(s)[0];
}

function toast(title, msg, type) {
  const box = document.createElement('div');
  box.className = 'toast ' + (type || 'info');
  const icon = type === 'success' ? '✅' : type === 'error' ? '⛔' : 'ℹ️';
  box.innerHTML = `<span class="ti">${icon}</span><div><div class="tt">${esc(title)}</div>${msg ? `<div class="tm">${esc(msg)}</div>` : ''}</div>`;
  $('#toast-container').appendChild(box);
  setTimeout(() => {
    box.style.transition = 'opacity .3s';
    box.style.opacity = '0';
    setTimeout(() => box.remove(), 320);
  }, 2600);
}

async function api(path, options) {
  const opts = Object.assign({ method: 'GET', credentials: 'same-origin', headers: {} }, options || {});
  if (opts.body !== undefined) {
    opts.headers['Content-Type'] = 'application/json';
    opts.body = JSON.stringify(opts.body);
  }
  let res;
  try {
    res = await fetch(path, opts);
  } catch (e) {
    throw new Error('无法连接服务器');
  }
  if (res.status === 401) {
    state.me = null;
    renderLogin('登录状态已失效，请重新登录');
    throw new Error('未登录');
  }
  const ct = res.headers.get('content-type') || '';
  const data = ct.indexOf('application/json') >= 0 ? await res.json() : null;
  if (!res.ok || (data && data.ok === false)) {
    throw new Error((data && data.error) || ('请求失败（HTTP ' + res.status + '）'));
  }
  return data ? data.data : null;
}

function uploadFile(url, formData, onProgress) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    xhr.open('POST', url);
    xhr.withCredentials = true;
    xhr.upload.onprogress = (e) => { if (e.lengthComputable) onProgress(e.loaded, e.total); };
    xhr.onload = () => {
      let data = null;
      try { data = JSON.parse(xhr.responseText); } catch (e) { return reject(new Error('服务器响应无法解析')); }
      if (xhr.status >= 200 && xhr.status < 300 && data && data.ok) resolve(data.data);
      else reject(new Error((data && data.error) || ('上传失败（HTTP ' + xhr.status + '）')));
    };
    xhr.onerror = () => reject(new Error('网络错误，上传中断'));
    xhr.send(formData);
  });
}

/* ---------------- 弹窗 ---------------- */

function modal(opts) {
  const mask = document.createElement('div');
  mask.className = 'modal-mask';
  mask.innerHTML = `
    <div class="modal ${opts.size === 'lg' ? 'modal-lg' : ''}">
      <div class="modal-head"><h3>${esc(opts.title)}</h3></div>
      <div class="modal-body">${opts.body || ''}</div>
      <div class="modal-foot">
        <button class="btn" data-act="cancel">${esc(opts.cancelText || '取消')}</button>
        ${opts.hideOk ? '' : `<button class="btn btn-primary" data-act="ok">${esc(opts.okText || '保存')}</button>`}
      </div>
    </div>`;
  $('#modal-root').appendChild(mask);
  const close = () => mask.remove();
  const cancelBtn = mask.querySelector('[data-act="cancel"]');
  cancelBtn.textContent = opts.cancelText || '取消';
  cancelBtn.onclick = () => { close(); if (opts.onCancel) opts.onCancel(); };
  mask.addEventListener('mousedown', (e) => { if (e.target === mask) { close(); if (opts.onCancel) opts.onCancel(); } });
  const okBtn = mask.querySelector('[data-act="ok"]');
  if (okBtn) {
    okBtn.onclick = async () => {
      okBtn.disabled = true;
      const old = okBtn.textContent;
      okBtn.textContent = '处理中…';
      try {
        const r = await opts.onOk(mask);
        if (r !== false) close();
      } catch (err) {
        toast('操作失败', err.message, 'error');
      } finally {
        okBtn.disabled = false;
        okBtn.textContent = old;
      }
    };
  }
  const first = mask.querySelector('input,select,textarea');
  if (first) setTimeout(() => first.focus(), 50);
  return mask;
}

function confirmBox(title, message, okText, danger) {
  return new Promise((resolve) => {
    let done = false;
    const finish = (v) => { if (!done) { done = true; resolve(v); } };
    const mask = modal({
      title,
      body: `<div class="muted">${esc(message)}</div>`,
      okText: okText || '确定',
      onCancel: () => finish(false),
      onOk: async () => { finish(true); },
    });
    const okBtn = mask.querySelector('[data-act="ok"]');
    if (okBtn && danger) okBtn.classList.add('btn-danger');
  });
}

const val = (root, sel) => {
  const el = root.querySelector(sel);
  return el ? String(el.value || '').trim() : '';
};

/* ---------------- 登录页 ---------------- */

function renderLogin(message) {
  document.title = APP_NAME + ' · 登录';
  app.innerHTML = `
    <div class="login-wrap">
      <form class="login-card" id="login-form" autocomplete="on">
        <div class="login-brand">
          <div class="brand-mark" style="width:44px;height:44px;font-size:19px">${esc(brandMark(APP_NAME))}</div>
          <div>
            <h1 class="login-title">${esc(APP_NAME)}</h1>
            <p class="login-sub">软件包管理平台</p>
          </div>
        </div>
        <div class="login-error ${message ? 'show' : ''}" id="login-error">${esc(message || '')}</div>
        <div class="field">
          <label for="username">用户名</label>
          <input class="input" id="username" name="username" placeholder="请输入用户名" autocomplete="username" required>
        </div>
        <div class="field">
          <label for="password">密码</label>
          <input class="input" id="password" name="password" type="password" placeholder="请输入密码" autocomplete="current-password" required>
        </div>
        <label class="flex small muted" style="margin-bottom:16px">
          <input type="checkbox" id="remember" style="width:auto"> 记住我（7 天免登录）
        </label>
        <button class="btn btn-primary" style="width:100%;padding:11px" type="submit" id="login-btn">登 录</button>
        <div class="login-tip">${esc(APP_NAME)} v${esc(VERSION)}</div>
      </form>
    </div>`;
  $('#login-form').addEventListener('submit', async (e) => {
    e.preventDefault();
    const btn = $('#login-btn');
    btn.disabled = true;
    btn.textContent = '登录中…';
    try {
      const user = await api('/api/auth/login', {
        method: 'POST',
        body: {
          username: val(document, '#username'),
          password: val(document, '#password'),
          remember: $('#remember').checked,
        },
      });
      state.me = user;
      await afterLogin();
      toast('欢迎回来', user.display_name || user.username, 'success');
    } catch (err) {
      const box = $('#login-error');
      box.textContent = err.message;
      box.classList.add('show');
      btn.disabled = false;
      btn.textContent = '登 录';
    }
  });
}

async function afterLogin() {
  await loadCategories();
  renderShell();
  route();
}

/* ---------------- 应用骨架 ---------------- */

async function loadCategories() {
  try { state.categories = (await api('/api/categories')) || []; }
  catch (e) { state.categories = []; }
}

function renderShell() {
  const me = state.me;
  const isAdmin = me && me.role === 'admin';
  document.title = APP_NAME + ' · 软件包管理';
  app.innerHTML = `
    <div class="layout">
      <aside class="sidebar" id="sidebar">
        <div class="brand">
          <div class="brand-mark">${esc(brandMark(APP_NAME))}</div>
          <div>
            <div class="brand-text">${esc(APP_NAME)}</div>
            <div class="brand-sub">软件包管理平台</div>
          </div>
        </div>
        <nav class="nav">
          <div class="nav-label">主导航</div>
          <button class="nav-item" data-action="nav" data-hash="#/dashboard"><span class="ico">📊</span>仪表盘</button>
          <button class="nav-item" data-action="nav" data-hash="#/packages"><span class="ico">📦</span>软件包<span class="count" id="nav-pkg-count"></span></button>
          ${isAdmin ? `<button class="nav-item" data-action="nav" data-hash="#/categories"><span class="ico">🗂️</span>分类管理</button>` : ''}
          ${isAdmin ? `<button class="nav-item" data-action="nav" data-hash="#/users"><span class="ico">👥</span>用户管理</button>` : ''}
          <div class="nav-label">分类</div>
          <div id="nav-categories"></div>
        </nav>
        <div class="sidebar-foot">
          <div class="kv"><span>版本</span><span>v${esc(VERSION)}</span></div>
          <div class="kv"><span>上传上限</span><span>${MAX_MB} MB</span></div>
        </div>
      </aside>
      <div class="main">
        <header class="topbar">
          <div>
            <h1 id="page-title">仪表盘</h1>
            <div class="crumb" id="page-crumb">系统总览</div>
          </div>
          <div class="topbar-right">
            <button class="icon-btn" id="theme-btn" title="切换主题">🌓</button>
            <div class="user-menu">
              <button class="user-chip" id="user-chip">
                <span class="avatar">${esc(initial(me))}</span>
                <span>
                  <div class="user-name">${esc(me.display_name || me.username)}</div>
                  <div class="user-role">${isAdmin ? '管理员' : '普通用户'}</div>
                </span>
                <span class="muted">▾</span>
              </button>
              <div class="dropdown" id="user-dropdown">
                <div class="dd-item" style="pointer-events:none;opacity:.75">
                  <span class="avatar" style="width:26px;height:26px;font-size:12px">${esc(initial(me))}</span>
                  <div><div style="font-weight:600">${esc(me.display_name || me.username)}</div>
                  <div class="small muted">@${esc(me.username)}</div></div>
                </div>
                <div class="dd-sep"></div>
                <button class="dd-item" data-action="nav" data-hash="#/account">⚙️ 个人设置</button>
                <button class="dd-item" data-action="logout">🚪 退出登录</button>
              </div>
            </div>
          </div>
        </header>
        <main class="content" id="view"></main>
      </div>
    </div>`;

  renderNavCategories();

  $('#theme-btn').onclick = () => {
    const cur = document.documentElement.getAttribute('data-theme');
    const next = cur === 'dark' ? 'light' : 'dark';
    document.documentElement.setAttribute('data-theme', next);
    localStorage.setItem('pkghub-theme', next);
  };
  $('#user-chip').onclick = (e) => { e.stopPropagation(); $('#user-dropdown').classList.toggle('open'); };
  document.addEventListener('click', () => {
    const dd = $('#user-dropdown');
    if (dd) dd.classList.remove('open');
  });
  updateNavCount();
}

function initial(me) {
  const n = (me && (me.display_name || me.username)) || '?';
  return n.trim().charAt(0).toUpperCase();
}

function renderNavCategories() {
  const box = $('#nav-categories');
  if (!box) return;
  box.innerHTML = state.categories.map((c) => `
      <button class="nav-item" data-action="filter-cat" data-id="${c.id}">
        <span class="ico" style="color:${esc(c.color)}">●</span>
        <span style="overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(c.name)}</span>
        <span class="count">${c.package_count}</span>
      </button>`).join('') || '<div class="nav-item" style="opacity:.6">暂无分类</div>';
}

async function updateNavCount() {
  try {
    const ov = await api('/api/stats/overview');
    const el = $('#nav-pkg-count');
    if (el) el.textContent = ov.package_count;
  } catch (e) { /* 忽略 */ }
}

function setTitle(title, crumb) {
  const t = $('#page-title'), c = $('#page-crumb');
  if (t) t.textContent = title;
  if (c) c.textContent = crumb || '';
}

function setActiveNav(hash) {
  $$('.nav-item').forEach((el) => el.classList.toggle('active', el.dataset.hash === hash));
}

function navigate(hash) { location.hash = hash; }

function currentRoute() {
  const h = (location.hash || '#/dashboard').replace(/^#\/?/, '');
  const parts = h.split('/').filter(Boolean);
  return { name: parts[0] || 'dashboard', param: parts[1] || '', hash: '#/' + (parts[0] || 'dashboard') };
}

let routeToken = 0;
async function route() {
  const r = currentRoute();
  const view = $('#view');
  if (!view) { renderLogin(); return; }
  setActiveNav(r.hash);
  const my = ++routeToken;
  view.innerHTML = '<span class="spinner"></span>';
  try {
    if (r.name === 'packages' && r.param) await viewPackageDetail(r.param);
    else if (r.name === 'packages') await viewPackages();
    else if (r.name === 'categories') await viewCategories();
    else if (r.name === 'users') await viewUsers();
    else if (r.name === 'account') await viewAccount();
    else await viewDashboard();
  } catch (err) {
    if (my === routeToken) view.innerHTML = `<div class="empty"><div class="big">😕</div><div class="t">页面加载失败</div><div>${esc(err.message)}</div></div>`;
  }
}

/* ---------------- 视图：仪表盘 ---------------- */

async function viewDashboard() {
  setTitle('仪表盘', '系统总览');
  const ov = await api('/api/stats/overview');
  const maxCat = Math.max(1, ...ov.category_stats.map((c) => c.package_count));
  $('#view').innerHTML = `
    <div class="stat-grid">
      ${statCard('📦', '软件包', ov.package_count, ov.category_stats.length + ' 个分类')}
      ${statCard('🏷️', '版本数量', ov.version_count, '累计发布的版本')}
      ${statCard('⬇️', '下载次数', ov.downloads, '所有版本合计')}
      ${statCard('💾', '存储占用', fmtBytes(ov.total_size), '服务端文件体积')}
      ${statCard('👥', '用户数', ov.user_count, '系统账号')}
    </div>
    <div style="display:grid;grid-template-columns:1.15fr .85fr;gap:16px;align-items:start">
      <div style="display:flex;flex-direction:column;gap:16px">
        <section class="card">
          <div class="card-head"><h3>最近上传</h3><span class="sub">最新发布的 8 个版本</span>
            <div class="right"><button class="btn btn-sm" data-action="nav" data-hash="#/packages">查看全部</button></div></div>
          <div class="card-pad">
            ${ov.recent_versions.length ? ov.recent_versions.map((v) => `
              <div class="list-item">
                <div class="grow">
                  <div class="t">${esc(v.package_name)} <span class="badge">${esc(v.version)}</span></div>
                  <div class="s">${esc(v.file_name)} · ${fmtBytes(v.size)} · ${esc(v.uploader_name)} · ${fmtAgo(v.created_at)}</div>
                </div>
                <button class="btn btn-sm" data-action="open-package" data-id="${v.package_id}">详情</button>
              </div>`).join('') : emptyBox('📦', '还没有上传任何版本', '在软件包详情页上传第一个版本')}
          </div>
        </section>
        <section class="card">
          <div class="card-head"><h3>下载排行</h3><span class="sub">Top 5 软件包</span></div>
          <div class="card-pad">
            ${ov.top_packages.filter((p) => p.downloads > 0).length ? ov.top_packages.filter((p) => p.downloads > 0).map((p, i) => `
              <div class="list-item">
                <div class="avatar" style="width:26px;height:26px;font-size:12px">${i + 1}</div>
                <div class="grow"><div class="t">${esc(p.name)}</div></div>
                <div class="badge">${p.downloads} 次</div>
                <button class="btn btn-sm" data-action="open-package" data-id="${p.package_id}">详情</button>
              </div>`).join('') : emptyBox('⬇️', '暂无下载记录', '下载软件包后这里会出现排行')}
          </div>
        </section>
      </div>
      <section class="card">
        <div class="card-head"><h3>分类分布</h3><span class="sub">按软件包数量</span></div>
        <div class="card-pad">
          ${ov.category_stats.length ? ov.category_stats.map((c) => `
            <div class="bar-row">
              <div class="bar-name" title="${esc(c.category_name)}">${esc(c.category_name)}</div>
              <div class="bar-track"><div class="bar-fill" style="width:${Math.round((c.package_count / maxCat) * 100)}%;background:${esc(c.color)}"></div></div>
              <div class="bar-val">${c.package_count} 个</div>
            </div>`).join('') : emptyBox('🗂️', '暂无分类', '先在分类管理中创建分类')}
        </div>
      </section>
    </div>`;
}

function statCard(icon, label, value, foot) {
  return `<div class="stat">
      <div class="label"><span>${icon}</span>${esc(label)}</div>
      <div class="value">${esc(value)}</div>
      <div class="foot">${esc(foot)}</div>
    </div>`;
}

function emptyBox(icon, title, sub) {
  return `<div class="empty"><div class="big">${icon}</div><div class="t">${esc(title)}</div><div>${esc(sub || '')}</div></div>`;
}

/* ---------------- 视图：软件包列表 ---------------- */

async function viewPackages() {
  setTitle('软件包', '按分类浏览、搜索与管理');
  const { keyword, category, sort, page, pageSize, view } = state.pkgs;
  const data = await api(`/api/packages?keyword=${encodeURIComponent(keyword)}&category_id=${category}&sort=${sort}&page=${page}&page_size=${pageSize}`);
  state.pkgs.total = data.total;
  state.pkgs.cache = data;
  $('#view').innerHTML = `
    <div class="toolbar">
      <div class="search-box">
        <span class="s-ico">🔍</span>
        <input class="input" id="pkg-search" placeholder="搜索软件包名称或描述…" value="${esc(keyword)}">
      </div>
      <select class="select" id="pkg-sort" style="width:140px">
        <option value="updated"${sort === 'updated' ? ' selected' : ''}>最近更新</option>
        <option value="created"${sort === 'created' ? ' selected' : ''}>最新创建</option>
        <option value="downloads"${sort === 'downloads' ? ' selected' : ''}>下载最多</option>
        <option value="size"${sort === 'size' ? ' selected' : ''}>体积最大</option>
        <option value="name"${sort === 'name' ? ' selected' : ''}>名称排序</option>
      </select>
      <div class="segmented" title="切换展示方式">
        <button data-action="set-view" data-view="card" class="${view === 'card' ? 'active' : ''}">▦ 卡片</button>
        <button data-action="set-view" data-view="list" class="${view === 'list' ? 'active' : ''}">☰ 列表</button>
      </div>
      ${isAdmin() ? '<button class="btn btn-primary" data-action="new-package">＋ 新建软件包</button>' : ''}
    </div>
    <div class="chips mb-16">
      <button class="chip ${category === 0 ? 'active' : ''}" data-action="filter-cat" data-id="0">全部</button>
      ${state.categories.map((c) => `<button class="chip ${category === c.id ? 'active' : ''}" data-action="filter-cat" data-id="${c.id}">${esc(c.name)}<span class="cnt">${c.package_count}</span></button>`).join('')}
      ${state.categories.length ? `<button class="chip ${category === -1 ? 'active' : ''}" data-action="filter-cat" data-id="-1">未分类</button>` : ''}
    </div>
    <div id="pkg-result">${pkgResultHTML()}</div>`;

  let timer = null;
  $('#pkg-search').addEventListener('input', (e) => {
    clearTimeout(timer);
    const v = e.target.value;
    timer = setTimeout(() => { state.pkgs.keyword = v; state.pkgs.page = 1; route(); setTimeout(() => { const el = $('#pkg-search'); if (el) { el.focus(); el.setSelectionRange(v.length, v.length); } }, 30); }, 320);
  });
  $('#pkg-sort').addEventListener('change', (e) => { state.pkgs.sort = e.target.value; state.pkgs.page = 1; route(); });
}

// pkgResultHTML 依据当前视图模式渲染结果区（卡片 / 列表）。
function pkgResultHTML() {
  const data = state.pkgs.cache;
  if (!data) return '';
  const { page, pageSize, keyword, view } = state.pkgs;
  if (!data.items.length) {
    const hint = keyword ? '试试其他关键词' : (isAdmin() ? '点击右上角新建一个软件包' : '请联系管理员创建软件包');
    return emptyBox('📦', '没有找到软件包', hint);
  }
  const pages = Math.max(1, Math.ceil(data.total / pageSize));
  const body = view === 'list'
    ? `<div class="card pkg-table-wrap"><div style="overflow-x:auto">
         <table class="table pkg-table">
           <thead><tr>
             <th>软件包</th><th>分类</th><th>最新版本</th>
             <th class="right">版本</th><th class="right">下载</th><th class="right">体积</th>
             <th>负责人</th><th>更新时间</th><th class="right">操作</th>
           </tr></thead>
           <tbody>${data.items.map(pkgRow).join('')}</tbody>
         </table>
       </div></div>`
    : `<div class="pkg-grid">${data.items.map(pkgCard).join('')}</div>`;
  return body + (pages > 1 ? pager(page, pages, data.total) : '');
}

// pkgRow 列表模式下的单行渲染。
function pkgRow(p) {
  const mark = String(p.name || '?').trim().charAt(0).toUpperCase();
  return `<tr data-action="open-package" data-id="${p.id}">
      <td>
        <div class="pkg-cell">
          <span class="thumb" style="background:${esc(p.category_color)}">${esc(mark)}</span>
          <div style="min-width:0">
            <div class="name">${esc(p.name)}</div>
            <div class="sub" title="${esc(p.description || '')}">${esc(p.description || '暂无描述')}</div>
          </div>
        </div>
      </td>
      <td><span class="badge" style="background:${esc(p.category_color)}22;color:${esc(p.category_color)}">
        <span class="badge-dot" style="background:${esc(p.category_color)}"></span>${esc(p.category_name)}</span></td>
      <td>${p.latest_version ? `<span class="badge" style="background:var(--brand-soft);color:var(--brand)">${esc(p.latest_version)}</span>` : '<span class="muted">无版本</span>'}</td>
      <td class="right num">${p.version_count}</td>
      <td class="right num">${p.downloads}</td>
      <td class="right num nowrap">${fmtBytes(p.total_size)}</td>
      <td class="nowrap">${esc(p.owner_name)}</td>
      <td class="nowrap muted">${fmtAgo(p.updated_at)}</td>
      <td><div class="row-actions">
        ${p.latest_version ? `<button class="btn btn-sm" data-action="download-latest" data-id="${p.id}">下载</button>` : ''}
        <button class="btn btn-sm" data-action="open-package" data-id="${p.id}">详情</button>
      </div></td>
    </tr>`;
}

function pkgCard(p) {
  return `<article class="pkg-card" data-action="open-package" data-id="${p.id}">
      <div class="name">${esc(p.name)}
        <span class="badge" style="background:${esc(p.category_color)}22;color:${esc(p.category_color)}">
          <span class="badge-dot" style="background:${esc(p.category_color)}"></span>${esc(p.category_name)}
        </span>
      </div>
      <div class="desc">${esc(p.description || '暂无描述')}</div>
      <div class="pkg-meta">
        <span class="item">🏷️ ${p.latest_version ? esc(p.latest_version) : '无版本'}</span>
        <span class="item">⬇️ ${p.downloads}</span>
        <span class="item">💾 ${fmtBytes(p.total_size)}</span>
      </div>
      <div class="pkg-foot">
        <span class="small muted">${p.version_count} 个版本 · ${esc(p.owner_name)}</span>
        <span class="small muted">${fmtAgo(p.updated_at)}</span>
      </div>
    </article>`;
}

function pager(page, pages, total) {
  const btns = [];
  const from = Math.max(1, page - 2), to = Math.min(pages, from + 4);
  for (let i = from; i <= to; i++) {
    btns.push(`<button class="btn btn-sm ${i === page ? 'btn-primary' : ''}" data-action="page" data-page="${i}">${i}</button>`);
  }
  return `<div class="pager">
      <span class="info">共 ${total} 条 / ${pages} 页</span>
      <button class="btn btn-sm" data-action="page" data-page="${Math.max(1, page - 1)}" ${page <= 1 ? 'disabled' : ''}>上一页</button>
      ${btns.join('')}
      <button class="btn btn-sm" data-action="page" data-page="${Math.min(pages, page + 1)}" ${page >= pages ? 'disabled' : ''}>下一页</button>
    </div>`;
}

/* ---------------- 视图：软件包详情 ---------------- */

async function viewPackageDetail(id) {
  const data = await api('/api/packages/' + encodeURIComponent(id));
  const p = data.package;
  const versions = data.versions || [];
  const me = state.me;
  const canManage = me.role === 'admin' || p.owner_id === me.id;
  setTitle(p.name, '软件包详情');

  $('#view').innerHTML = `
    <div class="mb-16"><button class="btn btn-sm" data-action="nav" data-hash="#/packages">← 返回列表</button></div>
    <section class="card card-pad mb-16">
      <div class="detail-head">
        <div>
          <h2 class="title">${esc(p.name)}</h2>
          <div class="flex" style="gap:8px;flex-wrap:wrap">
            <span class="badge" style="background:${esc(p.category_color)}22;color:${esc(p.category_color)}">
              <span class="badge-dot" style="background:${esc(p.category_color)}"></span>${esc(p.category_name)}
            </span>
            ${p.latest_version ? `<span class="badge">最新版本 ${esc(p.latest_version)}</span>` : '<span class="badge badge-off">暂无版本</span>'}
          </div>
          <div class="desc" style="margin-top:8px">${esc(p.description || '暂无描述')}</div>
        </div>
        <div class="actions">
          ${p.latest_version ? `<button class="btn btn-primary" data-action="download-latest" data-id="${p.id}">⬇️ 下载最新版</button>` : ''}
          ${isAdmin() ? `<button class="btn" data-action="edit-package" data-id="${p.id}">✏️ 编辑</button>` : ''}
          ${canManage ? `<button class="btn btn-danger" data-action="delete-package" data-id="${p.id}">🗑️ 删除</button>` : ''}
        </div>
      </div>
      <div class="kv-grid">
        <div class="kv"><div class="k">版本数量</div><div class="v">${p.version_count}</div></div>
        <div class="kv"><div class="k">下载次数</div><div class="v">${p.downloads}</div></div>
        <div class="kv"><div class="k">占用空间</div><div class="v">${fmtBytes(p.total_size)}</div></div>
        <div class="kv"><div class="k">负责人</div><div class="v" style="font-size:13px">${esc(p.owner_name)}</div></div>
        <div class="kv"><div class="k">创建时间</div><div class="v" style="font-size:13px">${fmtTime(p.created_at)}</div></div>
      </div>
    </section>

    ${canManage ? `
    <section class="card card-pad mb-16">
      <div class="card-head" style="padding:0 0 12px;border:none;margin-bottom:6px">
        <h3>上传新版本</h3><span class="sub">同一软件包可上传多个版本，单个文件不超过 ${MAX_MB} MB</span>
      </div>
      <div class="grid-2">
        <div class="field" style="margin-bottom:0">
          <label>版本号</label>
          <input class="input" id="v-version" placeholder="例如 1.2.0">
        </div>
        <div class="field" style="margin-bottom:0">
          <label>更新说明（可选）</label>
          <input class="input" id="v-notes" placeholder="例如 修复已知问题">
        </div>
      </div>
      <div class="drop-zone mt-16" id="drop-zone" data-action="pick-file">
        <div class="dz-ico">📤</div>
        <div class="dz-title">点击选择文件，或将安装包拖拽到此处</div>
        <div class="dz-sub">支持任意格式，单个文件最大 ${MAX_MB} MB</div>
      </div>
      <input type="file" id="file-input" hidden>
      <div id="file-pill"></div>
      <div class="progress" id="up-progress"><div></div></div>
      <div class="flex mt-16" style="justify-content:flex-end">
        <button class="btn btn-primary" data-action="upload-version" data-id="${p.id}">开始上传</button>
      </div>
    </section>` : `<section class="card card-pad mb-16 muted small">当前账号不是该软件包的负责人，仅可下载，不能上传或删除版本。</section>`}

    <section class="card">
      <div class="card-head"><h3>版本列表</h3><span class="sub">共 ${versions.length} 个版本</span></div>
      ${versions.length ? `
      <div style="overflow-x:auto">
        <table class="table">
          <thead><tr>
            <th>版本</th><th>文件名</th><th>大小</th><th>SHA256</th><th class="right">下载</th><th>上传者</th><th>上传时间</th><th class="right">操作</th>
          </tr></thead>
          <tbody>
            ${versions.map((v) => `<tr>
              <td><span class="badge" style="background:var(--brand-soft);color:var(--brand)">${esc(v.version)}</span></td>
              <td title="${esc(v.file_name)}" style="max-width:220px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap">${esc(v.file_name)}</td>
              <td class="num">${fmtBytes(v.size)}</td>
              <td class="mono muted" title="${esc(v.sha256)}">${esc((v.sha256 || '').slice(0, 12))}</td>
              <td class="right num">${v.downloads}</td>
              <td>${esc(v.uploader_name)}</td>
              <td class="nowrap">${fmtTime(v.created_at)}</td>
              <td><div class="row-actions">
                <button class="btn btn-sm" data-action="download-version" data-id="${v.id}">下载</button>
                <button class="btn btn-sm" data-action="copy-link" data-id="${v.id}" title="复制下载链接">链接</button>
                ${canManage ? `<button class="btn btn-sm btn-danger" data-action="delete-version" data-id="${v.id}" data-pkg="${p.id}">删除</button>` : ''}
              </div></td>
            </tr>`).join('')}
          </tbody>
        </table>
      </div>` : `<div class="empty"><div class="big">🏷️</div><div class="t">该软件包还没有版本</div><div>上传第一个版本后即可提供下载</div></div>`}
      ${versions.length ? `<div class="card-pad small muted">${esc(versions[0].notes || '')}</div>` : ''}
    </section>`;

  const input = $('#file-input');
  const zone = $('#drop-zone');
  if (zone && input) {
    zone.addEventListener('click', () => input.click());
    input.addEventListener('change', (e) => setUploadFile(e.target.files[0]));
  }
}

function setUploadFile(file) {
  state.uploadFile = file || null;
  const box = $('#file-pill');
  if (!box) return;
  box.innerHTML = file ? `<div class="file-pill">
      <span>📄</span>
      <span class="n">${esc(file.name)}</span>
      <span class="sz">${fmtBytes(file.size)}</span>
      <button class="btn btn-sm btn-ghost" data-action="clear-file">✕</button>
    </div>` : '';
}

function renderProgress(ratio) {
  const p = $('#up-progress');
  if (!p) return;
  p.classList.add('show');
  p.firstElementChild.style.width = Math.round(ratio * 100) + '%';
}

/* ---------------- 视图：分类管理 ---------------- */

async function viewCategories() {
  setTitle('分类管理', '维护软件包的分类体系');
  const list = await api('/api/categories');
  state.categories = list;
  $('#view').innerHTML = `
    <div class="toolbar">
      <div class="grow"><div class="small muted">共 ${list.length} 个分类，被使用的分类无法直接删除。</div></div>
      <button class="btn btn-primary" data-action="new-category">＋ 新建分类</button>
    </div>
    <section class="card">
      ${list.length ? `<div style="overflow-x:auto"><table class="table">
        <thead><tr><th>分类</th><th>标识</th><th>说明</th><th class="right">软件包</th><th>创建时间</th><th class="right">操作</th></tr></thead>
        <tbody>${list.map((c) => `<tr>
          <td><span class="badge" style="background:${esc(c.color)}22;color:${esc(c.color)}"><span class="badge-dot" style="background:${esc(c.color)}"></span>${esc(c.name)}</span></td>
          <td class="mono muted">${esc(c.slug)}</td>
          <td class="muted">${esc(c.description || '—')}</td>
          <td class="right num">${c.package_count}</td>
          <td class="nowrap">${fmtTime(c.created_at)}</td>
          <td><div class="row-actions">
            <button class="btn btn-sm" data-action="edit-category" data-id="${c.id}">编辑</button>
            <button class="btn btn-sm btn-danger" data-action="delete-category" data-id="${c.id}">删除</button>
          </div></td>
        </tr>`).join('')}</tbody>
      </table></div>` : emptyBox('🗂️', '还没有分类', '创建分类后即可对软件包归类')}
    </section>`;
  renderNavCategories();
}

/* ---------------- 视图：用户管理 ---------------- */

async function viewUsers() {
  setTitle('用户管理', '账号、角色与权限');
  const list = await api('/api/users');
  $('#view').innerHTML = `
    <div class="toolbar">
      <div class="grow"><div class="small muted">共 ${list.length} 个账号；管理员可管理分类与全部软件包，普通用户仅能管理自己创建的软件包。</div></div>
      <button class="btn btn-primary" data-action="new-user">＋ 新建用户</button>
    </div>
    <section class="card">
      <div style="overflow-x:auto"><table class="table">
        <thead><tr><th>用户</th><th>用户名</th><th>角色</th><th>状态</th><th>创建时间</th><th>最近登录</th><th class="right">操作</th></tr></thead>
        <tbody>${list.map((u) => `<tr>
          <td><div class="flex" style="gap:9px">
            <span class="avatar" style="width:30px;height:30px;font-size:12px">${esc(String(u.display_name || u.username).charAt(0).toUpperCase())}</span>
            <div><div style="font-weight:600">${esc(u.display_name || u.username)}</div>
            <div class="small muted">ID ${u.id}</div></div></div></td>
          <td class="mono">${esc(u.username)}</td>
          <td><span class="badge ${u.role === 'admin' ? 'badge-admin' : 'badge-user'}">${u.role === 'admin' ? '管理员' : '普通用户'}</span></td>
          <td><span class="badge ${u.status === 1 ? 'badge-on' : 'badge-off'}"><span class="badge-dot"></span>${u.status === 1 ? '启用' : '已禁用'}</span></td>
          <td class="nowrap">${fmtTime(u.created_at)}</td>
          <td class="nowrap">${u.last_login_at ? fmtAgo(u.last_login_at) : '从未登录'}</td>
          <td><div class="row-actions">
            <button class="btn btn-sm" data-action="edit-user" data-id="${u.id}">编辑</button>
            <button class="btn btn-sm" data-action="reset-password" data-id="${u.id}">重置密码</button>
            ${u.id === state.me.id ? '' : `<button class="btn btn-sm btn-danger" data-action="delete-user" data-id="${u.id}">删除</button>`}
          </div></td>
        </tr>`).join('')}</tbody>
      </table></div>
    </section>`;
}

/* ---------------- 视图：个人设置 ---------------- */

async function viewAccount() {
  setTitle('个人设置', '账号信息与系统信息');
  const me = state.me;
  $('#view').innerHTML = `
    <div style="display:grid;grid-template-columns:1fr 1fr;gap:16px;align-items:start">
      <section class="card card-pad">
        <div class="card-head" style="padding:0 0 12px;border:none"><h3>基本信息</h3></div>
        <div class="field">
          <label>用户名</label>
          <input class="input" value="${esc(me.username)}" disabled>
          <div class="hint">用户名不可修改</div>
        </div>
        <div class="field">
          <label>显示名称</label>
          <input class="input" id="acc-name" value="${esc(me.display_name || '')}" placeholder="用于界面展示">
        </div>
        <div class="field">
          <label>角色</label>
          <input class="input" value="${me.role === 'admin' ? '管理员' : '普通用户'}" disabled>
        </div>
        <button class="btn btn-primary" data-action="save-profile">保存资料</button>
      </section>
      <section class="card card-pad">
        <div class="card-head" style="padding:0 0 12px;border:none"><h3>修改密码</h3></div>
        <div class="field">
          <label>当前密码</label>
          <input class="input" id="acc-old" type="password" placeholder="请输入当前密码">
        </div>
        <div class="field">
          <label>新密码</label>
          <input class="input" id="acc-new" type="password" placeholder="至少 6 位">
        </div>
        <div class="field">
          <label>确认新密码</label>
          <input class="input" id="acc-new2" type="password" placeholder="再次输入新密码">
        </div>
        <button class="btn btn-primary" data-action="change-password">修改密码</button>
        <div class="hint" style="margin-top:10px">修改成功后需要重新登录。</div>
      </section>
      <section class="card card-pad" style="grid-column:1 / -1">
        <div class="card-head" style="padding:0 0 12px;border:none"><h3>系统信息</h3></div>
        <div class="kv-grid">
          <div class="kv"><div class="k">程序版本</div><div class="v">v${esc(VERSION)}</div></div>
          <div class="kv"><div class="k">上传上限</div><div class="v">${MAX_MB} MB</div></div>
          <div class="kv"><div class="k">数据目录</div><div class="v" style="font-size:12px;word-break:break-all">${esc((state.meta && state.meta.data_dir) || '—')}</div></div>
          <div class="kv"><div class="k">软件存储目录</div><div class="v" style="font-size:12px;word-break:break-all">${esc((state.meta && state.meta.storage_dir) || '—')}</div></div>
        </div>
      </section>
    </div>`;
}

/* ---------------- 交互动作 ---------------- */

const ACTIONS = {
  nav(el) {
    const hash = el.dataset.hash;
    if (el.dataset.id && hash === '#/packages') { state.pkgs.category = 0; }
    if (hash.indexOf('#/packages') === 0) { state.pkgs.page = 1; }
    if (location.hash === hash) route(); else location.hash = hash;
  },
  'filter-cat'(el) {
    const id = parseInt(el.dataset.id, 10) || 0;
    state.pkgs.category = id === -1 ? -1 : id;
    state.pkgs.page = 1;
    const r = currentRoute();
    // 仅当已经处于「软件包列表页」时才就地重渲染；
    // 详情页（#/packages/{id}）或其它页面需要先跳转回列表页。
    if (r.name === 'packages' && !r.param) {
      route();
      if (window.scrollTo) window.scrollTo({ top: 0 });
    } else {
      location.hash = '#/packages';
    }
  },
  page(el) { state.pkgs.page = parseInt(el.dataset.page, 10) || 1; route(); },
  // 卡片 / 列表视图切换：仅重渲染结果区，不重新请求数据
  'set-view'(el) {
    const v = el.dataset.view === 'list' ? 'list' : 'card';
    if (state.pkgs.view === v) return;
    state.pkgs.view = v;
    try { localStorage.setItem('pkghub-pkg-view', v); } catch (e) { /* 忽略存储失败 */ }
    $$('[data-action="set-view"]').forEach((b) => b.classList.toggle('active', b.dataset.view === v));
    const box = $('#pkg-result');
    if (box) box.innerHTML = pkgResultHTML();
  },
  'open-package'(el) { location.hash = '#/packages/' + el.dataset.id; },
  'pick-file'() { const i = $('#file-input'); if (i) i.click(); },
  'clear-file'() { const i = $('#file-input'); if (i) i.value = ''; setUploadFile(null); },
  async 'upload-version'(el) {
    const file = state.uploadFile;
    const version = val(document, '#v-version');
    if (!file) return toast('请选择文件', '点击上传区域选择安装包', 'error');
    if (!version) return toast('请填写版本号', '例如 1.0.0', 'error');
    const fd = new FormData();
    fd.append('file', file);
    fd.append('version', version);
    fd.append('notes', val(document, '#v-notes'));
    try {
      await uploadFile('/api/packages/' + el.dataset.id + '/versions', fd, (loaded, total) => renderProgress(loaded / total));
      toast('上传成功', version + ' 已发布', 'success');
      route();
    } catch (err) {
      toast('上传失败', err.message, 'error');
    }
  },
  'download-latest'(el) { startDownload('/api/packages/' + el.dataset.id + '/download'); },
  'download-version'(el) { startDownload('/api/versions/' + el.dataset.id + '/download'); },
  async 'copy-link'(el) {
    const link = location.origin + '/api/versions/' + el.dataset.id + '/download';
    try {
      await navigator.clipboard.writeText(link);
      toast('已复制下载链接', link, 'success');
    } catch (e) { toast('复制失败', link, 'error'); }
  },
  async 'delete-version'(el) {
    const okd = await confirmBox('删除版本', '删除后文件将从服务器移除，且无法恢复。确定继续吗？', '删除', true);
    if (!okd) return;
    try {
      await api('/api/versions/' + el.dataset.id, { method: 'DELETE' });
      toast('已删除', '版本文件已移除', 'success');
      route();
    } catch (err) { toast('删除失败', err.message, 'error'); }
  },
  'new-package'() {
    if (!requireAdmin('新增软件包')) return;
    packageDialog();
  },
  'edit-package'(el) {
    if (!requireAdmin('编辑软件包')) return;
    packageDialog(el.dataset.id);
  },
  async 'delete-package'(el) {
    const okd = await confirmBox('删除软件包', '将同时删除该软件包下的所有版本文件，确定继续吗？', '删除', true);
    if (!okd) return;
    try {
      await api('/api/packages/' + el.dataset.id, { method: 'DELETE' });
      toast('已删除', '软件包及其版本已移除', 'success');
      location.hash = '#/packages';
      route();
    } catch (err) { toast('删除失败', err.message, 'error'); }
  },
  'new-category'() { categoryDialog(); },
  'edit-category'(el) { categoryDialog(el.dataset.id); },
  async 'delete-category'(el) {
    const okd = await confirmBox('删除分类', '分类下存在软件包时无法删除。确定继续吗？', '删除', true);
    if (!okd) return;
    try {
      await api('/api/categories/' + el.dataset.id, { method: 'DELETE' });
      toast('已删除', '分类已移除', 'success');
      await loadCategories();
      renderNavCategories();
      route();
    } catch (err) { toast('删除失败', err.message, 'error'); }
  },
  'new-user'() { userDialog(); },
  'edit-user'(el) { userDialog(el.dataset.id); },
  'reset-password'(el) {
    const id = el.dataset.id;
    modal({
      title: '重置密码',
      body: `<div class="field"><label>新密码</label><input class="input" id="np" type="password" placeholder="至少 6 位"></div>
             <div class="hint">重置后该用户的登录会话将失效。</div>`,
      okText: '重置',
      onOk: async (m) => {
        const pwd = val(m, '#np');
        if (pwd.length < 6) { toast('密码太短', '至少需要 6 位', 'error'); return false; }
        await api('/api/users/' + id + '/password', { method: 'POST', body: { password: pwd } });
        toast('已重置', '用户密码已更新', 'success');
      },
    });
  },
  async 'delete-user'(el) {
    const okd = await confirmBox('删除用户', '删除后该账号将无法登录，其创建的软件包会保留。确定继续吗？', '删除', true);
    if (!okd) return;
    try {
      await api('/api/users/' + el.dataset.id, { method: 'DELETE' });
      toast('已删除', '用户账号已移除', 'success');
      route();
    } catch (err) { toast('删除失败', err.message, 'error'); }
  },
  async 'save-profile'() {
    const name = val(document, '#acc-name');
    if (!name) return toast('请填写显示名称', '', 'error');
    try {
      const u = await api('/api/account/profile', { method: 'PUT', body: { display_name: name } });
      state.me = u;
      toast('已保存', '资料更新成功', 'success');
      renderShell();
      route();
    } catch (err) { toast('保存失败', err.message, 'error'); }
  },
  async 'change-password'() {
    const oldP = val(document, '#acc-old');
    const np = val(document, '#acc-new');
    const np2 = val(document, '#acc-new2');
    if (np.length < 6) return toast('密码太短', '新密码至少需要 6 位', 'error');
    if (np !== np2) return toast('两次输入不一致', '请重新输入新密码', 'error');
    try {
      await api('/api/account/password', { method: 'POST', body: { old_password: oldP, new_password: np } });
      toast('密码已修改', '请使用新密码重新登录', 'success');
      state.me = null;
      setTimeout(() => renderLogin('密码已修改，请使用新密码登录'), 600);
    } catch (err) { toast('修改失败', err.message, 'error'); }
  },
  async logout() {
    try { await api('/api/auth/logout', { method: 'POST' }); } catch (e) { /* 忽略 */ }
    state.me = null;
    renderLogin('已退出登录');
  },
};

function startDownload(url) {
  const a = document.createElement('a');
  a.href = url;
  a.rel = 'noopener';
  document.body.appendChild(a);
  a.click();
  a.remove();
  setTimeout(() => { if (currentRoute().name === 'packages') route(); }, 1500);
}

/* ---------------- 表单弹窗 ---------------- */

const COLORS = ['#6366f1', '#8b5cf6', '#06b6d4', '#10b981', '#f59e0b', '#ef4444', '#ec4899', '#64748b'];

function colorField(selected) {
  return `<div class="field"><label>主题色</label>
    <div class="flex" style="gap:8px;flex-wrap:wrap">
      ${COLORS.map((c) => `<button type="button" class="chip" data-color="${c}" style="background:${c}22;color:${c};border-color:${c === selected ? c : 'transparent'};padding:6px 10px">${c === selected ? '✓ ' : ''}●</button>`).join('')}
    </div>
    <input type="hidden" id="f-color" value="${esc(selected || COLORS[0])}">
  </div>`;
}

function bindColors(mask) {
  $$('[data-color]', mask).forEach((btn) => {
    btn.addEventListener('click', () => {
      const c = btn.dataset.color;
      $('#f-color', mask).value = c;
      $$('[data-color]', mask).forEach((b) => {
        const on = b.dataset.color === c;
        b.textContent = (on ? '✓ ' : '') + '●';
        b.style.borderColor = on ? c : 'transparent';
      });
    });
  });
}

async function packageDialog(id) {
  if (!requireAdmin(id ? '编辑软件包' : '新增软件包')) return;
  let pkg = null;
  if (id) pkg = await api('/api/packages/' + id).then((d) => d.package);
  if (!state.categories.length) {
    toast('请先创建分类', '分类管理中可以新建分类', 'error');
    if (state.me.role === 'admin') location.hash = '#/categories';
    return;
  }
  modal({
    title: pkg ? '编辑软件包' : '新建软件包',
    body: `
      <div class="field"><label>名称</label><input class="input" id="f-name" value="${esc(pkg ? pkg.name : '')}" placeholder="例如 客户端安装包"></div>
      <div class="field"><label>分类</label><select class="select" id="f-cat">
        ${state.categories.map((c) => `<option value="${c.id}"${pkg && pkg.category_id === c.id ? ' selected' : ''}>${esc(c.name)}</option>`).join('')}
      </select></div>
      <div class="field"><label>描述</label><textarea class="textarea" id="f-desc" placeholder="用途、适用范围等">${esc(pkg ? pkg.description : '')}</textarea></div>`,
    onOk: async (m) => {
      const body = {
        name: val(m, '#f-name'),
        description: val(m, '#f-desc'),
        category_id: parseInt(val(m, '#f-cat'), 10) || 0,
      };
      if (!body.name) { toast('请填写名称', '', 'error'); return false; }
      if (pkg) await api('/api/packages/' + pkg.id, { method: 'PUT', body });
      else await api('/api/packages', { method: 'POST', body });
      toast(pkg ? '已保存' : '已创建', body.name, 'success');
      await loadCategories();
      renderNavCategories();
      route();
    },
  });
}

async function categoryDialog(id) {
  let cat = null;
  if (id) {
    const list = await api('/api/categories');
    cat = list.find((c) => String(c.id) === String(id));
  }
  modal({
    title: cat ? '编辑分类' : '新建分类',
    body: `
      <div class="field"><label>名称</label><input class="input" id="f-name" value="${esc(cat ? cat.name : '')}" placeholder="例如 基础软件"></div>
      <div class="field"><label>说明</label><input class="input" id="f-desc" value="${esc(cat ? cat.description : '')}" placeholder="可选"></div>
      ${colorField(cat ? cat.color : COLORS[0])}`,
    onOk: async (m) => {
      const body = {
        name: val(m, '#f-name'),
        description: val(m, '#f-desc'),
        color: val(m, '#f-color'),
      };
      if (!body.name) { toast('请填写名称', '', 'error'); return false; }
      if (cat) await api('/api/categories/' + cat.id, { method: 'PUT', body });
      else await api('/api/categories', { method: 'POST', body });
      toast(cat ? '已保存' : '已创建', body.name, 'success');
      await loadCategories();
      renderNavCategories();
      route();
    },
  });
  bindColors($('#modal-root .modal-mask:last-child'));
}

async function userDialog(id) {
  let user = null;
  if (id) {
    const list = await api('/api/users');
    user = list.find((u) => String(u.id) === String(id));
  }
  modal({
    title: user ? '编辑用户' : '新建用户',
    body: `
      <div class="grid-2">
        <div class="field"><label>用户名</label><input class="input" id="f-username" value="${esc(user ? user.username : '')}" ${user ? 'disabled' : ''} placeholder="登录账号"></div>
        <div class="field"><label>显示名称</label><input class="input" id="f-name" value="${esc(user ? user.display_name : '')}" placeholder="可选"></div>
      </div>
      ${user ? '' : `<div class="field"><label>初始密码</label><input class="input" id="f-password" type="password" placeholder="至少 6 位"></div>`}
      <div class="grid-2">
        <div class="field"><label>角色</label><select class="select" id="f-role">
          <option value="user"${user && user.role === 'user' ? ' selected' : ''}>普通用户</option>
          <option value="admin"${user && user.role === 'admin' ? ' selected' : ''}>管理员</option>
        </select></div>
        <div class="field"><label>状态</label><select class="select" id="f-status">
          <option value="1"${!user || user.status === 1 ? ' selected' : ''}>启用</option>
          <option value="0"${user && user.status === 0 ? ' selected' : ''}>禁用</option>
        </select></div>
      </div>`,
    onOk: async (m) => {
      if (user) {
        await api('/api/users/' + user.id, {
          method: 'PUT',
          body: {
            display_name: val(m, '#f-name') || user.username,
            role: val(m, '#f-role'),
            status: parseInt(val(m, '#f-status'), 10),
          },
        });
        toast('已保存', user.username, 'success');
      } else {
        const pwd = val(m, '#f-password');
        if (pwd.length < 6) { toast('密码太短', '至少需要 6 位', 'error'); return false; }
        await api('/api/users', {
          method: 'POST',
          body: {
            username: val(m, '#f-username'),
            display_name: val(m, '#f-name'),
            password: pwd,
            role: val(m, '#f-role'),
          },
        });
        toast('已创建', val(m, '#f-username'), 'success');
      }
      route();
    },
  });
}

/* ---------------- 事件绑定与启动 ---------------- */

app.addEventListener('click', (e) => {
  const el = e.target.closest('[data-action]');
  if (!el) return;
  const fn = ACTIONS[el.dataset.action];
  if (!fn) return;
  e.preventDefault();
  e.stopPropagation();
  Promise.resolve(fn(el)).catch((err) => toast('操作失败', err.message, 'error'));
});

['dragenter', 'dragover'].forEach((evt) => {
  app.addEventListener(evt, (e) => {
    const zone = e.target.closest && e.target.closest('#drop-zone');
    if (zone) { e.preventDefault(); zone.classList.add('drag'); }
  });
});
['dragleave', 'drop'].forEach((evt) => {
  app.addEventListener(evt, (e) => {
    const zone = e.target.closest && e.target.closest('#drop-zone');
    if (!zone) return;
    e.preventDefault();
    zone.classList.remove('drag');
    if (evt === 'drop' && e.dataTransfer && e.dataTransfer.files.length) setUploadFile(e.dataTransfer.files[0]);
  });
});

window.addEventListener('hashchange', () => { if (state.me) route(); });

async function boot() {
  try {
    const data = await api('/api/auth/me');
    state.me = data.user;
    state.meta = data;
    await afterLogin();
  } catch (e) {
    renderLogin();
  }
}

boot();
