const AUTH_KEY = "bu_auth";
const path = window.location.pathname.replace(/\/$/, "") || "/";
const isLogin = path === "/" || path === "/login";
const authed = localStorage.getItem(AUTH_KEY) === "1";

if (!isLogin && !authed) {
  window.location.href = "/login";
}

if (isLogin && authed) {
  window.location.href = "/dashboard";
}

document.querySelectorAll(".menu a").forEach((link) => {
  const href = link.getAttribute("href");
  link.classList.toggle("active", href === path);
});

const loginForm = document.querySelector("[data-login-form]");
if (loginForm) {
  loginForm.addEventListener("submit", (e) => {
    e.preventDefault();
    localStorage.setItem(AUTH_KEY, "1");
    window.location.href = "/dashboard";
  });
}

const logoutBtn = document.querySelector("[data-logout]");
if (logoutBtn) {
  logoutBtn.addEventListener("click", (e) => {
    e.preventDefault();
    localStorage.removeItem(AUTH_KEY);
    window.location.href = "/login";
  });
}

const agentsList = document.querySelector("[data-agents-list]");
const agentForm = document.querySelector("[data-agent-form]");
const agentIdInput = document.querySelector("[data-agent-id]");
const homeDirInput = document.querySelector("[data-home-dir]");
const tempDirInput = document.querySelector("[data-temp-dir]");
const serverAddrInput = document.querySelector("[data-server-addr]");
const scheduleInput = document.querySelector("[data-schedule]");
const pollInput = document.querySelector("[data-poll]");
const agentReset = document.querySelector("[data-agent-reset]");
const agentStatus = document.querySelector("[data-agent-status]");
let agentsCache = [];

function renderAgents(items) {
  if (!agentsList) return;
  if (!Array.isArray(items) || items.length === 0) {
    agentsList.innerHTML = "<div class='agent-item'><div class='title'>Нет данных</div></div>";
    return;
  }
  agentsList.innerHTML = items
    .map((agent) => {
      const status = agent.status || "offline";
      const label = status === "online" ? "ONLINE" : "OFFLINE";
      const name = agent.hostname || agent.agent_id || "unknown";
      const lastSeen = agent.last_seen_at ? new Date(agent.last_seen_at).toLocaleString() : "—";
      return `<div class="agent-item" data-agent-row="${agent.agent_id}">
        <div class="status">${label}</div>
        <div class="title">${name}</div>
        <div class="meta">ID: ${agent.agent_id}</div>
        <div class="meta">Last seen: ${lastSeen}</div>
      </div>`;
    })
    .join("");
}

function fillForm(agent) {
  if (!agentForm || !agent) return;
  agentIdInput.value = agent.agent_id || "";
  homeDirInput.value = agent.home_dir || "";
  tempDirInput.value = agent.temp_archive_dir || "";
  serverAddrInput.value = agent.server_addr || "";
  scheduleInput.value = agent.schedule_time || "";
  pollInput.value = agent.poll_interval_seconds || "";
  if (agentStatus) agentStatus.textContent = "Готово к изменению";
}

function clearForm() {
  if (!agentForm) return;
  agentIdInput.value = "";
  homeDirInput.value = "";
  tempDirInput.value = "";
  serverAddrInput.value = "";
  scheduleInput.value = "";
  pollInput.value = "";
  if (agentStatus) agentStatus.textContent = "Выберите агента";
}

async function loadAgents() {
  if (!agentsList) return;
  try {
    const resp = await fetch("/api/agents");
    if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
    const agents = await resp.json();
    agentsCache = Array.isArray(agents) ? agents : [];
    renderAgents(agentsCache);
    if (agentsCache.length > 0 && agentForm && !agentIdInput.value) {
      fillForm(agentsCache[0]);
      const first = agentsList.querySelector(`[data-agent-row="${agentsCache[0].agent_id}"]`);
      if (first) first.classList.add("active");
    }
  } catch (err) {
    agentsList.innerHTML = `<div class='agent-item'><div class='title'>Ошибка</div><div class='meta'>${err.message}</div></div>`;
  }
}

if (agentsList) {
  agentsList.addEventListener("click", (e) => {
    const row = e.target.closest("[data-agent-row]");
    if (!row) return;
    const agentId = row.getAttribute("data-agent-row");
    const agent = agentsCache.find((item) => item.agent_id === agentId);
    agentsList.querySelectorAll(".agent-item").forEach((el) => el.classList.remove("active"));
    row.classList.add("active");
    fillForm(agent);
  });
}

if (agentReset) {
  agentReset.addEventListener("click", () => {
    clearForm();
  });
}

if (agentForm) {
  agentForm.addEventListener("submit", async (e) => {
    e.preventDefault();
    const agentID = agentIdInput.value;
    if (!agentID) return;
    const payload = {
      schedule_time: scheduleInput.value || "",
      poll_interval_seconds: pollInput.value ? Number(pollInput.value) : 0,
      config_json: {
        home_dir: homeDirInput.value || "",
        temp_archive_dir: tempDirInput.value || "",
        server_addr: serverAddrInput.value || "",
      },
    };
    if (agentStatus) agentStatus.textContent = "Сохранение...";
    try {
      const resp = await fetch(`/api/agents/${agentID}/config`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      if (!resp.ok) throw new Error(`HTTP ${resp.status}`);
      if (agentStatus) agentStatus.textContent = "Конфигурация сохранена";
    } catch (err) {
      if (agentStatus) agentStatus.textContent = "Ошибка: " + err.message;
    }
  });
}

loadAgents();

const enrollBtn = document.querySelector("[data-enroll-generate]");
const enrollToken = document.querySelector("[data-enroll-token]");
const enrollExp = document.querySelector("[data-enroll-exp]");
const enrollScript = document.querySelector("[data-enroll-script]");
const enrollCopy = document.querySelector("[data-enroll-copy]");

async function generateEnrollScript() {
  if (!enrollBtn || !enrollScript) return;
  enrollBtn.disabled = true;
  enrollBtn.textContent = "Генерация...";
  try {
    const resp = await fetch("/api/enroll/script", { method: "POST" });
    if (!resp.ok) {
      throw new Error(`HTTP ${resp.status}`);
    }
    const data = await resp.json();
    if (enrollToken) enrollToken.textContent = data.token || "—";
    if (enrollExp) enrollExp.textContent = data.expires_at || "—";
    enrollScript.value = data.script || "";
  } catch (err) {
    if (enrollScript) enrollScript.value = "Ошибка генерации скрипта: " + err.message;
  } finally {
    enrollBtn.disabled = false;
    enrollBtn.textContent = "Сгенерировать скрипт";
  }
}

if (enrollBtn) {
  enrollBtn.addEventListener("click", (e) => {
    e.preventDefault();
    generateEnrollScript();
  });
}

if (enrollCopy) {
  enrollCopy.addEventListener("click", (e) => {
    e.preventDefault();
    const text = enrollScript ? enrollScript.value : "";
    if (!text) return;
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(text);
    } else {
      enrollScript.select();
      document.execCommand("copy");
    }
  });
}
