const messageList = document.getElementById("messageList");
const promptInput = document.getElementById("promptInput");
const sendBtn = document.getElementById("sendBtn");
const newChatBtn = document.getElementById("newChatBtn");
const sessionList = document.getElementById("sessionList");
const prefModal = document.getElementById("prefModal");
const prefQuestion = document.getElementById("prefQuestion");
const prefOptions = document.getElementById("prefOptions");
const prefPrevBtn = document.getElementById("prefPrevBtn");
const prefNextBtn = document.getElementById("prefNextBtn");
const authModal = document.getElementById("authModal");
const authUsername = document.getElementById("authUsername");
const authPassword = document.getElementById("authPassword");
const authRegisterBtn = document.getElementById("authRegisterBtn");
const authLoginBtn = document.getElementById("authLoginBtn");
const registerModal = document.getElementById("registerModal");
const regUsername = document.getElementById("regUsername");
const regPassword = document.getElementById("regPassword");
const regPassword2 = document.getElementById("regPassword2");
const regBackBtn = document.getElementById("regBackBtn");
const regSubmitBtn = document.getElementById("regSubmitBtn");
const toastEl = document.getElementById("toast");

let sessions = [];
let currentSessionId = null;
let token = localStorage.getItem("token") || "";
let userId = Number(localStorage.getItem("user_id") || 0);
let username = localStorage.getItem("username") || "";
let prefQuestions = [];
let prefStep = 0;
let prefAnswers = {
  spicy_level: "",
  budget_range: "",
  cuisine_likes: [],
  avoid_foods: [],
  diet_goal: "",
  dining_time: "",
};

function createSession(title = "新会话") {
  const id = `${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
  const session = { id, title, messages: [] };
  sessions.unshift(session);
  currentSessionId = id;
  renderSessions();
  renderMessages();
}

function getCurrentSession() {
  return sessions.find((s) => s.id === currentSessionId);
}

function renderSessions() {
  sessionList.innerHTML = "";
  sessions.forEach((s) => {
    const item = document.createElement("div");
    item.className = "session-item";
    item.textContent = s.title;
    item.onclick = () => {
      currentSessionId = s.id;
      renderMessages();
    };
    sessionList.appendChild(item);
  });
}

function appendMessage(role, text) {
  const session = getCurrentSession();
  if (!session) return;
  session.messages.push({ role, text });
  renderMessages();
}

let toastTimer = null;
function showToast(text, type = "success") {
  toastEl.textContent = text;
  toastEl.className = `toast ${type}`;
  toastEl.classList.remove("hidden");
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => {
    toastEl.classList.add("hidden");
  }, 2200);
}

function appendCards(merchants) {
  if (!Array.isArray(merchants) || merchants.length === 0) return;
  const cards = document.createElement("div");
  cards.className = "cards";
  merchants.forEach((m) => {
    const card = document.createElement("div");
    card.className = "card";
    card.innerHTML = `
      <h4>${m.name}</h4>
      <div class="meta">品类：${m.category} | 评分：${m.rating}</div>
      <div class="meta">人均：￥${m.avg_price} | 配送：${m.delivery_time}分钟</div>
      <div class="meta">推荐理由：${m.reason}</div>
      <button class="order-btn">选这家并自动下单支付</button>
    `;
    const btn = card.querySelector(".order-btn");
    btn.onclick = async () => {
      btn.disabled = true;
      btn.textContent = "处理中...";
      try {
        const resp = await fetch("/api/v1/order/auto-place-pay", {
          method: "POST",
          headers: authHeaders(),
          body: JSON.stringify({
            merchant_id: m.id,
            merchant_name: m.name,
            amount: Number(m.avg_price || 30),
          }),
        });
        const data = await resp.json();
        if (data.code !== 0) {
          appendMessage("assistant", "下单失败，请稍后重试。");
        } else {
          appendMessage("assistant", data.data.tip || "已自动下单并支付。");
        }
      } catch (e) {
        appendMessage("assistant", "下单失败，网络异常。");
      } finally {
        btn.disabled = false;
        btn.textContent = "选这家并自动下单支付";
      }
    };
    cards.appendChild(card);
  });
  messageList.appendChild(cards);
  messageList.scrollTop = messageList.scrollHeight;
}

function renderMessages() {
  const session = getCurrentSession();
  messageList.innerHTML = "";
  if (!session) return;
  session.messages.forEach((m) => {
    const div = document.createElement("div");
    div.className = `message ${m.role}`;
    div.textContent = m.text;
    messageList.appendChild(div);
  });
  messageList.scrollTop = messageList.scrollHeight;
}

async function sendMessage() {
  const text = promptInput.value.trim();
  if (!text) return;
  appendMessage("user", text);
  promptInput.value = "";

  try {
    const resp = await fetch("/api/v1/chat/send", {
      method: "POST",
      headers: authHeaders(),
      body: JSON.stringify({ requirement: text }),
    });
    const data = await resp.json();
    if (data.code !== 0) {
      appendMessage("assistant", "推荐失败，请稍后再试。");
      return;
    }
    appendMessage("assistant", data.data.reply || "已生成推荐结果。");
    appendCards(data.data.merchants || []);
  } catch (e) {
    appendMessage("assistant", "网络异常，请稍后重试。");
  }
}

sendBtn.onclick = sendMessage;
promptInput.addEventListener("keydown", (e) => {
  if (e.key === "Enter" && !e.shiftKey) {
    e.preventDefault();
    sendMessage();
  }
});
newChatBtn.onclick = () => createSession();

createSession("默认会话");
appendMessage("assistant", "你好，我是你的外卖助手。告诉我预算、口味、时间，我来推荐。");

function authHeaders() {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };
}

function setAuth(data) {
  token = data.token;
  userId = Number(data.user_id);
  username = data.username || "";
  localStorage.setItem("token", token);
  localStorage.setItem("user_id", String(userId));
  localStorage.setItem("username", username);
}

async function doLogin() {
  const u = authUsername.value.trim();
  const p = authPassword.value;
  if (!u || !p) return;
  const resp = await fetch("/api/v1/user/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: u, password: p }),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    showToast("登录失败，请检查用户名密码", "error");
    return;
  }
  setAuth(data);
  authModal.classList.add("hidden");
  showToast(`欢迎回来，${username}`);
  await initPreferenceOnFirstUse();
}

async function doRegister() {
  const u = regUsername.value.trim();
  const p = regPassword.value;
  const p2 = regPassword2.value;
  if (!u || !p) return;
  if (p.length < 6) {
    showToast("密码至少6位", "error");
    return;
  }
  if (p !== p2) {
    showToast("两次密码不一致", "error");
    return;
  }
  const resp = await fetch("/api/v1/user/register", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ username: u, password: p }),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    showToast("注册失败，用户名可能已存在", "error");
    return;
  }
  showToast("注册成功，请登录", "success");
  registerModal.classList.add("hidden");
  authModal.classList.remove("hidden");
  authUsername.value = u;
  authPassword.value = "";
}

authLoginBtn.onclick = () => doLogin();
authRegisterBtn.onclick = () => {
  authModal.classList.add("hidden");
  registerModal.classList.remove("hidden");
  regUsername.value = authUsername.value.trim();
  regPassword.value = "";
  regPassword2.value = "";
};
regBackBtn.onclick = () => {
  registerModal.classList.add("hidden");
  authModal.classList.remove("hidden");
};
regSubmitBtn.onclick = () => doRegister();

function toggleOption(question, option, btn) {
  const key = question.id;
  if (question.multi) {
    const arr = Array.isArray(prefAnswers[key]) ? prefAnswers[key] : [];
    const idx = arr.indexOf(option);
    if (idx >= 0) {
      arr.splice(idx, 1);
      btn.classList.remove("active");
    } else {
      arr.push(option);
      btn.classList.add("active");
    }
    prefAnswers[key] = arr;
  } else {
    prefAnswers[key] = option;
    prefOptions.querySelectorAll(".pref-option").forEach((el) => el.classList.remove("active"));
    btn.classList.add("active");
  }
}

function renderPrefStep() {
  const q = prefQuestions[prefStep];
  if (!q) return;
  prefQuestion.textContent = `${prefStep + 1}/${prefQuestions.length} ${q.title}`;
  prefOptions.innerHTML = "";
  q.options.forEach((opt) => {
    const btn = document.createElement("button");
    btn.type = "button";
    btn.className = "pref-option";
    btn.textContent = opt;
    const val = prefAnswers[q.id];
    if (q.multi && Array.isArray(val) && val.includes(opt)) {
      btn.classList.add("active");
    }
    if (!q.multi && val === opt) {
      btn.classList.add("active");
    }
    btn.onclick = () => toggleOption(q, opt, btn);
    prefOptions.appendChild(btn);
  });
  prefPrevBtn.disabled = prefStep === 0;
  prefNextBtn.textContent = prefStep === prefQuestions.length - 1 ? "提交偏好" : "下一步";
}

function hasAnswer(question) {
  const val = prefAnswers[question.id];
  if (question.multi) return Array.isArray(val) && val.length > 0;
  return typeof val === "string" && val.trim() !== "";
}

async function submitPreference() {
  const body = {
    spicy_level: prefAnswers.spicy_level,
    budget_range: prefAnswers.budget_range,
    cuisine_likes: prefAnswers.cuisine_likes,
    avoid_foods: prefAnswers.avoid_foods,
    diet_goal: prefAnswers.diet_goal,
    dining_time: prefAnswers.dining_time,
  };
  const resp = await fetch("/api/v1/user/preference", {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify(body),
  });
  const data = await resp.json();
  if (data.code !== 0) throw new Error("save preference failed");
}

prefPrevBtn.onclick = () => {
  if (prefStep > 0) {
    prefStep -= 1;
    renderPrefStep();
  }
};

prefNextBtn.onclick = async () => {
  const q = prefQuestions[prefStep];
  if (!hasAnswer(q)) {
    showToast(`请先完成当前题目：${q.title}`, "error");
    return;
  }
  if (prefStep < prefQuestions.length - 1) {
    prefStep += 1;
    renderPrefStep();
    return;
  }
  try {
    await submitPreference();
    prefModal.classList.add("hidden");
    showToast("偏好已保存");
  } catch (e) {
    showToast("偏好保存失败，请重试", "error");
  }
};

async function initPreferenceOnFirstUse() {
  try {
    const prefResp = await fetch(`/api/v1/user/preference`, { headers: authHeaders() });
    const prefData = await prefResp.json();
    if (prefData.code !== 0) return;
    if (prefData.data.has_preference) return;

    const qResp = await fetch("/api/v1/user/preference/questions", { headers: authHeaders() });
    const qData = await qResp.json();
    if (qData.code !== 0 || !Array.isArray(qData.data) || qData.data.length === 0) return;
    prefQuestions = qData.data;
    prefStep = 0;
    prefModal.classList.remove("hidden");
    renderPrefStep();
  } catch (e) {
  }
}

async function initApp() {
  if (!token || !userId) {
    authModal.classList.remove("hidden");
    return;
  }
  const meResp = await fetch("/api/v1/user/me", { headers: authHeaders() });
  const meData = await meResp.json();
  if (meData.code !== 0) {
    localStorage.removeItem("token");
    localStorage.removeItem("user_id");
    localStorage.removeItem("username");
    token = "";
    userId = 0;
    authModal.classList.remove("hidden");
    return;
  }
  username = meData.data.username;
  appendMessage("assistant", `欢迎回来，${username}。`);
  await initPreferenceOnFirstUse();
}

initApp();
