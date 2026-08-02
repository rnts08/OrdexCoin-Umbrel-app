/* OrdexCoin Web UI — frontend logic (vanilla JS, no dependencies). */

"use strict";

/* ---------------- helpers ---------------- */

const $ = (sel) => document.querySelector(sel);
const $$ = (sel) => Array.from(document.querySelectorAll(sel));

const esc = (s) =>
  String(s == null ? "" : s).replace(/[&<>"']/g, (c) =>
    ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" }[c]));

function fmtOXC(n) {
  if (n == null || isNaN(n)) return "—";
  return n.toLocaleString("en-US", { minimumFractionDigits: 0, maximumFractionDigits: 8 });
}

function fmtBytes(n) {
  if (n == null || isNaN(n)) return "—";
  if (n >= 1e9) return (n / 1e9).toFixed(2) + " GB";
  if (n >= 1e6) return (n / 1e6).toFixed(1) + " MB";
  if (n >= 1e3) return (n / 1e3).toFixed(1) + " KB";
  return n + " B";
}

function fmtHash(n) {
  if (n == null || isNaN(n)) return "—";
  const units = ["H/s", "KH/s", "MH/s", "GH/s", "TH/s", "PH/s", "EH/s"];
  let i = 0;
  while (n >= 1000 && i < units.length - 1) { n /= 1000; i++; }
  return n.toFixed(2) + " " + units[i];
}

function fmtPrice(n) {
  if (n == null || isNaN(n)) return "N/A";
  return "$" + Number(n).toFixed(6);
}

function fmtDate(ts) {
  if (!ts) return "—";
  return new Date(ts * 1000).toLocaleString();
}

function shortHash(h, n = 12) {
  if (!h) return "—";
  return h.length <= n ? h : h.slice(0, n) + "…";
}

function safeParseAmount(s) {
  const v = Number(String(s).trim());
  if (!isFinite(v)) return null;
  return v;
}

/* ---------------- API layer ---------------- */

async function api(path, opts = {}) {
  const resp = await fetch(path, {
    headers: { "Content-Type": "application/json" },
    ...opts,
  });
  let data = null;
  try {
    data = await resp.json();
  } catch (e) {
    data = null;
  }
  if (!resp.ok) {
    const msg = (data && data.error) ? data.error : "HTTP " + resp.status;
    throw new Error(msg);
  }
  return data;
}

const getJSON = (path) => api(path);
const postJSON = (path, body) => api(path, { method: "POST", body: JSON.stringify(body) });

/* ---------------- toasts & modal ---------------- */

function toast(msg, kind = "") {
  const el = document.createElement("div");
  el.className = "toast " + kind;
  el.textContent = msg;
  $("#toast-root").appendChild(el);
  setTimeout(() => {
    el.style.opacity = "0";
    el.style.transition = "opacity .4s";
    setTimeout(() => el.remove(), 400);
  }, 4200);
}

function confirmModal(title, rows, { confirmText = "Confirm", kind = "" } = {}) {
  return new Promise((resolve) => {
    const overlay = document.createElement("div");
    overlay.className = "modal-overlay";
    overlay.innerHTML = `
      <div class="modal">
        <h3>${esc(title)}</h3>
        ${rows.map((r) => `<div class="row"><span class="k">${esc(r.k)}</span><span class="v ${r.gold ? "gold" : ""}">${esc(r.v)}</span></div>`).join("")}
        <div class="modal-actions">
          <button class="btn btn-ghost" data-act="cancel">Cancel</button>
          <button class="btn ${kind === "danger" ? "btn-gold" : "btn-primary"}" data-act="ok">${esc(confirmText)}</button>
        </div>
      </div>`;
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) { cleanup(false); return; }
      const act = e.target.closest("[data-act]")?.dataset.act;
      if (act === "cancel") cleanup(false);
      else if (act === "ok") cleanup(true);
    });
    function cleanup(ok) {
      overlay.remove();
      resolve(ok);
    }
    $("#modal-root").appendChild(overlay);
  });
}

/* ---------------- clipboard ---------------- */

async function copyText(text, label = "Copied") {
  try {
    await navigator.clipboard.writeText(text);
    toast(label, "ok");
  } catch (e) {
    toast("Copy failed: " + e.message, "err");
  }
}

/* ---------------- state ---------------- */

const state = {
  info: null,
  wallet: localStorage.getItem("oxc-wallet") || "",
  statusTimer: null,
  slowTimer: null,
};

/* ---------------- tabs ---------------- */

function switchTab(name) {
  $$(".tab").forEach((t) => t.classList.toggle("active", t.dataset.tab === name));
  $$(".view").forEach((v) => v.classList.toggle("active", v.id === "view-" + name));
  if (name === "status") loadStatus();
  if (name === "balances") loadBalances();
  if (name === "transactions") loadTransactions();
  if (name === "addresses") loadAddresses();
  if (name === "send") loadFeeEstimate();
}

/* ---------------- node pill ---------------- */

function setNodePill(up) {
  const pill = $("#node-pill");
  pill.classList.toggle("pill-up", up);
  pill.classList.toggle("pill-down", !up);
  $("#node-pill-text").textContent = up ? "Online" : "Offline";
}

/* ---------------- wallets ---------------- */

async function loadWallets() {
  try {
    const data = await getJSON("/api/wallets");
    const sel = $("#wallet-select");
    sel.innerHTML = "";
    if (!data.wallets || data.wallets.length === 0) {
      sel.innerHTML = '<option value="">(no wallet loaded)</option>';
      $("#wallet-banner").hidden = false;
      $("#wallet-banner-text").textContent = "No wallet is loaded. Create a default wallet to start receiving and sending OXC, or load one via the RPC console (loadwallet \"<name>\").";
      $("#create-wallet-btn").hidden = false;
      state.wallet = "";
      return;
    }
    $("#wallet-banner").hidden = true;
    data.wallets.forEach((w) => {
      const opt = document.createElement("option");
      opt.value = w;
      opt.textContent = w;
      sel.appendChild(opt);
    });
    if (state.wallet && data.wallets.includes(state.wallet)) {
      sel.value = state.wallet;
    } else {
      state.wallet = data.wallets[0];
      sel.value = state.wallet;
    }
  } catch (e) {
    setNodePill(false);
  }
}

async function createWallet() {
  try {
    const res = await postJSON("/api/wallets/create", { name: "" });
    state.wallet = res.wallet;
    localStorage.setItem("oxc-wallet", state.wallet);
    toast("Wallet created: " + (res.wallet || "(default)"), "ok");
    await loadWallets();
    loadBalances();
    loadTransactions();
    loadAddresses();
  } catch (e) {
    toast(e.message, "err");
  }
}

async function switchWallet(name) {
  try {
    await postJSON("/api/wallet", { name });
    state.wallet = name;
    localStorage.setItem("oxc-wallet", name);
    toast("Wallet: " + name, "ok");
    loadBalances();
    loadTransactions();
    loadAddresses();
  } catch (e) {
    toast(e.message, "err");
  }
}

/* ---------------- status ---------------- */

const STATUS_CARDS = [
  { k: "chain", label: "Chain" },
  { k: "height", label: "Block height" },
  { k: "headers", label: "Headers" },
  { k: "difficulty", label: "Difficulty", fmt: (v) => Number(v).toLocaleString("en-US", { maximumFractionDigits: 0 }) },
  { k: "verificationprogress", label: "Verification", fmt: (v) => (v * 100).toFixed(4) + " %" },
  { k: "size_on_disk", label: "Blockchain size", fmt: fmtBytes },
  { k: "initialblockdownload", label: "Initial block download", fmt: (v) => (v ? "yes" : "no") },
  { k: "connections", label: "Connections", fmt: (v) => String(v) + " peers" },
  { k: "subversion", label: "Subversion" },
  { k: "version", label: "Version", fmt: (v) => "v" + (v / 10000).toFixed(2) },
  { k: "time", label: "Node time", fmt: fmtDate },
  { k: "mempoolbytes", label: "Mempool size", fmt: fmtBytes },
  { k: "mempooltx", label: "Mempool transactions" },
  { k: "poolHashrate", label: "Network hashrate", pool: true, fmt: fmtHash },
  { k: "poolBlockReward", label: "Block reward", pool: true },
];

function renderStatus(data) {
  const chain = data.chain || {};
  const network = data.network || {};
  const mempool = data.mempool || {};
  const pool = state.pool || {};
  const root = $("#status-cards");
  root.innerHTML = "";

  const map = {
    chain: chain.chain,
    height: chain.blocks,
    headers: chain.headers,
    difficulty: chain.difficulty,
    verificationprogress: chain.verificationprogress,
    size_on_disk: chain.size_on_disk,
    initialblockdownload: chain.initialblockdownload,
    connections: network.connections,
    subversion: network.subversion,
    version: network.version,
    time: chain.time,
    mempoolbytes: mempool.bytes,
    mempooltx: mempool.size,
    poolHashrate: pool.networkHashrate,
    poolBlockReward: pool.blockReward,
  };

  for (const c of STATUS_CARDS) {
    const v = map[c.k];
    if (v == null && !c.pool) continue;
    const card = document.createElement("div");
    card.className = "card";
    card.id = c.pool ? "pool-" + c.k : "";
    const display = v == null ? "—" : (c.fmt ? c.fmt(v) : v);
    card.innerHTML = `<div class="card-label">${esc(c.label)}</div>` +
      `<div class="card-value ${typeof display === "string" && display.length > 22 ? "small mono" : "small"}">${esc(display)}</div>`;
    root.appendChild(card);
  }

  const sync = $("#sync-banner");
  if (chain.initialblockdownload) {
    sync.hidden = false;
    sync.textContent = `Synchronizing… block ${chain.blocks} of ~${chain.headers} (${(chain.verificationprogress * 100).toFixed(2)}% verified).`;
  } else {
    sync.hidden = true;
  }

  setNodePill(true);
  if (chain.warnings) toast("Node warning: " + chain.warnings, "gold");
}

async function loadStatus() {
  try {
    const data = await getJSON("/api/status");
    renderStatus(data);
  } catch (e) {
    setNodePill(false);
  }
}

// Pool data (network hashrate, block reward, OXC price) is loaded from the
// backend proxy asynchronously. If the pool or price APIs are unreachable the
// cards simply keep their "—"/N/A placeholders; nothing here is critical.
async function loadPool() {
  let data = {};
  try {
    data = await getJSON("/api/pool");
  } catch (e) {
    data = {};
  }
  state.pool = data;
  updatePoolCards();
}

function updatePoolCards() {
  const pool = state.pool || {};
  const set = (id, v, fmt) => {
    const el = document.getElementById(id);
    if (el) el.textContent = fmt ? fmt(v) : (v == null ? "—" : v);
  };
  set("pool-poolHashrate", pool.networkHashrate, fmtHash);
  set("pool-poolBlockReward", pool.blockReward);
  set("bal-price", pool.priceUsd, fmtPrice);
}

/* ---------------- balances ---------------- */

async function loadBalances() {
  try {
    const data = await getJSON("/api/balances");
    $("#bal-available").textContent = fmtOXC(data.available);
    $("#bal-confirmed").textContent = fmtOXC(data.confirmed) + " OXC";
    $("#bal-unconfirmed").textContent = fmtOXC(data.unconfirmed) + " OXC";
    $("#bal-immature").textContent = fmtOXC(data.immature) + " OXC";
    $("#bal-utxos").textContent = data.utxoCount;
    setNodePill(true);
  } catch (e) {
    setNodePill(false);
    if (e.message.includes("No wallet")) {
      $("#wallet-banner").hidden = false;
      $("#wallet-banner-text").textContent = e.message;
      $("#create-wallet-btn").hidden = true;
    }
  }
}

/* ---------------- transactions ---------------- */

function txType(cat) {
  if (cat === "receive") return "in";
  if (cat === "send") return "out";
  if (cat === "self") return "self";
  return cat || "?";
}

function renderTransactions(txs) {
  const tbody = $("#tx-table tbody");
  const empty = $("#tx-empty");
  tbody.innerHTML = "";
  $("#tx-count").textContent = txs.length ? txs.length + " entries" : "";
  empty.hidden = txs.length > 0;

  for (const t of txs) {
    const type = txType(t.category);
    const amount = typeof t.amount === "number" ? t.amount : 0;
    const isOut = amount < 0;
    const cls = amount === 0 ? "amount-zero" : isOut ? "amount-out" : "amount-in";
    const conf = typeof t.confirmations === "number" ? t.confirmations : null;
    const confBadge = conf == null ? '<span class="badge">—</span>'
      : conf >= 1 ? `<span class="badge conf-ok">${conf} conf</span>`
      : '<span class="badge conf-pending">unconfirmed</span>';

    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td><span class="type-tag type-${esc(type)}">${esc(type)}</span></td>
      <td>${esc(fmtDate(t.time))}</td>
      <td class="mono-td">${esc(t.address || "—")}<span class="label-tag">${esc(t.label || "")}</span></td>
      <td class="num mono ${cls}">${isOut ? "-" : ""}${fmtOXC(Math.abs(amount))}</td>
      <td class="num">${confBadge}</td>
      <td class="mono-td">${esc(shortHash(t.txid))}
        <button class="copy-btn" data-copy="${esc(t.txid)}">copy</button>
      </td>`;
    tbody.appendChild(tr);
  }
  bindCopyButtons();
}

async function loadTransactions() {
  try {
    const txs = await getJSON("/api/transactions");
    renderTransactions(Array.isArray(txs) ? txs : []);
  } catch (e) {
    if (!e.message.includes("No wallet")) toast("Transactions: " + e.message, "err");
  }
}

/* ---------------- addresses ---------------- */

function renderAddresses(entries) {
  const tbody = $("#addr-table tbody");
  const empty = $("#addr-empty");
  tbody.innerHTML = "";
  empty.hidden = entries.length > 0;

  for (const e of entries) {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td>${esc(e.label || "(default)")}</td>
      <td class="mono-td">${esc(e.address)}
        <button class="copy-btn" data-copy="${esc(e.address)}">copy</button>
      </td>
      <td class="num mono">${fmtOXC(e.balance)}</td>
      <td class="num">${esc(e.txCount || 0)}</td>
      <td><button class="copy-btn" data-copy="${esc(e.address)}">copy addr</button></td>`;
    tbody.appendChild(tr);
  }
  bindCopyButtons();
}

async function loadAddresses() {
  try {
    const entries = await getJSON("/api/addresses");
    renderAddresses(Array.isArray(entries) ? entries : []);
  } catch (e) {
    if (!e.message.includes("No wallet")) toast("Addresses: " + e.message, "err");
  }
}

async function newAddress(label) {
  try {
    const res = await postJSON("/api/addresses/new", { label });
    const box = $("#new-address-result");
    box.hidden = false;
    box.textContent = "New address: " + res.address;
    $("#new-address-label").value = "";
    loadAddresses();
    copyText(res.address, "Address copied");
  } catch (e) {
    toast(e.message, "err");
  }
}

/* ---------------- send ---------------- */

async function loadFeeEstimate() {
  try {
    const data = await getJSON("/api/fee-estimate");
    $("#send-fee-line").textContent = data.available
      ? `Network fee estimate: ${data.satPerVbyte.toFixed(1)} sat/vB (≈ ${data.blocks} block${data.blocks === 1 ? "" : "s"})`
      : "Fee estimate not available yet.";
  } catch (e) {
    $("#send-fee-line").textContent = "Fee estimate unavailable.";
  }
}

function validateAddress(a) {
  return /^(oxc|bc1|tb1|bcrt|2|3|m|n)[0-9a-zA-Z]{10,}$/.test(a.trim());
}

async function submitSend() {
  const address = $("#send-address").value.trim();
  const amount = safeParseAmount($("#send-amount").value);
  const comment = $("#send-comment").value.trim();
  const subtractFee = $("#send-subtract-fee").checked;

  if (!validateAddress(address)) {
    toast("Please enter a valid address.", "err");
    return;
  }
  if (amount == null || amount <= 0) {
    toast("Please enter a valid amount.", "err");
    return;
  }

  const ok = await confirmModal("Confirm send", [
    { k: "To", v: address },
    { k: "Amount", v: fmtOXC(amount) + " OXC", gold: true },
    { k: "Fee", v: subtractFee ? "deducted from amount" : "added on top" },
  ], { confirmText: "Send OXC", kind: "danger" });

  if (!ok) return;

  try {
    const res = await postJSON("/api/send", {
      address, amount, comment, commentTo: "", subtractFee,
    });
    toast("Sent " + fmtOXC(amount) + " OXC. TXID: " + shortHash(res.txid, 20), "ok");
    $("#send-address").value = "";
    $("#send-amount").value = "";
    $("#send-comment").value = "";
    loadBalances();
    setTimeout(loadTransactions, 3000);
  } catch (e) {
    toast(e.message, "err");
  }
}

/* ---------------- console ---------------- */

const CONSOLE_SUGGESTIONS = [
  "getblockchaininfo", "getnetworkinfo", "getmempoolinfo", "getblockcount",
  "getbestblockhash", "getmininginfo", "uptime", "getpeerinfo", "getblockhash",
  "getblock", "decoderawtransaction", "estimatesmartfee",
  "getwalletinfo", "getbalance", "getbalances", "listunspent",
  "listtransactions", "listlabels", "getaddressesbylabel", "getaddressinfo",
  "getnewaddress", "getreceivedbyaddress", "gettransaction", "sendtoaddress",
  "bumpfee", "signmessage", "verifymessage", "listwallets", "loadwallet",
  "createwallet", "unloadwallet", "help", "getrpcinfo",
];

function initConsoleSuggestions() {
  const dl = $("#console-methods");
  dl.innerHTML = CONSOLE_SUGGESTIONS.map((m) => `<option value="${esc(m)}">`).join("");
}

function renderConsole(result, rpcError) {
  const out = $("#console-output");
  if (rpcError) {
    out.innerHTML = "";
    const err = document.createElement("div");
    err.className = "err";
    err.textContent = "error " + rpcError.code + ": " + rpcError.message;
    out.appendChild(err);
    return;
  }
  out.textContent = typeof result === "string"
    ? result
    : JSON.stringify(result, null, 2);
}

async function runConsole(method, paramsRaw) {
  let params;
  const s = paramsRaw.trim();
  if (s === "") params = "[]";
  else {
    try {
      params = JSON.stringify(JSON.parse(s));
    } catch (e) {
      renderConsole(null, { code: -1, message: "Invalid params JSON: " + e.message });
      return;
    }
  }
  try {
    const data = await postJSON("/api/console", { method, params });
    renderConsole(data.result, data.error);
  } catch (e) {
    renderConsole(null, { code: -1, message: e.message });
  }
}

/* ---------------- about / support / tip ---------------- */

function renderSupport(info) {
  const list = $("#support-addresses");
  list.innerHTML = "";
  for (const a of info.support || []) {
    const item = document.createElement("div");
    item.className = "support-item";
    item.innerHTML = `
      <span class="net">${esc(a.network)}</span>
      <span class="addr">${esc(a.address)}</span>
      <button class="copy-btn" data-copy="${esc(a.address)}">copy</button>`;
    list.appendChild(item);
  }
  bindCopyButtons();
}

function renderAbout(info) {
  $("#about-version").textContent = "v" + info.version;
  $("#about-runtime").textContent = "Node time, " + (navigator.userAgent.includes("Go") ? "standalone binary" : "web browser") + " — OrdexCoin Web " + info.version;
  renderSupport(info);
}

async function submitTip() {
  const amount = safeParseAmount($("#tip-amount").value);
  if (amount == null || amount <= 0) {
    toast("Please enter a valid tip amount.", "err");
    return;
  }
  const subtractFee = $("#tip-subtract-fee").checked;
  const ok = await confirmModal("Confirm tip", [
    { k: "To (developer)", v: "oxc1qcjav…w657e" },
    { k: "Amount", v: fmtOXC(amount) + " OXC", gold: true },
    { k: "Fee", v: subtractFee ? "deducted from amount" : "added on top" },
    { k: "Note", v: "This is a real, broadcast transaction." },
  ], { confirmText: "Send tip", kind: "danger" });
  if (!ok) return;

  try {
    const res = await postJSON("/api/tip", { amount, subtractFee });
    toast("Thank you! Tip sent. TXID: " + shortHash(res.txid, 20), "gold");
    $("#tip-amount").value = "";
    loadBalances();
  } catch (e) {
    toast(e.message, "err");
  }
}

/* ---------------- copy buttons ---------------- */

function bindCopyButtons() {
  $$(".copy-btn").forEach((btn) => {
    btn.removeEventListener("click", copyHandler);
    btn.addEventListener("click", copyHandler);
  });
}

function copyHandler(e) {
  e.stopPropagation();
  const v = e.target.dataset.copy;
  if (v) copyText(v);
}

/* ---------------- init & polling ---------------- */

function bindEvents() {
  $$(".tab").forEach((t) => t.addEventListener("click", () => switchTab(t.dataset.tab)));

  $("#refresh-btn").addEventListener("click", () => {
    loadStatus();
    loadPool();
    loadWallets().then(() => {
      loadBalances();
      loadTransactions();
      loadAddresses();
    });
    toast("Refreshed", "ok");
  });

  $("#wallet-select").addEventListener("change", (e) => {
    if (e.target.value) switchWallet(e.target.value);
  });

  $("#create-wallet-btn").addEventListener("click", async () => {
    const ok = await confirmModal("Create default wallet", [
      { k: "Action", v: "createwallet \"\"" },
      { k: "Note", v: "Creates a new local wallet and selects it." },
    ], { confirmText: "Create wallet" });
    if (ok) createWallet();
  });

  $("#new-address-form").addEventListener("submit", (e) => {
    e.preventDefault();
    newAddress($("#new-address-label").value.trim());
  });

  $("#send-form").addEventListener("submit", (e) => {
    e.preventDefault();
    submitSend();
  });

  $("#console-form").addEventListener("submit", (e) => {
    e.preventDefault();
    runConsole($("#console-method").value.trim(), $("#console-params").value);
    pushConsoleHistory($("#console-method").value.trim());
  });

  $("#tip-form").addEventListener("submit", (e) => {
    e.preventDefault();
    submitTip();
  });

  $$(".tip-preset").forEach((b) =>
    b.addEventListener("click", () => {
      $("#tip-amount").value = b.dataset.amount;
      $$(".tip-preset").forEach((x) => x.classList.toggle("active", x === b));
    }));

  // Send form: recompute fee estimate when amount changes (informational only).
  $("#send-amount").addEventListener("input", () => loadFeeEstimate());

  initConsoleHistory();
}

/* ---------------- console history ---------------- */

let consoleHistory = [];
let historyIdx = -1;

function initConsoleHistory() {
  try {
    consoleHistory = JSON.parse(localStorage.getItem("oxc-console-hist") || "[]");
  } catch (e) {
    consoleHistory = [];
  }
  const method = $("#console-method");
  method.addEventListener("keydown", (e) => {
    if (e.key === "ArrowUp") {
      e.preventDefault();
      if (historyIdx < 0) historyIdx = consoleHistory.length;
      historyIdx = Math.max(0, historyIdx - 1);
      if (consoleHistory[historyIdx]) method.value = consoleHistory[historyIdx];
    } else if (e.key === "ArrowDown") {
      e.preventDefault();
      if (historyIdx < 0) return;
      historyIdx++;
      if (historyIdx >= consoleHistory.length) { historyIdx = -1; method.value = ""; }
      else method.value = consoleHistory[historyIdx];
    }
  });
}

function pushConsoleHistory(m) {
  if (!m) return;
  consoleHistory = consoleHistory.filter((x) => x !== m);
  consoleHistory.unshift(m);
  consoleHistory = consoleHistory.slice(0, 30);
  localStorage.setItem("oxc-console-hist", JSON.stringify(consoleHistory));
  historyIdx = -1;
}

/* ---------------- boot ---------------- */

(async function boot() {
  try {
    const info = await getJSON("/api/info");
    state.info = info;
    $("#about-version").textContent = "v" + info.version;
    renderSupport(info);
    initConsoleSuggestions();
  } catch (e) {
    setNodePill(false);
    toast("Could not reach OrdexCoin Web API", "err");
  }

  await loadWallets();
  loadStatus();
  loadBalances();
  loadPool();

  if (state.wallet) {
    loadTransactions();
    loadAddresses();
  }

  // Polling: fast for status/balances, slow for the rest.
  setInterval(loadStatus, 6000);
  setInterval(loadBalances, 6000);
  setInterval(loadPool, 60000);
  setInterval(() => { if (state.wallet) { loadTransactions(); loadAddresses(); } }, 30000);
})();

document.addEventListener("DOMContentLoaded", bindEvents);
