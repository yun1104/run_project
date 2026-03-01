const usernameText = document.getElementById("usernameText");
const userIdText = document.getElementById("userIdText");
const backHomeBtn = document.getElementById("backHomeBtn");
const logoutBtn = document.getElementById("logoutBtn");
const openPasswordModalBtn = document.getElementById("openPasswordModalBtn");
const openPreferenceModalBtn = document.getElementById("openPreferenceModalBtn");
const passwordModal = document.getElementById("passwordModal");
const oldPassword = document.getElementById("oldPassword");
const newPassword = document.getElementById("newPassword");
const newPassword2 = document.getElementById("newPassword2");
const passwordCancelBtn = document.getElementById("passwordCancelBtn");
const changePasswordBtn = document.getElementById("changePasswordBtn");
const accountPrefModal = document.getElementById("accountPrefModal");
const prefQuestion = document.getElementById("prefQuestion");
const prefOptions = document.getElementById("prefOptions");
const prefCancelBtn = document.getElementById("prefCancelBtn");
const prefPrevBtn = document.getElementById("prefPrevBtn");
const prefNextBtn = document.getElementById("prefNextBtn");
const toastEl = document.getElementById("toast");

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

function authHeaders() {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };
}

let toastTimer = null;
function showToast(text, type = "success") {
  toastEl.textContent = text;
  toastEl.className = `toast ${type}`;
  toastEl.classList.remove("hidden");
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toastEl.classList.add("hidden"), 2200);
}

function ensureLogin() {
  if (!token || !userId) {
    window.location.href = "/";
    return false;
  }
  return true;
}

async function loadMe() {
  const resp = await fetch("/api/v1/user/me", { headers: authHeaders() });
  const data = await resp.json();
  if (data.code !== 0) {
    localStorage.removeItem("token");
    localStorage.removeItem("user_id");
    localStorage.removeItem("username");
    window.location.href = "/";
    return false;
  }
  username = data.data.username || username;
  userId = Number(data.data.user_id || userId);
  localStorage.setItem("username", username);
  localStorage.setItem("user_id", String(userId));
  usernameText.textContent = username;
  userIdText.textContent = String(userId);
  return true;
}

async function loadPreference() {
  const resp = await fetch("/api/v1/user/preference", { headers: authHeaders() });
  const data = await resp.json();
  if (data.code !== 0 || !data.data || !data.data.has_preference || !data.data.preference) return;
  const p = data.data.preference;
  prefAnswers = {
    spicy_level: p.spicy_level || "",
    budget_range: p.budget_range || "",
    cuisine_likes: Array.isArray(p.cuisine_likes) ? p.cuisine_likes : [],
    avoid_foods: Array.isArray(p.avoid_foods) ? p.avoid_foods : [],
    diet_goal: p.diet_goal || "",
    dining_time: p.dining_time || "",
  };
}

async function loadQuestions() {
  if (prefQuestions.length > 0) return;
  const resp = await fetch("/api/v1/user/preference/questions", { headers: authHeaders() });
  const data = await resp.json();
  if (data.code !== 0 || !Array.isArray(data.data) || data.data.length === 0) {
    throw new Error("load questions failed");
  }
  prefQuestions = data.data;
}

function hasAnswer(question) {
  const val = prefAnswers[question.id];
  if (question.multi) return Array.isArray(val) && val.length > 0;
  return typeof val === "string" && val.trim() !== "";
}

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
    return;
  }
  prefAnswers[key] = option;
  prefOptions.querySelectorAll(".pref-option").forEach((el) => el.classList.remove("active"));
  btn.classList.add("active");
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
    if (q.multi && Array.isArray(val) && val.includes(opt)) btn.classList.add("active");
    if (!q.multi && val === opt) btn.classList.add("active");
    btn.onclick = () => toggleOption(q, opt, btn);
    prefOptions.appendChild(btn);
  });
  prefPrevBtn.disabled = prefStep === 0;
  prefNextBtn.textContent = prefStep === prefQuestions.length - 1 ? "提交偏好" : "下一步";
}

function openPasswordModal() {
  passwordModal.classList.remove("hidden");
}

function closePasswordModal() {
  passwordModal.classList.add("hidden");
  oldPassword.value = "";
  newPassword.value = "";
  newPassword2.value = "";
}

async function changePassword() {
  try {
    const oldPwd = oldPassword.value;
    const newPwd = newPassword.value;
    const newPwd2 = newPassword2.value;
    if (!oldPwd || !newPwd || !newPwd2) {
      showToast("请填写完整密码信息", "error");
      return;
    }
    if (newPwd.length < 6) {
      showToast("新密码至少6位", "error");
      return;
    }
    if (newPwd !== newPwd2) {
      showToast("两次密码不一致", "error");
      return;
    }
    const resp = await fetch("/api/v1/user/password", {
      method: "PUT",
      headers: authHeaders(),
      body: JSON.stringify({
        old_password: oldPwd,
        new_password: newPwd,
      }),
    });
    if (resp.status === 401) {
      showToast("旧密码不对", "error");
      return;
    }
    if (resp.status === 404) {
      showToast("修改密码失败", "error");
      return;
    }

    const text = await resp.text();
    let data = {};
    try {
      data = text ? JSON.parse(text) : {};
    } catch (e) {
      data = {};
    }

    if (!resp.ok) {
      const rawMsg = String(data.message || "");
      if (rawMsg.includes("old password") || rawMsg.includes("旧密码")) {
        showToast("旧密码不对", "error");
      } else {
        showToast(rawMsg || "修改密码失败", "error");
      }
      return;
    }

    if (Number(data.code || 0) !== 0) {
      const rawMsg = String(data.message || "");
      if (rawMsg.includes("old password") || rawMsg.includes("旧密码")) {
        showToast("旧密码不对", "error");
      } else {
        showToast(rawMsg || "修改密码失败", "error");
      }
      return;
    }
    closePasswordModal();
    showToast("密码修改成功");
  } catch (e) {
    showToast("修改密码失败", "error");
  }
}

async function openPreferenceModal() {
  await loadQuestions();
  await loadPreference();
  prefStep = 0;
  renderPrefStep();
  accountPrefModal.classList.remove("hidden");
}

function closePreferenceModal() {
  accountPrefModal.classList.add("hidden");
}

async function submitPreference() {
  for (const q of prefQuestions) {
    if (!hasAnswer(q)) {
      showToast(`请先完成当前题目：${q.title}`, "error");
      return false;
    }
  }
  const resp = await fetch("/api/v1/user/preference", {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify({
      spicy_level: prefAnswers.spicy_level,
      budget_range: prefAnswers.budget_range,
      cuisine_likes: prefAnswers.cuisine_likes,
      avoid_foods: prefAnswers.avoid_foods,
      diet_goal: prefAnswers.diet_goal,
      dining_time: prefAnswers.dining_time,
    }),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    showToast("偏好保存失败，请重试", "error");
    return false;
  }
  showToast("偏好已更新");
  return true;
}

backHomeBtn.onclick = () => {
  window.location.href = "/";
};

logoutBtn.onclick = () => {
  localStorage.removeItem("token");
  localStorage.removeItem("user_id");
  localStorage.removeItem("username");
  window.location.href = "/";
};

openPasswordModalBtn.onclick = () => {
  openPasswordModal();
};

passwordCancelBtn.onclick = () => {
  closePasswordModal();
};

changePasswordBtn.onclick = () => {
  changePassword();
};

openPreferenceModalBtn.onclick = () => {
  openPreferenceModal().catch(() => showToast("问卷加载失败", "error"));
};

prefCancelBtn.onclick = () => {
  closePreferenceModal();
};

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
  const ok = await submitPreference();
  if (ok) closePreferenceModal();
};

passwordModal.addEventListener("click", (e) => {
  if (e.target === passwordModal) closePasswordModal();
});

accountPrefModal.addEventListener("click", (e) => {
  if (e.target === accountPrefModal) closePreferenceModal();
});

async function init() {
  if (!ensureLogin()) return;
  const ok = await loadMe();
  if (!ok) return;
}

init().catch(() => showToast("初始化失败，请刷新重试", "error"));
