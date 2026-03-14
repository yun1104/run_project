const backHomeBtn = document.getElementById("backHomeBtn");
const refreshBtn = document.getElementById("refreshBtn");
const accountBtn = document.getElementById("accountBtn");
const logoutBtn = document.getElementById("logoutBtn");
const orderTitle = document.getElementById("orderTitle");
const orderList = document.getElementById("orderList");
const toastEl = document.getElementById("toast");
const locationPermModal = document.getElementById("locationPermModal");
const locAllowOnceBtn = document.getElementById("locAllowOnceBtn");
const locAllowAlwaysBtn = document.getElementById("locAllowAlwaysBtn");
const locDenyBtn = document.getElementById("locDenyBtn");

let token = localStorage.getItem("token") || "";
let userId = Number(localStorage.getItem("user_id") || 0);
let timer = null;
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

function formatMoney(v) {
  return Number(v || 0).toFixed(2);
}

function formatOrderStatus(status) {
  const map = {
    created: "待处理",
    paid: "已支付",
    accepted: "商家已接单",
    delivering: "配送中",
    delivered: "已送达",
    cancelled: "已取消",
  };
  return map[String(status || "").toLowerCase()] || (status || "未知");
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
    sessionStorage.removeItem(locationOnceKey());
    locationMode = "denied";
    return;
  }
}

function loadLocationMode() {
  const loginDenied = localStorage.getItem(loginLocationPermKey());
  if (loginDenied === "denied") {
    locationMode = "denied";
    return;
  }
  const always = localStorage.getItem(locationAlwaysKey());
  if (always === "denied") {
    locationMode = "denied";
    return;
  }
  if (always === "always") {
    locationMode = "always";
    return;
  }
  const once = sessionStorage.getItem(locationOnceKey());
  if (once === "once") {
    locationMode = "once";
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
      { enableHighAccuracy: true, timeout: 3000, maximumAge: 300000 }
    );
  });
}

async function requestLocationPermissionChoice(reason = "login") {
  const choice = await openLocationModal();
  applyLocationChoice(choice);
  if (choice === "once") {
    await saveServerLocationPermission("unset");
  } else if (choice === "always" || choice === "denied") {
    await saveServerLocationPermission(choice);
  }
  if (choice === "denied") {
    if (reason === "login") {
      showToast("你已选择不允许定位，可在下单时再次授权", "error");
    }
    return false;
  }
  const loc = await getCurrentLocation();
  if (!loc) {
    showToast("定位授权已选择，但暂未成功获取坐标", "error");
  }
  return true;
}

async function initLocationPermissionOnLogin() {
  if (!locationPermissionSynced) {
    const p = await getServerLocationPermission();
    applyServerLocationPermission(p);
    locationPermissionSynced = true;
  }
  loadLocationMode();
  if (hasLocationPermission()) {
    await getCurrentLocation();
    return;
  }
  await requestLocationPermissionChoice("login");
}

async function ensureLocationPermissionForOrder() {
  if (!locationPermissionSynced) {
    const p = await getServerLocationPermission();
    applyServerLocationPermission(p);
    locationPermissionSynced = true;
  }
  loadLocationMode();
  if (locationMode === "denied") return false;
  if (hasLocationPermission()) return true;
  return await requestLocationPermissionChoice("order");
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

async function fetchOrders() {
  const resp = await fetch("/api/v1/order/list", { headers: authHeaders() });
  const data = await resp.json();
  if (data.code !== 0) {
    throw new Error(data.message || "获取订单失败");
  }
  const list = Array.isArray(data.data?.orders) ? data.data.orders.slice() : [];
  list.sort((a, b) => new Date(b.created_at).getTime() - new Date(a.created_at).getTime());
  return list;
}

async function cancelOrder(orderId) {
  const ok = window.confirm(`确认取消订单 ${orderId} 吗？`);
  if (!ok) return false;
  const resp = await fetch("/api/v1/order/cancel", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ order_id: orderId }),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    showToast(data.message || "取消失败", "error");
    return false;
  }
  showToast("订单已取消");
  return true;
}

async function reorderOrder(orderId) {
  const ok = window.confirm(`确认为订单 ${orderId} 再来一单吗？`);
  if (!ok) return false;
  const hasPerm = await ensureLocationPermissionForOrder();
  if (!hasPerm) {
    showToast("你未授权位置权限，本次暂不下单。", "error");
    return false;
  }
  const resp = await fetch("/api/v1/order/reorder", {
    method: "POST",
    headers: authHeaders(),
    body: JSON.stringify({ order_id: orderId, auto_pay: true }),
  });
  const data = await resp.json();
  if (data.code !== 0) {
    showToast(data.message || "再来一单失败", "error");
    return false;
  }
  showToast("已成功再来一单");
  return true;
}

function renderOrders(orders) {
  orderTitle.textContent = `订单列表（${orders.length}）`;
  orderList.innerHTML = "";
  if (orders.length === 0) {
    orderList.innerHTML = `<div class="order-item"><div class="line">暂无订单。</div></div>`;
    return;
  }

  orders.forEach((o) => {
    const item = document.createElement("div");
    item.className = "order-item";
    const dishes = Array.isArray(o.items) && o.items.length
      ? o.items.map((x) => `${x.name}x${x.quantity}`).join("、")
      : "商家默认套餐";
    item.innerHTML = `
      <div class="title">#${o.order_id} ${o.merchant_name}</div>
      <div class="line">状态：${formatOrderStatus(o.status)} | 金额：￥${formatMoney(o.amount)}</div>
      <div class="line">下单时间：${o.created_at || "-"}</div>
      <div class="line">菜品：${dishes}</div>
      <div class="line">地址：${o.delivery_address || "未填写"}</div>
      <div class="line">备注：${o.remark || "无"}</div>
      <div class="line">预计送达：${o.estimated_minutes || "-"} 分钟</div>
    `;
    orderList.appendChild(item);
  });
}

async function refreshOrders() {
  try {
    const orders = await fetchOrders();
    renderOrders(orders);
  } catch (e) {
    showToast("获取订单失败，请稍后重试", "error");
  }
}

function startAutoRefresh() {
  if (timer) clearInterval(timer);
  timer = setInterval(() => refreshOrders(), 10000);
}

function stopAutoRefresh() {
  if (timer) {
    clearInterval(timer);
    timer = null;
  }
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
refreshBtn.onclick = () => refreshOrders();
window.addEventListener("beforeunload", () => stopAutoRefresh());
locAllowOnceBtn.onclick = () => closeLocationModal("once");
locAllowAlwaysBtn.onclick = () => closeLocationModal("always");
locDenyBtn.onclick = () => closeLocationModal("denied");

async function init() {
  const ok = await ensureLogin();
  if (!ok) return;
  loadLocationMode();
  await initLocationPermissionOnLogin();
  await refreshOrders();
  startAutoRefresh();
}

init().catch(() => showToast("初始化失败，请刷新重试", "error"));
