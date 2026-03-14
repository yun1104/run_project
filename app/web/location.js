const backHomeBtn = document.getElementById("backHomeBtn");
const refreshLocationBtn = document.getElementById("refreshLocationBtn");
const accountBtn = document.getElementById("accountBtn");
const logoutBtn = document.getElementById("logoutBtn");
const locationSummary = document.getElementById("locationSummary");
const nearbyTitle = document.getElementById("nearbyTitle");
const nearbyList = document.getElementById("nearbyList");
const historyTitle = document.getElementById("historyTitle");
const historyList = document.getElementById("historyList");
const toastEl = document.getElementById("toast");
const locationPermModal = document.getElementById("locationPermModal");
const locAllowOnceBtn = document.getElementById("locAllowOnceBtn");
const locAllowAlwaysBtn = document.getElementById("locAllowAlwaysBtn");
const locDenyBtn = document.getElementById("locDenyBtn");

let token = localStorage.getItem("token") || "";
let userId = Number(localStorage.getItem("user_id") || 0);
let toastTimer = null;
let cachedLocation = null;
let locationMode = "none"; // none | once | always | denied
let locationModalResolver = null;
let locationPermissionSynced = false;

function authHeaders() {
  return {
    "Content-Type": "application/json",
    Authorization: `Bearer ${token}`,
  };
}

function showToast(text, type = "success") {
  toastEl.textContent = text;
  toastEl.className = `toast ${type}`;
  toastEl.classList.remove("hidden");
  if (toastTimer) clearTimeout(toastTimer);
  toastTimer = setTimeout(() => toastEl.classList.add("hidden"), 2200);
}

function locationAlwaysKey() {
  return `location_mode_${userId}`;
}

function locationOnceKey() {
  return `location_once_${userId}`;
}

function loginLocationPermKey() {
  return `login_location_perm_${userId}`;
}

async function getServerLocationPermission() {
  const resp = await fetch("/api/v1/user/location-permission", { headers: authHeaders() });
  const data = await resp.json();
  if (data.code !== 0) return "unset";
  const p = String(data.location_permission || "").trim().toLowerCase();
  if (p === "always" || p === "denied") return p;
  return "unset";
}

async function saveServerLocationPermission(permission) {
  const resp = await fetch("/api/v1/user/location-permission", {
    method: "PUT",
    headers: authHeaders(),
    body: JSON.stringify({ location_permission: permission }),
  });
  const data = await resp.json();
  return data.code === 0;
}

function applyServerLocationPermission(permission) {
  const p = String(permission || "").trim().toLowerCase();
  if (p === "always") {
    localStorage.setItem(locationAlwaysKey(), "always");
    localStorage.setItem(loginLocationPermKey(), "always");
    sessionStorage.removeItem(locationOnceKey());
    locationMode = "always";
    return;
  }
  if (p === "denied") {
    localStorage.setItem(locationAlwaysKey(), "denied");
    localStorage.setItem(loginLocationPermKey(), "denied");
    if (sessionStorage.getItem(locationOnceKey()) === "once") {
      locationMode = "once";
      return;
    }
    locationMode = "denied";
    return;
  }
}

function loadLocationMode() {
  const once = sessionStorage.getItem(locationOnceKey());
  if (once === "once") {
    locationMode = "once";
    return;
  }
  const mode = localStorage.getItem(locationAlwaysKey());
  if (mode === "always") {
    locationMode = "always";
    return;
  }
  if (mode === "denied") {
    locationMode = "denied";
    return;
  }
  const loginDenied = localStorage.getItem(loginLocationPermKey());
  if (loginDenied === "denied") {
    locationMode = "denied";
    return;
  }
  locationMode = "none";
}

function hasLocationPermission() {
  return locationMode === "always" || locationMode === "once";
}

function applyLocationChoice(choice) {
  if (choice === "always") {
    localStorage.setItem(locationAlwaysKey(), "always");
    localStorage.removeItem(loginLocationPermKey());
    sessionStorage.removeItem(locationOnceKey());
    locationMode = "always";
    return;
  }
  if (choice === "once") {
    sessionStorage.setItem(locationOnceKey(), "once");
    localStorage.removeItem(loginLocationPermKey());
    localStorage.removeItem(locationAlwaysKey());
    locationMode = "once";
    return;
  }
  localStorage.setItem(locationAlwaysKey(), "denied");
  localStorage.setItem(loginLocationPermKey(), "denied");
  sessionStorage.removeItem(locationOnceKey());
  locationMode = "denied";
}

function openLocationModal() {
  return new Promise((resolve) => {
    locationModalResolver = resolve;
    locationPermModal.classList.remove("hidden");
  });
}

function closeLocationModal(choice) {
  locationPermModal.classList.add("hidden");
  if (locationModalResolver) {
    const fn = locationModalResolver;
    locationModalResolver = null;
    fn(choice);
  }
}

function getCurrentLocation() {
  return new Promise((resolve) => {
    if (cachedLocation) {
      resolve(cachedLocation);
      return;
    }
    if (!navigator.geolocation) {
      resolve(null);
      return;
    }
    navigator.geolocation.getCurrentPosition(
      (pos) => {
        cachedLocation = {
          latitude: Number(pos.coords.latitude.toFixed(6)),
          longitude: Number(pos.coords.longitude.toFixed(6)),
        };
        resolve(cachedLocation);
      },
      () => resolve(null),
      { enableHighAccuracy: true, timeout: 5000, maximumAge: 300000 }
    );
  });
}

async function requestLocationPermissionChoice() {
  const choice = await openLocationModal();
  applyLocationChoice(choice);
  if (choice === "once") {
    await saveServerLocationPermission("unset");
  } else if (choice === "always" || choice === "denied") {
    await saveServerLocationPermission(choice);
  }
  if (choice === "denied") return false;
  return true;
}

async function ensureLocationPermission(forcePrompt = false) {
  if (!locationPermissionSynced) {
    const p = await getServerLocationPermission();
    applyServerLocationPermission(p);
    locationPermissionSynced = true;
  }
  loadLocationMode();
  if (forcePrompt) {
    return await requestLocationPermissionChoice();
  }
  if (locationMode === "denied") return false;
  if (hasLocationPermission()) return true;
  return await requestLocationPermissionChoice();
}

async function ensureLogin() {
  if (!token || !userId) {
    window.location.href = "/";
    return false;
  }
  const meResp = await fetch("/api/v1/user/me", { headers: authHeaders() });
  const meData = await meResp.json();
  if (meData.code !== 0) {
    localStorage.removeItem("token");
    localStorage.removeItem("user_id");
    localStorage.removeItem("username");
    window.location.href = "/";
    return false;
  }
  applyServerLocationPermission(meData.data?.location_permission || "");
  locationPermissionSynced = true;
  return true;
}

async function saveCurrentLocation(loc, source = "location_page") {
  const resp = await fetch("/api/v1/user/location", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({
      latitude: Number(loc.latitude),
      longitude: Number(loc.longitude),
      source,
    }),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    throw new Error(data.message || "保存定位失败");
  }
}

async function queryCurrentLocation() {
  const resp = await fetch("/api/v1/user/location/current?with_nearby=1&radius=3000&limit=8", {
    headers: authHeaders(),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    throw new Error(data.message || "查询定位失败");
  }
  return data.data || {};
}

async function queryLocationHistory() {
  const resp = await fetch("/api/v1/user/location/history?limit=20", {
    headers: authHeaders(),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    throw new Error(data.message || "查询定位历史失败");
  }
  return Array.isArray(data.data?.history) ? data.data.history : [];
}

function renderNearbyFoods(list) {
  nearbyList.innerHTML = "";
  const arr = Array.isArray(list) ? list : [];
  nearbyTitle.textContent = `附近美食（${arr.length}）`;
  if (arr.length === 0) {
    nearbyList.innerHTML = `<div class="order-item"><div class="line">暂无附近美食数据。</div></div>`;
    return;
  }
  arr.forEach((n) => {
    const card = document.createElement("div");
    card.className = "card";
    const dishes = Array.isArray(n.dishes) && n.dishes.length ? n.dishes.join("、") : "附近热门菜";
    card.innerHTML = `
      <h4>${n.name || "附近美食店"}</h4>
      <div class="meta">分类：${n.category || "餐饮服务"}</div>
      <div class="meta">距离：${n.distance_meters || "-"} 米</div>
      <div class="meta">地址：${n.address || "暂无地址信息"}</div>
      <div class="meta">推荐菜：${dishes}</div>
    `;
    nearbyList.appendChild(card);
  });
}

function renderLocationHistory(list) {
  historyList.innerHTML = "";
  const arr = Array.isArray(list) ? list : [];
  historyTitle.textContent = `定位历史（${arr.length}）`;
  if (arr.length === 0) {
    historyList.innerHTML = `<div class="order-item"><div class="line">暂无定位历史。</div></div>`;
    return;
  }
  arr.forEach((h) => {
    const item = document.createElement("div");
    item.className = "order-item";
    item.innerHTML = `
      <div class="line">纬度：${Number(h.latitude || 0).toFixed(6)}，经度：${Number(h.longitude || 0).toFixed(6)}</div>
      <div class="line">来源：${h.source || "unknown"}</div>
      <div class="line">时间：${h.updated_at || "-"}</div>
    `;
    historyList.appendChild(item);
  });
}

async function refreshLocationView() {
  const hasPerm = await ensureLocationPermission();
  if (!hasPerm) {
    locationSummary.textContent = "你未授权定位，无法查询当前位置。";
    renderNearbyFoods([]);
    renderLocationHistory([]);
    return;
  }
  const loc = await getCurrentLocation();
  if (!loc) {
    locationSummary.textContent = "定位失败，请确认浏览器位置权限已开启。";
    renderNearbyFoods([]);
    renderLocationHistory([]);
    return;
  }

  try {
    await saveCurrentLocation(loc, "location_page_browser");
  } catch (_) {}
  let data = {};
  try {
    data = await queryCurrentLocation();
  } catch (_) {
    data = { location: loc, nearby_foods: [] };
  }
  const current = data.location || loc;
  locationSummary.textContent =
    `纬度 ${Number(current.latitude || 0).toFixed(6)}，经度 ${Number(current.longitude || 0).toFixed(6)}，来源：${current.source || "浏览器"}，更新时间：${current.updated_at || "-"}`;
  renderNearbyFoods(data.nearby_foods || []);
  let history = [];
  try {
    history = await queryLocationHistory();
  } catch (_) {}
  renderLocationHistory(history);
}

backHomeBtn.onclick = () => (window.location.href = "/");
accountBtn.onclick = () => (window.location.href = "/account");
logoutBtn.onclick = () => {
  sessionStorage.removeItem(locationOnceKey());
  localStorage.removeItem("token");
  localStorage.removeItem("user_id");
  localStorage.removeItem("username");
  window.location.href = "/";
};
refreshLocationBtn.onclick = async () => {
  refreshLocationBtn.disabled = true;
  try {
    const serverPermission = await getServerLocationPermission();
    applyServerLocationPermission(serverPermission);
    locationPermissionSynced = true;
    const shouldPromptByServer = serverPermission === "denied";
    const hasPerm = shouldPromptByServer
      ? await requestLocationPermissionChoice()
      : await ensureLocationPermission();
    if (!hasPerm) {
      locationSummary.textContent = "你未授权定位，无法查询当前位置。";
      renderNearbyFoods([]);
      renderLocationHistory([]);
      return;
    }
    await refreshLocationView();
    showToast("定位已更新");
  } catch (e) {
    showToast("定位查询失败，请稍后重试", "error");
  } finally {
    refreshLocationBtn.disabled = false;
  }
};

locAllowOnceBtn.onclick = () => closeLocationModal("once");
locAllowAlwaysBtn.onclick = () => closeLocationModal("always");
locDenyBtn.onclick = () => closeLocationModal("denied");

async function init() {
  try {
    const ok = await ensureLogin();
    if (!ok) return;
    loadLocationMode();
    await refreshLocationView();
  } catch (e) {
    showToast("初始化失败，请刷新重试", "error");
  }
}

init();
