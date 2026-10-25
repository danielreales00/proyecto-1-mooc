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
    const fingerprint = `${method}:${path}:${JSON.stringify(body)}`;
    const headers = this.headers(method, body, etag, key, fingerprint);
    signal ??= this.controller.signal;
    const response = await send(path, method, headers, body, signal);
    const data =
      response.status === 204 ? null : await response.json().catch(() => ({}));
    if (signal.aborted)
      throw new DOMException("Navegación cancelada", "AbortError");
    if (response.status < 500) this.pending.delete(fingerprint);
    if (!response.ok) {
      if (response.status === 401 && !path.startsWith("/auth/"))
        this.setToken(null);
      throw new ApiError(response.status, data);
    }
    const tag = response.headers.get("ETag");
    if (tag) this.etags.set(path, tag);
    return data;
  }
  // Un POST repetido tras un fallo de red conserva su Idempotency-Key: el
  // servidor lo reconoce y no lo aplica dos veces.
  headers(method, body, etag, key, fingerprint) {
    const headers = { Accept: "application/json" };
    if (this.token) headers.Authorization = `Bearer ${this.token}`;
    if (body !== undefined) headers["Content-Type"] = "application/json";
    if (method === "POST") {
      if (!this.pending.has(fingerprint))
        this.pending.set(fingerprint, key || crypto.randomUUID());
      headers["Idempotency-Key"] = this.pending.get(fingerprint);
    }
    if (etag) headers["If-Match"] = etag;
    return headers;
  }
  get(path) {
    return this.request(path);
  }
  // Recorre todas las páginas siguiendo el cursor, una tras otra.
  async all(path, cursor = null, items = [], seen = new Set()) {
    const page = await this.get(withCursor(path, cursor));
    items.push(...page.items);
    const next = page.next_cursor;
    if (!next) return { items };
    if (seen.has(next)) throw new Error("La paginación no pudo continuar.");
    seen.add(next);
    return this.all(path, next, items, seen);
  }
  post(path, body, options = {}) {
    return this.request(path, { method: "POST", body, ...options });
  }
  patch(path, body, etag) {
    return this.request(path, { method: "PATCH", body, etag });
  }
}
async function send(path, method, headers, body, signal) {
  try {
    return await fetch(`/api/v1${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    });
  } catch (err) {
    if (err.name === "AbortError") throw err;
    throw new Error(
      "No pudimos conectar. Comprueba tu conexión y vuelve a intentar.",
    );
  }
}
function withCursor(path, cursor) {
  if (!cursor) return path;
  const separator = path.includes("?") ? "&" : "?";
  return path + separator + "cursor=" + encodeURIComponent(cursor);
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
