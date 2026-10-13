import test from "node:test";
import assert from "node:assert/strict";
import { Client, escapeHTML } from "../api.js";
const storage = () => {
  const data = new Map();
  return {
    getItem: (k) => data.get(k),
    setItem: (k, v) => data.set(k, v),
    removeItem: (k) => data.delete(k),
  };
};
test("sesión, If-Match e idempotencia se envían al backend", async () => {
  const c = new Client(storage());
  c.setToken("opaque");
  let calls = [];
  globalThis.fetch = async (url, options) => {
    calls.push({ url, ...options });
    return new Response('{"ok":true}', {
      headers: { "Content-Type": "application/json", ETag: '"new"' },
    });
  };
  await c.patch("/resources/id/content", { content_md: "Hola" }, '"old"');
  assert.equal(calls[0].headers.Authorization, "Bearer opaque");
  assert.equal(calls[0].headers["If-Match"], '"old"');
  await c.post("/enrollments", { course_id: "id" });
  assert.match(calls[1].headers["Idempotency-Key"], /^[0-9a-f-]{36}$/);
});
test("reintento tras fallo de red conserva clave, éxito la libera", async () => {
  const c = new Client(storage());
  const keys = [];
  let failed = true;
  globalThis.fetch = async (_url, opts) => {
    keys.push(opts.headers["Idempotency-Key"]);
    if (failed) {
      failed = false;
      throw new Error("network");
    }
    return new Response("{}");
  };
  await assert.rejects(c.post("/enrollments", { course_id: "id" }));
  await c.post("/enrollments", { course_id: "id" });
  assert.equal(keys[0], keys[1]);
  await c.post("/enrollments", { course_id: "id" });
  assert.notEqual(keys[1], keys[2]);
});
test("error uniforme mantiene detalle y referencia; 401 revoca sesión", async () => {
  const c = new Client(storage());
  c.setToken("expired");
  globalThis.fetch = async () =>
    new Response(
      JSON.stringify({ detail: "Sesión vencida", trace_id: "trace" }),
      { status: 401 },
    );
  await assert.rejects(
    c.get("/me"),
    (err) =>
      err.message === "Sesión vencida" && err.problem.trace_id === "trace",
  );
  assert.equal(c.token, null);
});
test("contenido arbitrario se escapa antes de entrar en HTML", () => {
  assert.equal(
    escapeHTML('<img src=x onerror="alert(1)">'),
    "&lt;img src=x onerror=&quot;alert(1)&quot;&gt;",
  );
});

test("cambiar de página cancela peticiones sin reutilizar respuestas viejas", async () => {
  const c = new Client(storage());
  globalThis.fetch = async (_url, { signal }) => {
    c.beginPage();
    return new Response("{}");
  };
  await assert.rejects(
    c.post("/courses", { title: "Curso" }),
    (err) => err.name === "AbortError",
  );
});
