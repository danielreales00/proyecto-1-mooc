export class ApiError extends Error {
  constructor(status, problem) {
    super(problem.detail || problem.title || `Error ${status}`);
    this.status = status;
    this.problem = problem;
  }
}
export class Client {
  constructor(storage = sessionStorage) {
    this.storage = storage;
    this.token = storage.getItem("mooc.session");
    this.etags = new Map();
    this.pending = new Map();
    this.controller = new AbortController();
  }
  beginPage() {
    this.controller.abort();
    this.controller = new AbortController();
  }
  setToken(token) {
    this.token = token;
    token
      ? this.storage.setItem("mooc.session", token)
      : this.storage.removeItem("mooc.session");
  }
  async request(path, { method = "GET", body, etag, key, signal } = {}) {
    const headers = { Accept: "application/json" };
    if (this.token) headers.Authorization = `Bearer ${this.token}`;
    if (body !== undefined) headers["Content-Type"] = "application/json";
    const fingerprint = `${method}:${path}:${JSON.stringify(body)}`;
    if (method === "POST") {
      if (!this.pending.has(fingerprint))
        this.pending.set(fingerprint, key || crypto.randomUUID());
      headers["Idempotency-Key"] = this.pending.get(fingerprint);
    }
    if (!signal) signal = this.controller.signal;
    if (etag) headers["If-Match"] = etag;
    let response;
    try {
      response = await fetch(`/api/v1${path}`, {
        method,
        headers,
        body: body === undefined ? undefined : JSON.stringify(body),
        signal,
      });
    } catch (e) {
      if (e.name === "AbortError") throw e;
      throw new Error(
        "No pudimos conectar. Comprueba tu conexión y vuelve a intentar.",
      );
    }
    const data =
      response.status === 204 ? null : await response.json().catch(() => ({}));
    if (signal?.aborted)
      throw new DOMException("Navegación cancelada", "AbortError");
    if (response.ok || (response.status >= 400 && response.status < 500))
      this.pending.delete(fingerprint);
    if (!response.ok) {
      if (response.status === 401 && !path.startsWith("/auth/"))
        this.setToken(null);
      throw new ApiError(response.status, data);
    }
    const tag = response.headers.get("ETag");
    if (tag) this.etags.set(path, tag);
    return data;
  }
  get(path) {
    return this.request(path);
  }
  async all(path) {
    const items = [];
    let cursor = null;
    const seen = new Set();
    do {
      const page = await this.get(
        `${path}${cursor ? `${path.includes("?") ? "&" : "?"}cursor=${encodeURIComponent(cursor)}` : ""}`,
      );
      items.push(...page.items);
      cursor = page.next_cursor;
      if (cursor && seen.has(cursor))
        throw new Error("La paginación no pudo continuar.");
      if (cursor) seen.add(cursor);
    } while (cursor);
    return { items };
  }
  post(path, body, options = {}) {
    return this.request(path, { method: "POST", body, ...options });
  }
  patch(path, body, etag) {
    return this.request(path, { method: "PATCH", body, etag });
  }
}
export const escapeHTML = (value) =>
  String(value ?? "").replace(
    /[&<>"']/g,
    (c) =>
      ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[
        c
      ],
  );
export const safeURL = (value) => {
  try {
    const url = new URL(value, location.origin);
    return ["http:", "https:"].includes(url.protocol) ? url.href : "";
  } catch {
    return "";
  }
};
