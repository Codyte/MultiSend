// ====================== BEGIN NAV INDEX ======================
// NAV INDEX — auto-generated symbol map (refresh via the navindex skill)
//   L47    state
//   L63    elements
//   L98    activeStatuses
//   L99    resumableStatuses
//   L101   node
//   L108   number
//   L113   formatBytes
//   L122   formatSpeed
//   L127   statusLabel
//   L141   normalizeStatus
//   L146   api
//   L164   toOperation
//   L189   allOperations
//   L197   operationKind
//   L201   operationTitle
//   L207   renderOperation
//   L267   renderOperations
//   L285   renderNetwork
//   L320   syncPeerOptions
//   L331   renderSummary
//   L341   renderConnection
//   L348   showBanner
//   L358   lines
//   L362   field
//   L366   setSettingsState
//   L373   populateSettings
//   L405   loadSettings
//   L421   settingsPayload
//   L447   submitSettings
//   L469   switchView
//   L485   switchOperation
//   L497   render
//   L505   refresh
//   L535   submitDownload
//   L574   showFormError
//   L583   submitSend
//   L618   submitPull
//   L646   runOperationAction
//   L703   query
//   L705   source
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
  activeOperation: "download",
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
  downloadForm: document.querySelector("#download-form"),
  downloadURL: document.querySelector("#download-url"),
  downloadSubmit: document.querySelector("#download-submit"),
  formError: document.querySelector("#form-error"),
  sendForm: document.querySelector("#send-form"),
  sendPath: document.querySelector("#send-path"),
  sendPeer: document.querySelector("#send-peer"),
  sendSubmit: document.querySelector("#send-submit"),
  sendError: document.querySelector("#send-error"),
  pullForm: document.querySelector("#pull-form"),
  pullSource: document.querySelector("#pull-source"),
  pullSubmit: document.querySelector("#pull-submit"),
  pullError: document.querySelector("#pull-error"),
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
    empty.append(node("span", "", state.filter === "all" ? "Use os formulários acima para baixar, enviar ou receber um arquivo." : "Selecione outro filtro para ver as demais operações."));
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
  const selected = elements.sendPeer.value;
  const options = [new Option("Selecione um computador", "")];
  for (const peer of state.peers) {
    const detail = (peer.ips || []).join(", ") || peer.addr || "disponível";
    options.push(new Option(`${peer.name || peer.node_id} — ${detail}`, peer.node_id));
  }
  elements.sendPeer.replaceChildren(...options);
  if (state.peers.some((peer) => peer.node_id === selected)) elements.sendPeer.value = selected;
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
  if (target === "settings" && !state.configResponse) loadSettings();
}

function switchOperation(target) {
  state.activeOperation = target;
  document.querySelectorAll("[data-operation-target]").forEach((button) => {
    const active = button.dataset.operationTarget === target;
    button.classList.toggle("active", active);
    button.setAttribute("aria-selected", String(active));
  });
  document.querySelectorAll("[data-operation-form]").forEach((form) => {
    form.hidden = form.dataset.operationForm !== target;
  });
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
  const requests = [
    ["health", "/health"],
    ["peers", "/peers"],
    ["interfaces", "/interfaces"],
    ["downloads", "/downloads"],
    ["jobs", "/jobs"],
    ["pulls", "/pulls"],
  ];
  const results = await Promise.allSettled(requests.map(([, path]) => api(path)));
  const errors = [];
  results.forEach((result, index) => {
    const key = requests[index][0];
    if (result.status === "fulfilled") {
      state[key] = result.value ?? (key === "health" || key === "interfaces" ? null : []);
    } else {
      if (key === "health") state.health = null;
      errors.push(`${key}: ${result.reason.message}`);
    }
  });
  state.loading = false;
  state.refreshing = false;
  elements.refreshButton.disabled = false;
  showBanner(errors);
  render();
}

async function submitDownload(event) {
  event.preventDefault();
  elements.formError.hidden = true;
  elements.downloadURL.removeAttribute("aria-invalid");
  const form = new FormData(elements.downloadForm);
  const rawURL = String(form.get("url") || "").trim();
  try {
    const parsed = new URL(rawURL);
    if (!new Set(["http:", "https:"]).has(parsed.protocol)) throw new Error("Use uma URL HTTP ou HTTPS válida.");
  } catch (error) {
    elements.downloadURL.setAttribute("aria-invalid", "true");
    elements.formError.textContent = error.message === "Use uma URL HTTP ou HTTPS válida." ? error.message : "Informe uma URL HTTP ou HTTPS válida.";
    elements.formError.hidden = false;
    elements.downloadURL.focus();
    return;
  }

  const chunkSize = number(form.get("chunk_size_mb"));
  const payload = {
    url: rawURL,
    output_dir: String(form.get("output_dir") || "").trim(),
    file_name: String(form.get("file_name") || "").trim(),
    chunk_size_mb: chunkSize > 0 ? chunkSize : 0,
  };
  elements.downloadSubmit.disabled = true;
  elements.downloadSubmit.textContent = "Iniciando…";
  try {
    await api("/downloads", { method: "POST", body: payload });
    elements.downloadForm.reset();
    await refresh();
  } catch (error) {
    elements.formError.textContent = error.message;
    elements.formError.hidden = false;
  } finally {
    elements.downloadSubmit.disabled = false;
    elements.downloadSubmit.textContent = "Iniciar download";
  }
}

function showFormError(element, input, message) {
  element.textContent = message;
  element.hidden = false;
  if (input) {
    input.setAttribute("aria-invalid", "true");
    input.focus();
  }
}

async function submitSend(event) {
  event.preventDefault();
  elements.sendError.hidden = true;
  elements.sendPath.removeAttribute("aria-invalid");
  const form = new FormData(elements.sendForm);
  const filePath = String(form.get("file_path") || "").trim();
  const peerNodeID = String(form.get("peer_node_id") || "").trim();
  const peerAddress = String(form.get("peer_address") || "").trim();
  if (!/^(?:[a-zA-Z]:[\\/]|\\\\)/.test(filePath)) {
    showFormError(elements.sendError, elements.sendPath, "Informe um caminho absoluto do Windows ou UNC.");
    return;
  }
  if (!peerNodeID && !peerAddress) {
    showFormError(elements.sendError, elements.sendPeer, "Selecione um computador ou informe o endereço manual.");
    return;
  }
  elements.sendSubmit.disabled = true;
  elements.sendSubmit.textContent = "Iniciando…";
  try {
    await api("/send", { method: "POST", body: {
      file_path: filePath,
      peer_node_id: peerNodeID,
      peer_address: peerAddress,
      chunk_size_mb: Math.max(0, number(form.get("chunk_size_mb"))),
    } });
    elements.sendForm.reset();
    await refresh();
  } catch (error) {
    showFormError(elements.sendError, null, error.message);
  } finally {
    elements.sendSubmit.disabled = false;
    elements.sendSubmit.textContent = "Iniciar envio";
  }
}

async function submitPull(event) {
  event.preventDefault();
  elements.pullError.hidden = true;
  elements.pullSource.removeAttribute("aria-invalid");
  const form = new FormData(elements.pullForm);
  const sourceURL = String(form.get("source_url") || "").trim();
  if (!/^file:\/\//i.test(sourceURL) && !/^\\\\/.test(sourceURL)) {
    showFormError(elements.pullError, elements.pullSource, "Use uma origem file:// ou um caminho UNC.");
    return;
  }
  elements.pullSubmit.disabled = true;
  elements.pullSubmit.textContent = "Iniciando…";
  try {
    await api("/pulls", { method: "POST", body: {
      source_url: sourceURL,
      output_dir: String(form.get("output_dir") || "").trim(),
      chunk_size_mb: Math.max(0, number(form.get("chunk_size_mb"))),
    } });
    elements.pullForm.reset();
    await refresh();
  } catch (error) {
    showFormError(elements.pullError, null, error.message);
  } finally {
    elements.pullSubmit.disabled = false;
    elements.pullSubmit.textContent = "Iniciar recebimento";
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
    await refresh();
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

elements.downloadForm.addEventListener("submit", submitDownload);
elements.sendForm.addEventListener("submit", submitSend);
elements.pullForm.addEventListener("submit", submitPull);
elements.settingsForm.addEventListener("submit", submitSettings);
elements.settingsForm.addEventListener("input", () => {
  if (state.configResponse) setSettingsState("Alterações não salvas", "Revise os valores e salve para persistir.");
});
elements.refreshButton.addEventListener("click", () => state.activeView === "settings" ? loadSettings() : refresh());
document.querySelectorAll("[data-view-target]").forEach((button) => {
  button.addEventListener("click", () => switchView(button.dataset.viewTarget));
});
document.querySelectorAll("[data-operation-target]").forEach((button) => {
  button.addEventListener("click", () => switchOperation(button.dataset.operationTarget));
});
elements.operationsList.addEventListener("click", (event) => {
  const button = event.target.closest("[data-operation-action]");
  if (button) runOperationAction(button);
});
document.addEventListener("visibilitychange", () => {
  if (document.visibilityState === "visible") refresh();
});

refresh();
const query = new URLSearchParams(window.location.search);
if (query.get("view") === "settings") switchView("settings");
const source = query.get("source");
if (source) {
  if (/^https?:\/\//i.test(source)) {
    elements.downloadURL.value = source;
    switchOperation("download");
  } else {
    elements.pullSource.value = source;
    switchOperation("pull");
  }
}
setInterval(() => {
  if (document.visibilityState === "visible") refresh();
}, 1200);
