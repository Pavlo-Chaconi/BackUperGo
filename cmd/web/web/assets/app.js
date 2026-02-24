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
