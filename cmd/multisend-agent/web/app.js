// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L52    state
//   L67    elements
//   L99    activeStatuses
//   L100   resumableStatuses
//   L101   activeRefreshDelayMS
//   L102   idleRefreshDelayMS
//   L105   node
//   L112   number
//   L117   formatBytes
//   L126   formatSpeed
//   L131   statusLabel
//   L145   normalizeStatus
//   L150   api
//   L168   toOperation
//   L193   allOperations
//   L201   operationKind
//   L205   operationTitle
//   L211   renderOperation
//   L271   renderOperations
//   L289   renderNetwork
//   L324   syncPeerOptions
//   L335   renderSummary
//   L345   renderConnection
//   L352   showBanner
//   L362   lines
//   L366   field
//   L370   setSettingsState
//   L377   populateSettings
//   L409   loadSettings
//   L425   settingsPayload
//   L451   submitSettings
//   L473   switchView
//   L494   render
//   L502   refresh
//   L527   clearScheduledRefresh
//   L534   nextRefreshDelay
//   L540   scheduleRefresh
//   L546   refreshAndSchedule
//   L552   showFormError
//   L561   detectTransferKind
//   L570   updateTransferForm
//   L590   submitTransfer
//   L668   runOperationAction
//   L731   query
//   L734   source
// ======================= END NAV INDEX =======================

"use strict";

const state = {
  health: null,
  peers: [],
  interfaces: null,
  downloads: [],
  jobs: [],
  pulls: [],
  configResponse: null,
  activeView: "dashboard",
  filter: "all",
  loading: true,
  refreshing: false,
  settingsLoading: false,
};

const elements = {
  agentStatus: document.querySelector("#agent-status"),
  refreshButton: document.querySelector("#refresh-button"),
  errorBanner: document.querySelector("#error-banner"),
  activeCount: document.querySelector("#active-count"),
  currentSpeed: document.querySelector("#current-speed"),
  peerCount: document.querySelector("#peer-count"),
  interfaceCount: document.querySelector("#interface-count"),
  networkContent: document.querySelector("#network-content"),
  operationsList: document.querySelector("#operations-list"),
  transferForm: document.querySelector("#transfer-form"),
  transferSource: document.querySelector("#transfer-source"),
  transferMode: document.querySelector("#transfer-mode"),
  transferKind: document.querySelector("#transfer-kind"),
  transferPeer: document.querySelector("#transfer-peer"),
  transferSubmit: document.querySelector("#transfer-submit"),
  transferError: document.querySelector("#transfer-error"),
  localOutputFields: document.querySelector("#local-output-fields"),
  downloadNameField: document.querySelector("#download-name-field"),
  peerFields: document.querySelector("#peer-fields"),
  dashboardView: document.querySelector("#dashboard-view"),
  settingsView: document.querySelector("#settings-view"),
  settingsForm: document.querySelector("#settings-form"),
  settingsSave: document.querySelector("#settings-save"),
  settingsState: document.querySelector("#settings-state"),
  settingsMessage: document.querySelector("#settings-message"),
  settingsSavebar: document.querySelector(".settings-savebar"),
  runtimeSummary: document.querySelector("#runtime-summary"),
  secretStatus: document.querySelector("#secret-status"),
  lastUpdated: document.querySelector("#last-updated"),
};

const activeStatuses = new Set(["running", "starting", "resuming", "canceling"]);
const resumableStatuses = new Set(["canceled", "failed"]);
const activeRefreshDelayMS = 1200;
const idleRefreshDelayMS = 5000;
let refreshTimer = null;

function node(tag, className, text) {
  const element = document.createElement(tag);
  if (className) element.className = className;
  if (text !== undefined) element.textContent = text;
  return element;
}

function number(value, fallback = 0) {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : fallback;
}

function formatBytes(value) {
  const bytes = Math.max(0, number(value));
  if (bytes === 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  const index = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1);
  const amount = bytes / (1024 ** index);
  return `${amount.toLocaleString("pt-BR", { maximumFractionDigits: index === 0 ? 0 : 1 })} ${units[index]}`;
}

function formatSpeed(value) {
  const speed = Math.max(0, number(value));
  return `${speed.toLocaleString("pt-BR", { maximumFractionDigits: 1 })} Mbps`;
}

function statusLabel(value) {
  const labels = {
    running: "Em andamento",
    starting: "Iniciando",
    resuming: "Retomando",
    canceling: "Cancelando",
    canceled: "Interrompida",
    failed: "Falhou",
    done: "Concluída",
    completed: "Concluída",
  };
  return labels[value] || value || "Desconhecido";
}

function normalizeStatus(value) {
  const status = String(value || "unknown").toLowerCase();
  return status === "completed" ? "done" : status;
}

async function api(path, options = {}) {
  const request = { ...options, headers: { Accept: "application/json", ...(options.headers || {}) } };
  if (request.body !== undefined) {
    request.headers["Content-Type"] = "application/json";
    request.body = JSON.stringify(request.body);
  }
  const response = await fetch(path, request);
  const text = await response.text();
  let payload = null;
  if (text) {
    try { payload = JSON.parse(text); } catch { payload = { message: text }; }
  }
  if (!response.ok) {
    throw new Error(payload?.message || payload?.error || `HTTP ${response.status}`);
  }
  return payload;
}

function toOperation(item, type) {
  const status = normalizeStatus(item.status);
  const done = number(item.bytes_done ?? item.bytes_sent);
  const total = number(item.total_bytes);
  const computedPercent = total > 0 ? (done * 100) / total : 0;
  const percent = Math.min(100, Math.max(0, number(item.percent, computedPercent)));
  const source = item.url || item.file_path || item.source_url || item.output_path || item.id;
  const output = item.output_path || item.file_path || item.folder_result || "";
  return {
    id: String(item.id || ""),
    type,
    status,
    source: String(source || "Operação sem nome"),
    output: String(output || ""),
    message: String(item.message || item.last_error || ""),
    done,
    total,
    percent,
    speed: number(item.mbps_now),
    chunksDone: number(item.chunks_done),
    chunksTotal: number(item.chunks_total),
    resumeSupported: item.resume_supported !== false,
  };
}

function allOperations() {
  return [
    ...state.downloads.map((item) => toOperation(item, "downloads")),
    ...state.jobs.map((item) => toOperation(item, "jobs")),
    ...state.pulls.map((item) => toOperation(item, "pulls")),
  ].sort((a, b) => b.id.localeCompare(a.id));
}

function operationKind(type) {
  return { downloads: "Download HTTP(S)", jobs: "Envio pela rede", pulls: "Recebimento remoto" }[type] || "Transferência";
}

function operationTitle(operation) {
  const source = operation.source.replaceAll("\\", "/");
  const parts = source.split("/").filter(Boolean);
  return parts.at(-1) || source;
}

function renderOperation(operation) {
  const card = node("article", "operation-card");
  const top = node("div", "operation-top");
  const title = node("div", "operation-title");
  title.append(node("h3", "", operationTitle(operation)), node("p", "operation-kind", operationKind(operation.type)));
  top.append(title, node("span", `status-badge ${operation.status}`, statusLabel(operation.status)));

  const track = node("div", "progress-track");
  track.setAttribute("role", "progressbar");
  track.setAttribute("aria-label", `Progresso de ${operationTitle(operation)}`);
  track.setAttribute("aria-valuemin", "0");
  track.setAttribute("aria-valuemax", "100");
  track.setAttribute("aria-valuenow", operation.percent.toFixed(0));
  const fill = node("span", "progress-fill");
  fill.style.width = `${operation.percent}%`;
  track.append(fill);

  const meta = node("div", "operation-meta");
  meta.append(
    node("span", "", `${operation.percent.toLocaleString("pt-BR", { maximumFractionDigits: 1 })}%`),
    node("span", "", `${formatBytes(operation.done)} de ${operation.total > 0 ? formatBytes(operation.total) : "tamanho desconhecido"}`),
    node("span", "", formatSpeed(operation.speed)),
  );
  if (operation.chunksTotal > 0) meta.append(node("span", "", `${operation.chunksDone}/${operation.chunksTotal} chunks`));

  card.append(top, track, meta);
  if (operation.message) card.append(node("p", `operation-message ${operation.status}`, operation.message));

  const bottom = node("div", "operation-bottom");
  const path = node("span", "operation-path", operation.output || operation.source);
  path.title = operation.output || operation.source;
  const actions = node("div", "operation-actions");
  if (activeStatuses.has(operation.status)) {
    const cancel = node("button", "button danger small", "Interromper");
    cancel.type = "button";
    cancel.dataset.operationAction = "cancel";
    cancel.dataset.operationType = operation.type;
    cancel.dataset.operationId = operation.id;
    actions.append(cancel);
  } else if (resumableStatuses.has(operation.status) && operation.resumeSupported) {
    const resume = node("button", "button secondary small", "Retomar");
    resume.type = "button";
    resume.dataset.operationAction = "resume";
    resume.dataset.operationType = operation.type;
    resume.dataset.operationId = operation.id;
    actions.append(resume);
  }
  if (!activeStatuses.has(operation.status)) {
    const forget = node("button", "button danger small", "Remover");
    forget.type = "button";
    forget.dataset.operationAction = "forget";
    forget.dataset.operationType = operation.type;
    forget.dataset.operationId = operation.id;
    actions.append(forget);
  }
  bottom.append(path, actions);
  card.append(bottom);
  return card;
}

function renderOperations() {
  const operations = allOperations().filter((operation) => {
    if (state.filter === "active") return activeStatuses.has(operation.status);
    if (state.filter === "finished") return !activeStatuses.has(operation.status);
    return true;
  });
  elements.operationsList.replaceChildren();
  elements.operationsList.setAttribute("aria-busy", "false");
  if (operations.length === 0) {
    const empty = node("div", "empty-state");
    empty.append(node("strong", "", state.filter === "all" ? "Nenhuma transferência ainda" : "Nenhuma operação neste filtro"));
    empty.append(node("span", "", state.filter === "all" ? "Use o formulário acima para baixar, enviar ou receber um arquivo." : "Selecione outro filtro para ver as demais operações."));
    elements.operationsList.append(empty);
    return;
  }
  elements.operationsList.append(...operations.map(renderOperation));
}

function renderNetwork() {
  elements.networkContent.replaceChildren();
  const peerGroup = node("div", "network-group");
  peerGroup.append(node("p", "network-group-title", "Computadores"));
  if (state.peers.length === 0) {
    peerGroup.append(node("p", "empty-mini", "Nenhum peer descoberto agora."));
  } else {
    for (const peer of state.peers.slice(0, 6)) {
      const row = node("div", "network-row");
      row.append(node("span", "network-name", peer.name || peer.node_id || "Peer"));
      row.append(node("span", "network-detail", (peer.ips || []).join(", ") || peer.addr || "Disponível"));
      peerGroup.append(row);
    }
  }

  const interfaceGroup = node("div", "network-group");
  interfaceGroup.append(node("p", "network-group-title", "Interfaces"));
  const interfaces = state.interfaces?.interfaces || [];
  if (interfaces.length === 0) {
    interfaceGroup.append(node("p", "empty-mini", "Nenhuma interface informada."));
  } else {
    for (const item of interfaces.slice(0, 8)) {
      const row = node("div", "network-row");
      const name = item.name || item.Name || "Interface";
      const usable = item.usable ?? item.Usable;
      const ipv4 = item.ipv4 || item.IPv4;
      row.append(node("span", "network-name", name));
      row.append(node("span", "network-detail", usable ? (ipv4 || "Utilizável") : "Ignorada"));
      interfaceGroup.append(row);
    }
  }
  elements.networkContent.append(peerGroup, interfaceGroup);
  syncPeerOptions();
}

function syncPeerOptions() {
  const selected = elements.transferPeer.value;
  const options = [new Option("Selecione um computador", "")];
  for (const peer of state.peers) {
    const detail = (peer.ips || []).join(", ") || peer.addr || "disponível";
    options.push(new Option(`${peer.name || peer.node_id} — ${detail}`, peer.node_id));
  }
  elements.transferPeer.replaceChildren(...options);
  if (state.peers.some((peer) => peer.node_id === selected)) elements.transferPeer.value = selected;
}

function renderSummary() {
  const operations = allOperations();
  const active = operations.filter((operation) => activeStatuses.has(operation.status));
  const usable = state.interfaces?.usable_interfaces || [];
  elements.activeCount.textContent = String(active.length);
  elements.currentSpeed.textContent = formatSpeed(active.reduce((sum, operation) => sum + operation.speed, 0));
  elements.peerCount.textContent = String(state.peers.length);
  elements.interfaceCount.textContent = String(usable.length);
}

function renderConnection(online) {
  elements.agentStatus.className = `connection ${online ? "online" : "offline"}`;
  elements.agentStatus.lastChild.textContent = online
    ? ` ${state.health?.name || "Agente online"}`
    : " Agente indisponível";
}

function showBanner(messages) {
  if (messages.length === 0) {
    elements.errorBanner.hidden = true;
    elements.errorBanner.textContent = "";
    return;
  }
  elements.errorBanner.textContent = `Alguns dados não puderam ser atualizados: ${messages.join(" · ")}`;
  elements.errorBanner.hidden = false;
}

function lines(value) {
  return String(value || "").split(/\r?\n/).map((item) => item.trim()).filter(Boolean);
}

function field(name) {
  return elements.settingsForm.elements.namedItem(name);
}

function setSettingsState(title, message, kind = "") {
  elements.settingsState.textContent = title;
  elements.settingsMessage.textContent = message;
  elements.settingsSavebar.classList.remove("success", "error");
  if (kind) elements.settingsSavebar.classList.add(kind);
}

function populateSettings(response) {
  const cfg = response.config;
  const scalarFields = [
    "display_name", "receive_path", "interface_policy", "interface_refresh_seconds",
    "pull_folder_result", "download_pipeline_mode", "download_in_flight_per_channel",
    "download_channel_strategy", "download_split_cable_pct",
  ];
  const booleanFields = [
    "allow_new_interfaces_during_transfer", "ignore_virtual_interfaces", "ignore_vpn_interfaces",
    "ignore_link_local", "cleanup_completed_chunks", "keep_manifests",
    "cleanup_empty_download_dirs", "require_auth",
  ];
  const listFields = ["allowed_interface_types", "manual_interfaces", "ignored_interfaces", "remote_send_roots"];
  scalarFields.forEach((name) => { field(name).value = cfg[name] ?? ""; });
  booleanFields.forEach((name) => { field(name).checked = Boolean(cfg[name]); });
  listFields.forEach((name) => { field(name).value = (cfg[name] || []).join("\n"); });

  const runtime = response.runtime;
  const ports = runtime.selected_ports;
  elements.runtimeSummary.textContent = `${runtime.node_id} · v${runtime.app_version} · API ${ports.local_api} · transferência ${ports.transfer}`;
  elements.secretStatus.textContent = response.secret_configured
    ? "O segredo do nó está configurado e permanece oculto."
    : "Nenhum segredo existe; um será gerado ao ativar a autenticação.";
  setSettingsState(
    response.restart_required ? "Configuração salva" : "Nenhuma alteração pendente",
    response.restart_required
      ? "Reabra o MultiSend pelo atalho para aplicar todos os campos."
      : "Alterações passam a valer completamente após reiniciar o agente.",
    response.restart_required ? "success" : "",
  );
}

async function loadSettings() {
  if (state.settingsLoading) return;
  state.settingsLoading = true;
  elements.settingsSave.disabled = true;
  setSettingsState("Carregando configuração…", "Lendo o arquivo pelo agente.");
  try {
    state.configResponse = await api("/api/v1/config");
    populateSettings(state.configResponse);
  } catch (error) {
    setSettingsState("Falha ao carregar", error.message, "error");
  } finally {
    state.settingsLoading = false;
    elements.settingsSave.disabled = false;
  }
}

function settingsPayload() {
  return {
    display_name: field("display_name").value.trim(),
    receive_path: field("receive_path").value.trim(),
    interface_policy: field("interface_policy").value,
    interface_refresh_seconds: number(field("interface_refresh_seconds").value),
    allow_new_interfaces_during_transfer: field("allow_new_interfaces_during_transfer").checked,
    ignore_virtual_interfaces: field("ignore_virtual_interfaces").checked,
    ignore_vpn_interfaces: field("ignore_vpn_interfaces").checked,
    ignore_link_local: field("ignore_link_local").checked,
    allowed_interface_types: lines(field("allowed_interface_types").value),
    manual_interfaces: lines(field("manual_interfaces").value),
    ignored_interfaces: lines(field("ignored_interfaces").value),
    cleanup_completed_chunks: field("cleanup_completed_chunks").checked,
    keep_manifests: field("keep_manifests").checked,
    cleanup_empty_download_dirs: field("cleanup_empty_download_dirs").checked,
    pull_folder_result: field("pull_folder_result").value,
    download_pipeline_mode: field("download_pipeline_mode").value,
    download_in_flight_per_channel: number(field("download_in_flight_per_channel").value),
    download_channel_strategy: field("download_channel_strategy").value,
    download_split_cable_pct: number(field("download_split_cable_pct").value),
    require_auth: field("require_auth").checked,
    remote_send_roots: lines(field("remote_send_roots").value),
  };
}

async function submitSettings(event) {
  event.preventDefault();
  if (!elements.settingsForm.checkValidity()) {
    elements.settingsForm.reportValidity();
    setSettingsState("Revise os campos destacados", "Existem valores obrigatórios ou fora da faixa permitida.", "error");
    return;
  }
  elements.settingsSave.disabled = true;
  elements.settingsSave.textContent = "Salvando…";
  setSettingsState("Validando configuração…", "Nenhuma alteração foi gravada ainda.");
  try {
    const response = await api("/api/v1/config", { method: "PUT", body: settingsPayload() });
    state.configResponse = response;
    populateSettings(response);
  } catch (error) {
    setSettingsState("Configuração não salva", error.message, "error");
  } finally {
    elements.settingsSave.disabled = false;
    elements.settingsSave.textContent = "Salvar configuração";
  }
}

function switchView(target) {
  state.activeView = target;
  elements.dashboardView.hidden = target !== "dashboard";
  elements.settingsView.hidden = target !== "settings";
  document.querySelectorAll("[data-view-target]").forEach((button) => {
    const active = button.dataset.viewTarget === target;
    button.classList.toggle("active", active);
    button.setAttribute("aria-pressed", String(active));
  });
  const locationURL = new URL(window.location.href);
  if (target === "settings") locationURL.searchParams.set("view", "settings");
  else locationURL.searchParams.delete("view");
  history.replaceState(null, "", locationURL);
  if (target === "settings") {
    clearScheduledRefresh();
    if (!state.configResponse) loadSettings();
  } else {
    refreshAndSchedule();
  }
}

function render() {
  renderConnection(Boolean(state.health?.ok));
  renderSummary();
  renderNetwork();
  renderOperations();
  elements.lastUpdated.textContent = `Atualizado às ${new Date().toLocaleTimeString("pt-BR")}`;
}

async function refresh() {
  if (state.refreshing) return;
  state.refreshing = true;
  elements.refreshButton.disabled = true;
  const errors = [];
  try {
    const snapshot = await api("/api/v1/dashboard");
    state.health = snapshot?.health ?? null;
    state.peers = snapshot?.peers ?? [];
    state.interfaces = snapshot?.interfaces ?? null;
    state.downloads = snapshot?.downloads ?? [];
    state.jobs = snapshot?.jobs ?? [];
    state.pulls = snapshot?.pulls ?? [];
  } catch (error) {
    state.health = null;
    errors.push(`dashboard: ${error.message}`);
  } finally {
    state.loading = false;
    state.refreshing = false;
    elements.refreshButton.disabled = false;
    showBanner(errors);
    render();
  }
}

function clearScheduledRefresh() {
  if (refreshTimer !== null) {
    window.clearTimeout(refreshTimer);
    refreshTimer = null;
  }
}

function nextRefreshDelay() {
  return allOperations().some((operation) => activeStatuses.has(operation.status))
    ? activeRefreshDelayMS
    : idleRefreshDelayMS;
}

function scheduleRefresh() {
  clearScheduledRefresh();
  if (document.visibilityState !== "visible" || state.activeView !== "dashboard") return;
  refreshTimer = window.setTimeout(refreshAndSchedule, nextRefreshDelay());
}

async function refreshAndSchedule() {
  clearScheduledRefresh();
  await refresh();
  scheduleRefresh();
}

function showFormError(element, input, message) {
  element.textContent = message;
  element.hidden = false;
  if (input) {
    input.setAttribute("aria-invalid", "true");
    input.focus();
  }
}

function detectTransferKind(source, requestedMode = elements.transferMode.value) {
  if (!source) return "";
  if (requestedMode !== "auto") return requestedMode;
  if (/^https?:\/\//i.test(source)) return "download";
  if (/^file:\/\//i.test(source) || /^\\\\/.test(source)) return "pull";
  if (/^[a-zA-Z]:[\\/]/.test(source)) return "send";
  return "";
}

function updateTransferForm() {
  const kind = detectTransferKind(elements.transferSource.value.trim());
  const descriptions = {
    download: "Download HTTP(S) para este computador.",
    pull: "Recebimento de um compartilhamento remoto para este computador.",
    send: "Envio de um arquivo local para outro computador.",
  };
  elements.transferKind.textContent = descriptions[kind] || "Informe uma URL, caminho local ou compartilhamento de rede.";
  elements.localOutputFields.hidden = kind !== "download" && kind !== "pull";
  elements.downloadNameField.hidden = kind !== "download";
  elements.peerFields.hidden = kind !== "send";
  elements.transferSubmit.disabled = kind === "";
  elements.transferSubmit.textContent = {
    download: "Iniciar download",
    pull: "Iniciar recebimento",
    send: "Iniciar envio",
  }[kind] || "Iniciar transferência";
  return kind;
}

async function submitTransfer(event) {
  event.preventDefault();
  elements.transferError.hidden = true;
  elements.transferSource.removeAttribute("aria-invalid");
  if (!elements.transferForm.checkValidity()) {
    elements.transferForm.reportValidity();
    showFormError(elements.transferError, null, "Revise os campos obrigatórios ou fora da faixa permitida.");
    return;
  }
  const form = new FormData(elements.transferForm);
  const source = String(form.get("source") || "").trim();
  const kind = detectTransferKind(source, String(form.get("transfer_mode") || "auto"));
  if (!kind) {
    showFormError(elements.transferError, elements.transferSource, "Use uma URL HTTP(S), caminho local absoluto ou compartilhamento UNC/file://.");
    return;
  }
  const chunkSize = Math.max(0, number(form.get("chunk_size_mb")));
  let path;
  let payload;
  if (kind === "download") {
    try {
      const parsed = new URL(source);
      if (parsed.protocol !== "http:" && parsed.protocol !== "https:") throw new Error();
    } catch {
      showFormError(elements.transferError, elements.transferSource, "Informe uma URL HTTP ou HTTPS válida.");
      return;
    }
    path = "/downloads";
    payload = {
      url: source,
      output_dir: String(form.get("output_dir") || "").trim(),
      file_name: String(form.get("file_name") || "").trim(),
      chunk_size_mb: chunkSize,
    };
  } else if (kind === "pull") {
    if (!/^file:\/\//i.test(source) && !/^\\\\/.test(source)) {
      showFormError(elements.transferError, elements.transferSource, "Para receber, use uma origem file:// ou um caminho UNC.");
      return;
    }
    path = "/pulls";
    payload = {
      source_url: source,
      output_dir: String(form.get("output_dir") || "").trim(),
      chunk_size_mb: chunkSize,
    };
  } else {
    if (!/^(?:[a-zA-Z]:[\\/]|\\\\)/.test(source)) {
      showFormError(elements.transferError, elements.transferSource, "Para enviar, use um caminho absoluto do Windows ou UNC.");
      return;
    }
    const peerNodeID = String(form.get("peer_node_id") || "").trim();
    const peerAddress = String(form.get("peer_address") || "").trim();
    if (!peerNodeID && !peerAddress) {
      showFormError(elements.transferError, elements.transferPeer, "Selecione um computador ou informe o endereço manual.");
      return;
    }
    path = "/send";
    payload = {
      file_path: source,
      peer_node_id: peerNodeID,
      peer_address: peerAddress,
      chunk_size_mb: chunkSize,
    };
  }
  elements.transferSubmit.disabled = true;
  elements.transferSubmit.textContent = "Iniciando…";
  try {
    await api(path, { method: "POST", body: payload });
    elements.transferForm.reset();
    updateTransferForm();
    await refreshAndSchedule();
  } catch (error) {
    showFormError(elements.transferError, null, error.message);
  } finally {
    updateTransferForm();
  }
}

async function runOperationAction(button) {
  const { operationAction: action, operationType: type, operationId: id } = button.dataset;
  if (!action || !type || !id) return;
  if (action === "forget") {
    const warning = type === "downloads"
      ? "Remover este download do histórico? Os chunks parciais e a capacidade de retomar serão apagados; o arquivo final será preservado."
      : "Remover esta operação do histórico local? Arquivos de origem e destino serão preservados.";
    if (!window.confirm(warning)) return;
  }
  button.disabled = true;
  try {
    const path = `/${encodeURIComponent(type)}/${encodeURIComponent(id)}`;
    if (action === "forget") await api(path, { method: "DELETE" });
    else await api(`${path}/${action}`, { method: "POST", body: {} });
    await refreshAndSchedule();
  } catch (error) {
    showBanner([error.message]);
  } finally {
    button.disabled = false;
  }
}

document.querySelectorAll("[data-filter]").forEach((button) => {
  button.addEventListener("click", () => {
    state.filter = button.dataset.filter;
    document.querySelectorAll("[data-filter]").forEach((item) => {
      const active = item === button;
      item.classList.toggle("active", active);
      item.setAttribute("aria-pressed", String(active));
    });
    renderOperations();
  });
});

elements.transferForm.addEventListener("submit", submitTransfer);
elements.transferSource.addEventListener("input", () => {
  elements.transferError.hidden = true;
  elements.transferSource.removeAttribute("aria-invalid");
  updateTransferForm();
});
elements.transferMode.addEventListener("change", updateTransferForm);
elements.settingsForm.addEventListener("submit", submitSettings);
elements.settingsForm.addEventListener("input", () => {
  if (state.configResponse) setSettingsState("Alterações não salvas", "Revise os valores e salve para persistir.");
});
elements.refreshButton.addEventListener("click", () => state.activeView === "settings" ? loadSettings() : refreshAndSchedule());
document.querySelectorAll("[data-view-target]").forEach((button) => {
  button.addEventListener("click", () => switchView(button.dataset.viewTarget));
});
elements.operationsList.addEventListener("click", (event) => {
  const button = event.target.closest("[data-operation-action]");
  if (button) runOperationAction(button);
});
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState !== "visible") {
    clearScheduledRefresh();
  } else if (state.activeView === "settings") {
    loadSettings();
  } else {
    refreshAndSchedule();
  }
});

const query = new URLSearchParams(window.location.search);
if (query.get("view") === "settings") switchView("settings");
else refreshAndSchedule();
const source = query.get("source");
if (source) {
  elements.transferSource.value = source;
  const mode = query.get("mode");
  if (["send", "pull", "download"].includes(mode)) elements.transferMode.value = mode;
  updateTransferForm();
  const cleanURL = new URL(window.location.href);
  cleanURL.searchParams.delete("source");
  cleanURL.searchParams.delete("mode");
  window.history.replaceState(null, "", cleanURL);
}
