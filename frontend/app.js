import { Client, escapeHTML as e, safeURL } from "./api.js";
const api = new Client();
const main = document.querySelector("#main");
let user = null,
  cleanup = async () => {},
  pageRun = 0;
const $ = (s, root = document) => root.querySelector(s);
const link = (url, text, cls = "button") =>
  `<a class="${cls}" href="#${url}">${text}</a>`;
const field = (name, label, value = "", type = "text", required = true) =>
  `<label>${label}<input name="${name}" type="${type}" value="${e(value)}" ${required ? "required" : ""}></label>`;
const option = (value, selected) =>
  `<option value="${e(value)}" ${value === selected ? "selected" : ""}>${e(value ? status(value) : "Todos")}</option>`;
const select = (name, label, values, value) =>
  `<label>${label}<select name="${name}" aria-label="${e(label)}">${values.map((v) => option(v, value)).join("")}</select></label>`;
const check = (name, label, value = true) =>
  `<label class="check"><input type="checkbox" name="${name}" ${value ? "checked" : ""}>${label}</label>`;
const button = (text, action, cls = "primary") =>
  `<button type="button" class="${cls}" data-action="${action}">${text}</button>`;
const empty = (text) => `<div class="empty">${text}</div>`;
const title = (label, heading, desc = "") => {
  const intro = desc ? '<p class="muted">' + e(desc) + "</p>" : "";
  return `<div class="eyebrow">${label}</div><h1>${e(heading)}</h1>${intro}`;
};
const date = (d) => (d ? new Date(d).toLocaleDateString("es-CO") : "—");
const status = (s) =>
  ({
    active: "Activo",
    draft: "Borrador",
    published: "Publicado",
    withdrawn: "Retirado",
    approved: "Aprobado",
    in_progress: "En curso",
    completed: "Completado",
    pending_verification: "Por verificar",
    revoked: "Revocada",
    valid: "Válida",
    ready: "Listo",
    submitted: "Entregado",
    not_started: "Sin comenzar",
    clean: "Verificado",
    uploading: "Cargando",
    uploaded: "Recibido",
    scanning: "En revisión",
    processing: "Procesando",
    failed: "Fallido",
    rejected: "Rechazado",
    infected: "Archivo bloqueado",
    unpublished: "Retirado",
    suspended: "Suspendido",
    deleted: "Eliminado",
    student: "Estudiante",
    teacher: "Docente",
    admin: "Administrador",
    rich_text: "Lectura",
    video: "Video",
    audio: "Audio",
    image: "Imagen",
    pdf: "PDF",
    slides: "Presentación",
    download: "Archivo",
    file: "Archivo",
    iframe: "Contenido integrado",
    link: "Enlace",
    quiz: "Evaluación",
    single: "Una respuesta",
    multiple: "Varias respuestas",
    none: "Sin retroalimentación",
    score_only: "Solo puntaje",
    correctness: "Corrección",
    full: "Completa",
    es: "Español",
    en: "Inglés",
  })[s] ||
  s ||
  "—";
function notice(text) {
  const el = $("#notice");
  el.textContent = text;
  el.className = "show";
  clearTimeout(notice.timer);
  notice.timer = setTimeout(() => (el.className = ""), 6000);
}
const fieldErrorHTML = (x) => `<li>${e(x.field)}: ${e(x.detail)}</li>`;
function errorHTML(err) {
  const trace = err.problem?.trace_id
    ? '<p class="meta">Referencia: ' + e(err.problem.trace_id) + "</p>"
    : "";
  const fields = err.problem?.errors?.length
    ? "<ul>" + err.problem.errors.map(fieldErrorHTML).join("") + "</ul>"
    : "";
  return `<div class="error" role="alert">${e(err.message)}${trace}${fields}</div>`;
}
function bindForm(id, fn) {
  const form = $(id);
  if (!form) return;
  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const submit = form.querySelector("[type=submit]");
    if (submit?.disabled) return;
    if (submit) submit.disabled = true;
    let out = form.querySelector(".form-error");
    if (!out) {
      out = document.createElement("div");
      out.className = "form-error";
      form.append(out);
    }
    out.innerHTML = "";
    try {
      await fn(Object.fromEntries(new FormData(form)), form);
    } catch (err) {
      if (err.name !== "AbortError") out.innerHTML = errorHTML(err);
    } finally {
      if (submit) submit.disabled = false;
    }
  });
}
function action(name, fn) {
  document.querySelectorAll(`[data-action="${name}"]`).forEach(
    (el) =>
      (el.onclick = async () => {
        el.disabled = true;
        try {
          await fn(el);
        } catch (err) {
          if (err.name !== "AbortError") notice(err.message);
        } finally {
          el.disabled = false;
        }
      }),
  );
}
function show(html) {
  main.innerHTML = `<div class="wrap">${html}</div>`;
}
function modal(html) {
  $("#modal-body").innerHTML = html;
  $("#modal").showModal();
}
$("#close-modal").onclick = () => $("#modal").close();
function navItems() {
  const items = [["/catalog", "Explorar cursos"]];
  if (user)
    items.push(["/learning", "Mi aprendizaje"], ["/badges", "Mis insignias"]);
  if (["teacher", "admin"].includes(user?.role))
    items.push(["/studio", "Estudio"]);
  if (user?.role === "admin") items.push(["/admin", "Administración"]);
  return items;
}
function navLinkHTML([path, text], route) {
  const active = route.startsWith(path)
    ? 'class="active" aria-current="page"'
    : "";
  return `<a href="#${path}" ${active}>${text}</a>`;
}
function sessionActionsHTML() {
  if (user)
    return (
      "<span>" +
      e(user.full_name) +
      "</span>" +
      button("Salir", "logout", "text")
    );
  return (
    link("/login", "Iniciar sesión", "text") + link("/register", "Crear cuenta")
  );
}
function header() {
  const actual = location.hash.slice(1) || "/";
  const nav = navItems()
    .map((item) => navLinkHTML(item, actual))
    .join("");
  $("#header").innerHTML =
    `<a class="brand" href="#/" aria-label="MOOC inicio">mooc<span>✳</span></a><nav aria-label="Principal">${nav}</nav><div class="actions">${sessionActionsHTML()}</div>`;
  action("logout", async () => {
    await cleanup();
    cleanup = async () => {};
    await api.post("/auth/logout");
    api.setToken(null);
    user = null;
    location.hash = "/";
    await route();
  });
}
function requireUser(roles) {
  if (!user) {
    location.hash = "/login";
    return false;
  }
  if (roles && !roles.includes(user.role)) {
    show(
      title("Acceso", "Esta sección requiere otro rol") +
        link("/", "Volver al inicio"),
    );
    return false;
  }
  return true;
}
function cardHTML(c, i) {
  const explore = link(
    "/course/" + encodeURIComponent(c.slug),
    "Explorar curso ↗",
    "text",
  );
  return `<article class="card"><div class="card-art"><span>${e(c.language?.toUpperCase() || "CURSO")}</span><b aria-hidden="true">${["↗", "◉", "✳"][i % 3]}</b></div><span class="chip">${e(c.category || "Aprendizaje")}</span><h3>${e(c.title)}</h3><p>${e(c.summary)}</p><div class="meta">${e(c.enrolled_count ?? 0)} estudiantes · Versión ${e(c.version_number)}</div><div class="actions">${explore}</div></article>`;
}
function cards(items) {
  if (!items.length)
    return empty("Aún no hay cursos publicados que coincidan con tu búsqueda.");
  return `<div class="grid">${items.map(cardHTML).join("")}</div>`;
}
async function home() {
  show(
    `<section class="hero"><div class="eyebrow">CONOCIMIENTO SIN LÍMITES</div><h1>Aprender nos<br>transforma.</h1><p class="lead">Una nueva habilidad. Otra perspectiva. Tu próximo paso empieza con lo que aprendes hoy.</p><div class="actions" style="justify-content:center">${link("/catalog", "Encuentra tu próximo curso ↗")}${link(user ? "/learning" : "/register", user ? "Continúa aprendiendo" : "Empieza a aprender", "text")}</div></section><div class="split"><section class="feature"><div class="eyebrow">A TU RITMO</div><h2>Grandes ideas.<br>Pequeños pasos.</h2><p>Avanza con lecturas, videos y actividades. Retoma tu aprendizaje donde lo dejaste.</p><div class="orbit" aria-hidden="true"></div></section><section class="feature stone"><span class="big-mark" aria-hidden="true">✳</span><div class="eyebrow">LOGROS QUE CUENTAN</div><h2>Haz visible<br>lo que sabes.</h2><p>Completa tus cursos y comparte insignias que cualquiera puede verificar.</p>${link("/verify", "Verificar una insignia ↗", "text")}</section></div><div class="section-head"><div><div class="eyebrow">EXPLORA POSIBILIDADES</div><h2>El siguiente capítulo es tuyo.</h2></div>${link("/catalog", "Ver todos los cursos ↗", "text")}</div><div id="courses" class="loading">Cargando cursos</div>`,
  );
  try {
    const d = await api.get("/catalog/courses?limit=3");
    if ($("#courses")) $("#courses").outerHTML = cards(d.items);
  } catch (err) {
    if ($("#courses")) $("#courses").innerHTML = errorHTML(err);
  }
}
async function catalog() {
  show(
    title(
      "Catálogo",
      "Siempre hay algo<br>por descubrir.".replace("<br>", " "),
      "Explora el conocimiento y encuentra un curso para tu próximo paso.",
    ) +
      `<form id="filters" class="filters">${field("q", "Buscar cursos", "", "search", false)}${field("category", "Categoría", "", "text", false)}${select("language", "Idioma", ["", "es", "en"], "")}<button class="primary" type="submit">Buscar ↗</button></form><div id="catalog-list"></div><div id="more"></div>`,
  );
  let query = "",
    cursor = null;
  const load = async (append = false) => {
    const target = $("#catalog-list");
    if (!append)
      target.innerHTML = '<div class="loading">Cargando cursos</div>';
    try {
      const more =
        append && cursor ? "&cursor=" + encodeURIComponent(cursor) : "";
      const d = await api.get(`/catalog/courses?limit=12&${query}${more}`);
      if (append) target.insertAdjacentHTML("beforeend", cards(d.items));
      else target.innerHTML = cards(d.items);
      cursor = d.next_cursor;
      $("#more").innerHTML = cursor
        ? button("Cargar más cursos", "more", "outline")
        : "";
      action("more", () => load(true));
    } catch (err) {
      target.innerHTML = errorHTML(err);
    }
  };
  bindForm("#filters", async (data) => {
    query = new URLSearchParams(data).toString();
    await load();
  });
  await load();
}
const unitTitleHTML = (u) => `<p>${e(u.title)}</p>`;
function outlineModuleHTML(m, i) {
  return `<section class="row"><div><span class="meta">MÓDULO ${String(i + 1).padStart(2, "0")}</span><h3>${e(m.title)}</h3>${m.units.map(unitTitleHTML).join("")}</div><span aria-hidden="true">↗</span></section>`;
}
async function course(slug) {
  const c = await api.get(`/catalog/courses/${encodeURIComponent(slug)}`);
  const enroll = user
    ? button("Inscribirme al curso ↗", "enroll")
    : link("/login", "Inicia sesión para inscribirte");
  const outline =
    (c.outline || []).map(outlineModuleHTML).join("") ||
    empty("El temario estará disponible pronto.");
  show(
    `${link("/catalog", "← Catálogo", "text")}<div class="split"><div>${title(c.category, c.title, c.summary)}<div class="meta">${e(c.language)} · ${e(c.enrolled_count)} estudiantes · Versión ${e(c.version_number)}</div><div class="actions" style="margin-top:28px">${enroll}</div></div><div class="feature"><div class="eyebrow">TU PRÓXIMO PASO</div><h2>Aprende.<br>Practica.<br>Avanza.</h2><div class="orbit" aria-hidden="true"></div></div></div><h2>Tu recorrido</h2>${outline}`,
  );
  action("enroll", async () => {
    const en = await api.post("/enrollments", { course_id: c.id });
    location.hash = `/learn/${en.id}`;
  });
}
const AUTH = {
  login: { heading: "Qué bueno verte de nuevo.", submit: "Iniciar sesión ↗" },
  register: {
    heading: "Abre nuevas posibilidades.",
    submit: "Crear mi cuenta ↗",
  },
  "verify-email": { heading: "Verifica tu correo", submit: "Verificar correo" },
};
function authFieldsHTML(kind) {
  if (kind === "verify-email")
    return field(
      "token",
      "Código de verificación",
      new URLSearchParams(location.search).get("token") || "",
    );
  const name = kind === "register" ? field("full_name", "Nombre completo") : "";
  return (
    name +
    field("email", "Correo electrónico", "", "email") +
    field("password", "Contraseña", "", "password")
  );
}
async function auth(kind) {
  const register = kind === "register",
    verify = kind === "verify-email",
    texts = AUTH[kind];
  const other = register
    ? link("/login", "Ya tengo una cuenta", "text")
    : link("/register", "Crear cuenta", "text");
  show(
    `<div class="form-panel">${title("TU ESPACIO", texts.heading)}<form id="auth" class="panel">${authFieldsHTML(kind)}<button class="primary" type="submit">${texts.submit}</button></form><div class="actions">${other}${link("/verify-email", "Verificar mi correo", "text")}</div></div>`,
  );
  bindForm("#auth", async (data) => {
    const d = await api.post("/auth/" + kind, data);
    if (register) {
      show(
        `<div class="form-panel">${title("CASI LISTO", "Revisa tu correo.")}<p>Te enviamos un código para activar tu cuenta.</p>${link("/verify-email", "Verificar mi correo")}</div>`,
      );
    } else if (verify) {
      notice("Correo verificado. Ya puedes iniciar sesión.");
      location.hash = "/login";
    } else {
      api.setToken(d.token);
      user = d.user;
      location.hash = "/learning";
    }
  });
}
async function learning() {
  if (!requireUser()) return;
  const [d, cat] = await Promise.all([
    api.get("/enrollments"),
    api.all("/catalog/courses?limit=100"),
  ]);
  show(
    title(
      "Mi aprendizaje",
      `Sigue avanzando, ${user.full_name.split(" ")[0]}.`,
      "Cada paso cuenta. Retoma tus cursos y descubre lo que viene.",
    ) +
      (d.items.length
        ? d.items
            .map((en) => {
              const c = cat.items.find((c) => c.id === en.course_id);
              const next =
                en.status === "withdrawn"
                  ? '<span class="muted">Inscripción retirada</span>'
                  : link("/learn/" + en.id, "Continuar ↗");
              return `<article class="row"><div><span class="chip">${e(status(en.state))}</span><h3>${e(c?.title || "Curso inscrito")}</h3><div class="progress"><span style="width:${Math.max(0, Math.min(100, Number(en.progress_pct) || 0))}%"></span></div><span class="meta">${e(en.progress_pct)}% completado · ${date(en.enrolled_at)}</span></div><div class="actions">${next}</div></article>`;
            })
            .join("")
        : empty("Todavía no tienes cursos. Tu próximo capítulo te espera.")) +
      link("/catalog", "Explorar cursos", "text"),
  );
}
const resourceNavHTML = (r, i) =>
  `<button data-resource="${i}">${e(r.title)}<br><span class="meta">${e(r.type)} · ${e(status(r.progress.state))}</span></button>`;
function attemptRowHTML(a) {
  const label = a.status === "in_progress" ? "Continuar" : "Ver resultado";
  return `<div class="row"><span>Intento ${e(a.attempt_number)} · ${e(status(a.status))}</span>${link("/attempt/" + a.id, label, "text")}</div>`;
}
function externalHTML(r) {
  const url = safeURL(r.external_url);
  if (r.type === "iframe" && url.startsWith("https:"))
    return `<iframe src="${e(url)}" title="${e(r.title)}" sandbox="allow-scripts allow-same-origin allow-presentation" referrerpolicy="no-referrer" style="width:100%;height:480px;border:1px solid var(--line)"></iframe>`;
  return `<a class="button" href="${e(url)}" target="_blank" rel="noopener noreferrer">Abrir recurso ↗</a>`;
}
function assetHTML(r, url) {
  if (r.type === "image")
    return `<div class="reading"><img src="${e(url)}" alt="${e(r.title)}"></div>`;
  if (r.type === "pdf")
    return `<iframe title="${e(r.title)}" src="${e(url)}" style="width:100%;height:600px;border:1px solid var(--line)"></iframe><a class="text" href="${e(url)}" target="_blank" rel="noopener noreferrer">Abrir documento ↗</a>`;
  return `<a class="button" href="${e(url)}" target="_blank" rel="noopener noreferrer">Abrir archivo ↗</a>`;
}
function showPlaybackError(player, data) {
  if (!data.fatal) return;
  player.dataset.playbackError = data.details;
  player.dataset.playbackErrorType = data.type;
  const info = $("#video-info");
  if (info)
    info.textContent =
      data.type === "mediaError"
        ? "Este navegador no puede reproducir el formato de este recurso."
        : "No se pudo reproducir. Vuelve a abrir el recurso para renovar el acceso.";
}
// Conecta el reproductor: hls.js sobre MediaSource cuando el navegador
// decodifica H.264/AAC, o HLS nativo. Devuelve la instancia de hls.js o null.
function attachPlayer(player, type, url) {
  const mse = window.MediaSource || window.ManagedMediaSource;
  const codecs =
    type === "audio"
      ? 'audio/mp4; codecs="mp4a.40.2"'
      : 'video/mp4; codecs="avc1.4d401f,mp4a.40.2"';
  if (window.Hls?.isSupported() && mse?.isTypeSupported(codecs)) {
    const hls = new window.Hls({ enableWorker: false });
    hls.loadSource(url);
    hls.attachMedia(player);
    hls.on(window.Hls.Events.ERROR, (_event, data) =>
      showPlaybackError(player, data),
    );
    return hls;
  }
  if (player.canPlayType("application/vnd.apple.mpegurl")) {
    player.src = url;
    return null;
  }
  throw new Error("Este navegador no admite la reproducción de video.");
}
// Retoma la reproducción con una credencial nueva, en la misma posición.
function resumePlayback(hls, player, url, position, playing) {
  const resume = () => {
    if (playing) void player.play().catch(() => {});
  };
  if (hls) {
    hls.config.startPosition = position;
    hls.once(window.Hls.Events.MANIFEST_PARSED, () => {
      hls.startLoad(position);
      resume();
    });
    hls.loadSource(url);
    return;
  }
  player.addEventListener(
    "loadedmetadata",
    () => {
      player.currentTime = position;
      resume();
    },
    { once: true },
  );
  player.src = url;
}
async function learn(id) {
  if (!requireUser()) return;
  const activePage = pageRun;
  const d = await api.get(`/enrollments/${id}/content`);
  const nav = d.resources.map(resourceNavHTML).join("");
  show(
    `${link("/learning", "← Mi aprendizaje", "text")}<div class="title-line">${title("Aula", "Un paso más cerca.")}<div><span id="progress" class="chip">${e(d.enrollment.progress_pct)}% completado</span>${button("Retirarme", "withdraw", "text danger")}</div></div><div class="workspace"><aside class="sidebar" aria-label="Recursos del curso">${nav}</aside><article id="reader">${empty("Selecciona un recurso para comenzar.")}</article></div>`,
  );
  let current = null,
    timer = null,
    video = null,
    hls = null,
    renewTimer = null;
  async function evidence(kind) {
    if (!current) return;
    const resource = current;
    const position = video?.currentTime;
    const res = await api.post(`/enrollments/${id}/progress`, {
      resource_stable_id: resource.stable_id,
      kind,
      ...(video && kind !== "open" ? { position_seconds: position } : {}),
    });
    if (activePage !== pageRun) return;
    if (res.accepted && position !== undefined && kind !== "open")
      resource.progress.last_position_seconds = position;
    if ($("#progress"))
      $("#progress").textContent = `${res.progress_pct}% completado`;
    const item = document.querySelector(
      `[data-resource="${d.resources.indexOf(resource)}"] .meta`,
    );
    if (item)
      item.textContent = `${status(resource.type)} · ${status(res.resource_state)}`;
  }
  cleanup = () => {
    clearInterval(timer);
    clearTimeout(renewTimer);
    const finalEvidence = current
      ? evidence("close").catch(() => {})
      : Promise.resolve();
    hls?.destroy();
    return finalEvidence;
  };
  // La credencial de reproducción vence: se renueva un minuto antes, entre
  // 5 s y 10 min, sin perder la posición.
  function scheduleRenewal(resource, player, access) {
    clearTimeout(renewTimer);
    const delay = Math.min(
      10 * 60 * 1000,
      Math.max(
        5000,
        new Date(access.expires_at).getTime() - Date.now() - 60000,
      ),
    );
    renewTimer = setTimeout(() => renew(resource, player), delay);
  }
  async function renew(resource, player) {
    try {
      const position = player.currentTime,
        playing = !player.paused;
      const access = await api.post(`/enrollments/${id}/media-sessions`, {
        resource_stable_id: resource.stable_id,
      });
      if (current !== resource || video !== player || !player.isConnected)
        return;
      resumePlayback(
        hls,
        player,
        safeURL(access.manifest_url),
        position,
        playing,
      );
      scheduleRenewal(resource, player, access);
    } catch (err) {
      if (player.isConnected) notice(err.message);
    }
  }
  async function renderQuiz(content) {
    const base = `/enrollments/${id}/quizzes/${current.stable_id}/attempts`;
    const history = await api.get(base);
    const rows = (history.items || []).map(attemptRowHTML).join("");
    content.innerHTML = `<p>Comprueba lo que aprendiste. Tus respuestas se guardan antes de entregar.</p>${button("Iniciar intento", "start-quiz")}<h3>Intentos anteriores</h3>${rows}`;
    action("start-quiz", async () => {
      const a = await api.post(base);
      location.hash = `/attempt/${a.id}`;
    });
  }
  async function playMedia(content) {
    const session = await api.post(`/enrollments/${id}/media-sessions`, {
      resource_stable_id: current.stable_id,
    });
    content.innerHTML = `<video controls playsinline aria-label="${e(current.title)}"></video><p class="muted" id="video-info">Reproduce el contenido para registrar tu avance.</p>`;
    video = $("video");
    hls = attachPlayer(video, current.type, safeURL(session.manifest_url));
    scheduleRenewal(current, video, session);
    video.addEventListener(
      "loadedmetadata",
      () => {
        video.currentTime = Number(current.progress.last_position_seconds) || 0;
      },
      { once: true },
    );
    video.addEventListener("error", () => {
      if ($("#video-info"))
        $("#video-info").textContent =
          "No se pudo reproducir el video. Reabre el recurso o prueba otro navegador.";
    });
  }
  async function renderAsset(content) {
    const session = await api.post(`/enrollments/${id}/media-sessions`, {
      resource_stable_id: current.stable_id,
    });
    content.innerHTML = assetHTML(
      current,
      safeURL(session.content_url || session.manifest_url),
    );
  }
  async function renderResource(content) {
    if (current.content_html)
      content.innerHTML = `<div class="reading">${current.content_html}</div>`;
    else if (["video", "audio"].includes(current.type))
      await playMedia(content);
    else if (current.external_url) content.innerHTML = externalHTML(current);
    else if (current.has_asset) await renderAsset(content);
    else content.innerHTML = empty("Este recurso todavía no tiene contenido.");
  }
  async function open(index) {
    clearInterval(timer);
    clearTimeout(renewTimer);
    if (current) await evidence("close");
    hls?.destroy();
    hls = null;
    current = d.resources[index];
    video = null;
    document
      .querySelectorAll("[data-resource]")
      .forEach((el) =>
        el.classList.toggle("active", Number(el.dataset.resource) === index),
      );
    const required = current.required ? "· OBLIGATORIO" : "";
    $("#reader").innerHTML =
      `<div class="eyebrow">${e(status(current.type))} ${required}</div><h2>${e(current.title)}</h2><div id="resource-content"></div>`;
    const content = $("#resource-content");
    if (current.type === "quiz") return renderQuiz(content);
    await renderResource(content);
    await evidence("open");
    timer = setInterval(() => {
      if (document.visibilityState === "visible" && !video?.paused)
        void evidence("heartbeat").catch((err) => notice(err.message));
    }, 15000);
  }
  document.querySelectorAll("[data-resource]").forEach(
    (el) =>
      (el.onclick = () =>
        open(Number(el.dataset.resource)).catch((err) => {
          if (err.name !== "AbortError" && $("#reader"))
            $("#reader").innerHTML = errorHTML(err);
        })),
  );
  action("withdraw", async () => {
    modal(
      `<h2>¿Retirarte del curso?</h2><p>Tu inscripción quedará retirada.</p>${button("Confirmar retiro", "confirm-withdraw")}`,
    );
    action("confirm-withdraw", async () => {
      await api.post(`/enrollments/${id}/withdraw`);
      $("#modal").close();
      location.hash = "/learning";
    });
  });
  if (d.resources.length) await open(0);
}
function optionHTML(q, o, chosen) {
  const type = q.kind === "single" ? "radio" : "checkbox";
  const checked = chosen.includes(o.stable_id) ? "checked" : "";
  return `<label class="option"><input type="${type}" name="${e(q.stable_id)}" value="${e(o.stable_id)}" ${checked}>${e(o.text_md)}</label>`;
}
function questionHTML(q, i, answers) {
  const chosen =
    answers.find((x) => x.question_stable_id === q.stable_id)
      ?.selected_option_stable_ids || [];
  const options = q.options.map((o) => optionHTML(q, o, chosen)).join("");
  return `<fieldset><legend>${i + 1}. ${e(q.statement_md)}</legend>${options}</fieldset>`;
}
async function attempt(id) {
  if (!requireUser()) return;
  const a = await api.get(`/attempts/${id}`);
  if (a.status !== "in_progress") {
    const result = await api.get(`/attempts/${id}/result`);
    show(
      title("Evaluación", "Tu resultado") +
        resultView(result, a.questions) +
        link("/learning", "Volver a mis cursos"),
    );
    return;
  }
  show(
    title("Evaluación", `Intento ${a.attempt_number}`) +
      (a.expires_at
        ? `<p class="meta">Disponible hasta ${e(new Date(a.expires_at).toLocaleString("es-CO"))}</p>`
        : "") +
      `<form id="quiz">${a.questions.map((q, i) => questionHTML(q, i, a.answers)).join("")}<div class="actions"><button class="primary" type="submit">Guardar respuestas</button>${button("Entregar evaluación ↗", "submit-quiz", "outline")}</div></form>`,
  );
  const save = async () => {
    const f = new FormData($("#quiz"));
    return api.patch(`/attempts/${id}/answers`, {
      answers: a.questions.map((q) => ({
        question_stable_id: q.stable_id,
        selected_option_stable_ids: f.getAll(q.stable_id),
      })),
    });
  };
  bindForm("#quiz", async () => {
    await save();
    notice("Respuestas guardadas.");
  });
  action("submit-quiz", async () => {
    await save();
    modal(
      `<h2>¿Entregar tus respuestas?</h2><p>Una vez entregadas, este intento quedará cerrado.</p>${button("Entregar", "confirm-submit")}`,
    );
    action("confirm-submit", async () => {
      const result = await api.post(`/attempts/${id}/submit`);
      $("#modal").close();
      show(
        title("Evaluación", "Tu resultado") +
          resultView(result, a.questions) +
          link("/learning", "Volver a mis cursos"),
      );
    });
  });
}
function resultQuestionHTML(q, i, questions) {
  const asked = questions.find((p) => p.stable_id === q.question_stable_id);
  const heading = asked?.statement_md || "Pregunta " + (i + 1);
  const verdict = q.correct ? "Respuesta correcta" : "Revisa tu respuesta";
  const correct = q.correct_option_stable_ids?.length
    ? "<p>Respuesta correcta: " +
      q.correct_option_stable_ids
        .map((id) =>
          e(
            asked?.options.find((o) => o.stable_id === id)?.text_md ||
              "Opción correcta",
          ),
        )
        .join(", ") +
      "</p>"
    : "";
  const explanation = q.explanation_md
    ? "<p>" + e(q.explanation_md) + "</p>"
    : "";
  return `<div class="row"><div><h3>${e(heading)}</h3><p>${verdict} · ${e(q.awarded_points)} de ${e(q.max_points)} puntos</p>${correct}${explanation}</div></div>`;
}
function resultView(r, questions = []) {
  const heading = r.passed ? "Objetivo alcanzado." : "Evaluación entregada.";
  const score =
    r.score === undefined ? "" : "<p>Calificación: " + e(r.score) + "</p>";
  const pass =
    r.pass_score === undefined
      ? ""
      : "<p>Umbral de aprobación: " + e(r.pass_score) + "</p>";
  const rows = r.questions
    ? r.questions.map((q, i) => resultQuestionHTML(q, i, questions)).join("")
    : "";
  return `<div class="panel"><h2>${heading}</h2><p>Estado: ${e(status(r.status))}</p>${score}${pass}${rows}</div>`;
}
function badgeCardHTML(b) {
  const img = b.image_url
    ? '<img class="badge-img" src="' +
      e(safeURL(b.image_url)) +
      '" alt="Insignia de ' +
      e(b.course_title) +
      '">'
    : "";
  const share = link(
    "/verify/" + encodeURIComponent(b.public_code),
    "Verificar y compartir ↗",
    "text",
  );
  return `<article class="card">${img}<span class="chip">${e(status(b.status))}</span><h3>${e(b.course_title)}</h3><p>Emitida el ${date(b.issued_at)}</p>${share}</article>`;
}
async function badges() {
  if (!requireUser()) return;
  const d = await api.get("/me/badges");
  show(
    title(
      "Tus logros",
      "Lo aprendido deja huella.",
      "Comparte tus insignias y permite que otros verifiquen tus logros.",
    ) +
      (d.items.length
        ? `<div class="grid">${d.items.map(badgeCardHTML).join("")}</div>`
        : empty("Cuando apruebes un curso, tu insignia aparecerá aquí.")),
  );
}
async function verify(code) {
  if (!code) {
    show(
      `<div class="form-panel">${title("Verificación pública", "Un logro verificable.", "Consulta la autenticidad y el estado de una insignia.")}<form id="verify">${field("code", "Código de la insignia")}<button class="primary" type="submit">Verificar ↗</button></form></div>`,
    );
    bindForm(
      "#verify",
      async (data) =>
        (location.hash = `/verify/${encodeURIComponent(data.code)}`),
    );
    return;
  }
  const b = await api.get(`/verify/${encodeURIComponent(code)}`);
  const img = b.image_url
    ? '<img class="badge-img" src="' +
      e(safeURL(b.image_url)) +
      '" alt="Insignia">'
    : "";
  const revoked = b.revoked_at
    ? '<p class="danger">Revocada el ' + date(b.revoked_at) + "</p>"
    : "";
  show(
    `<div class="form-panel">${title("Verificación pública", b.course_title)}<div class="panel">${img}<h2>${e(b.student_name)}</h2><span class="chip">${e(status(b.status))}</span><p>Versión ${e(b.course_version)} · Emitida el ${date(b.issued_at)}</p>${revoked}</div>${button("Copiar enlace para compartir", "copy")} ${link("/verify", "Verificar otra", "text")}</div>`,
  );
  action("copy", async () => {
    await navigator.clipboard.writeText(location.href);
    notice("Enlace copiado.");
  });
}
function courseFields(c = {}) {
  return `<div class="form-grid">${field("title", "Título", c.title)}${field("category", "Categoría", c.category)}${select("language", "Idioma", ["es", "en"], c.language || "es")}</div><label>Descripción<textarea name="summary" required>${e(c.summary || "")}</textarea></label>`;
}
async function studio() {
  if (!requireUser(["teacher", "admin"])) return;
  const d = await api.all("/courses?limit=100");
  show(
    title(
      "Estudio docente",
      "Comparte lo que sabes.",
      "Crea cursos y transforma tu experiencia en nuevas oportunidades para otros.",
    ) +
      `<div class="tabs">${link("/studio", "Cursos", "active")}${link("/media", "Biblioteca de medios", "")}</div><details class="panel"><summary>Crear un curso</summary><form id="new-course">${courseFields()}<button class="primary" type="submit">Crear curso ↗</button></form></details>` +
      (d.items.length
        ? d.items
            .map(
              (c) =>
                `<article class="row"><div><span class="chip">${e(status(c.status))}</span><h3>${e(c.title)}</h3><p>${e(c.summary)}</p></div>${link("/studio/" + c.id, "Editar curso ↗")}</article>`,
            )
            .join("")
        : empty("Tu primer curso empieza aquí.")),
  );
  bindForm("#new-course", async (data) => {
    const c = await api.post("/courses", data);
    location.hash = `/studio/${c.course.id}`;
  });
}
async function studioCourse(id) {
  if (!requireUser(["teacher", "admin"])) return;
  const [c, vs] = await Promise.all([
    api.get(`/courses/${id}`),
    api.get(`/courses/${id}/versions`),
  ]);
  show(
    link("/studio", "← Estudio", "text") +
      title("Curso", c.title) +
      `<details class="panel"><summary>Información del curso</summary><form id="edit-course">${courseFields(c)}<button class="primary" type="submit">Guardar información</button></form></details><div class="section-head"><h2>Versiones</h2>${vs.items.some((v) => v.status === "draft") ? '<span class="muted">Continúa editando tu borrador</span>' : button("Crear nueva versión", "new-version")}</div>` +
      vs.items
        .map(
          (v) =>
            `<article class="row"><div><h3>Versión ${e(v.version_number)}</h3><span class="chip">${e(status(v.status))}</span><p>${date(v.created_at)}</p></div>${link("/studio/" + id + "/" + v.version_number, v.status === "draft" ? "Editar contenido ↗" : "Ver contenido ↗")}</article>`,
        )
        .join(""),
  );
  bindForm("#edit-course", async (data) => {
    await api.patch(`/courses/${id}`, data);
    notice("Información guardada.");
  });
  action("new-version", async () => {
    const v = await api.post(`/courses/${id}/versions`);
    location.hash = `/studio/${id}/${v.version_number}`;
  });
}
function resourceFields(r = {}) {
  return `<div class="form-grid">${field("title", "Título", r.title)}${select("type", "Tipo", ["rich_text", "image", "video", "audio", "pdf", "slides", "download", "iframe", "quiz", "link"], r.type || "rich_text")}${field("asset_id", "ID del archivo", r.asset_id || "", "text", false)}${field("external_url", "Enlace externo", r.external_url || "", "url", false)}</div>${check("visible", "Visible", r.visible ?? true)}${check("required", "Obligatorio", r.required ?? true)}${check("downloadable", "Permitir descarga", r.downloadable ?? false)}<label>Contenido Markdown<textarea name="content_md">${e(r.content_md || "")}</textarea></label>`;
}
function resourceData(data, form) {
  return {
    ...data,
    visible: form.elements.visible.checked,
    required: form.elements.required.checked,
    downloadable: form.elements.downloadable.checked,
    asset_id: data.asset_id || null,
    external_url: data.external_url || null,
  };
}
function versionActionsHTML(v, draft) {
  let lifecycle = "";
  if (draft) lifecycle = button("Publicar versión ↗", "publish");
  else if (v.status === "published")
    lifecycle = button("Retirar publicación", "unpublish", "outline");
  return (
    button("Vista previa", "preview", "outline") +
    button("Validar contenido", "validate", "outline") +
    lifecycle
  );
}
function resourceEditorHTML(r, draft) {
  let body = "<p>" + e(r.content_md || "") + "</p>";
  if (draft) {
    const quiz =
      r.type === "quiz"
        ? link("/quiz-editor/" + r.id, "Configurar evaluación ↗", "text")
        : "";
    body = `<form id="resource-${r.id}">${resourceFields(r)}<button class="primary" type="submit">Guardar recurso</button></form>${quiz}`;
  }
  return `<details class="resource-editor"><summary>${e(r.title)} · ${e(r.type)} · ${e(r.processing_status)}</summary>${body}</details>`;
}
function unitEditorHTML(u, draft) {
  const resources = (u.resources || [])
    .map((r) => resourceEditorHTML(r, draft))
    .join("");
  const add = draft
    ? `<details class="resource-editor"><summary>Agregar recurso</summary><form id="add-resource-${u.id}">${resourceFields()}<button class="primary" type="submit">Agregar recurso</button></form></details>`
    : "";
  return `<section><h3>${e(u.title)}</h3>${resources}${add}</section>`;
}
function moduleEditorHTML(m, draft) {
  const units = (m.units || []).map((u) => unitEditorHTML(u, draft)).join("");
  const add = draft
    ? `<form id="add-unit-${m.id}" class="resource-editor">${field("title", "Nueva unidad")}<button class="primary" type="submit">Agregar unidad</button></form>`
    : "";
  return `<section class="panel"><h2>${e(m.title)}</h2>${units}${add}</section>`;
}
function versionHTML(id, n, d, draft) {
  const v = d.version;
  const immutable = draft
    ? ""
    : '<p class="muted">Esta versión es inmutable. Crea una nueva versión para editar el contenido.</p>';
  const modules = (v.modules || [])
    .map((m) => moduleEditorHTML(m, draft))
    .join("");
  const addModule = draft
    ? `<form id="add-module" class="panel">${field("title", "Nuevo módulo")}<button class="primary" type="submit">Agregar módulo</button></form>`
    : "";
  return (
    link("/studio/" + id, "← Versiones", "text") +
    title("Versión " + n + " · " + status(v.status), d.course.title) +
    `<div class="actions">${versionActionsHTML(v, draft)}</div><div id="validation"></div>${immutable}${modules}${addModule}`
  );
}
function bindResourceForm(id, n, r) {
  bindForm(`#resource-${r.id}`, async (data, form) => {
    const body = resourceData(data, form);
    const content = body.content_md;
    delete body.content_md;
    await api.patch(`/resources/${r.id}`, body);
    if (content !== r.content_md && r.type === "rich_text")
      await api.patch(
        `/resources/${r.id}/content`,
        { content_md: content },
        r.etag,
      );
    notice("Recurso guardado.");
    await version(id, n);
  });
}
function bindUnitForms(id, n, u) {
  bindForm(`#add-resource-${u.id}`, async (data, form) => {
    await api.post(`/units/${u.id}/resources`, resourceData(data, form));
    await version(id, n);
  });
  for (const r of u.resources || []) bindResourceForm(id, n, r);
}
function bindVersionForms(id, n, v) {
  bindForm("#add-module", async (data) => {
    await api.post(`/versions/${v.id}/modules`, data);
    await version(id, n);
  });
  for (const m of v.modules || []) {
    bindForm(`#add-unit-${m.id}`, async (data) => {
      await api.post(`/modules/${m.id}/units`, data);
      await version(id, n);
    });
    for (const u of m.units || []) bindUnitForms(id, n, u);
  }
}
function confirmAction(html, name, run) {
  modal(html);
  action(name, async () => {
    await run();
    $("#modal").close();
  });
}
function previewResourceHTML(r) {
  return `<h4>${e(r.title)}</h4><div class="reading">${r.content_html || e(r.content_md || r.type)}</div>`;
}
function previewUnitHTML(u) {
  const resources = (u.resources || [])
    .filter((r) => r.visible)
    .map(previewResourceHTML)
    .join("");
  return `<h4>${e(u.title)}</h4>${resources}`;
}
function previewModuleHTML(m) {
  return `<h3>${e(m.title)}</h3>${(m.units || []).map(previewUnitHTML).join("")}`;
}
function bindVersionActions(id, n, base) {
  const problems = (message, res) => {
    $("#validation").innerHTML = errorHTML({ message, problem: res });
  };
  action("validate", async () => {
    const res = await api.post(`${base}/validate`);
    if (res.publishable)
      $("#validation").innerHTML =
        '<p class="success">Todo listo para publicar.</p>';
    else problems("Revisa estos puntos antes de publicar.", res);
  });
  action("publish", async () => {
    const res = await api.post(`${base}/validate`);
    if (!res.publishable) {
      problems("El curso necesita cambios.", res);
      return;
    }
    confirmAction(
      `<h2>Publicar versión ${e(n)}</h2><p>Los estudiantes podrán inscribirse. El contenido publicado será inmutable.</p>${button("Confirmar publicación", "confirm-publish")}`,
      "confirm-publish",
      async () => {
        await api.post(`${base}/publish`);
        await version(id, n);
      },
    );
  });
  action("unpublish", async () => {
    confirmAction(
      `<h2>Retirar publicación</h2><p>Esta versión dejará de estar disponible para nuevas inscripciones.</p>${button("Confirmar", "confirm-unpublish")}`,
      "confirm-unpublish",
      async () => {
        await api.post(`${base}/unpublish`);
        await version(id, n);
      },
    );
  });
  action("preview", async () => {
    const p = await api.get(`${base}/preview`);
    const modules = (p.version.modules || []).map(previewModuleHTML).join("");
    modal(`<h2>Vista previa</h2>${modules}`);
  });
}
async function version(id, n) {
  if (!requireUser(["teacher", "admin"])) return;
  const base = `/courses/${id}/versions/${n}`;
  const d = await api.get(base);
  const draft = d.version.status === "draft";
  show(versionHTML(id, n, d, draft));
  if (draft) bindVersionForms(id, n, d.version);
  bindVersionActions(id, n, base);
}
function optionEditorHTML(o, j) {
  return `<div data-option="${j}">${field("text_md", "Opción " + (j + 1), o.text_md)}${check("is_correct", "Respuesta correcta", o.is_correct)}</div>`;
}
function questionEditorHTML(x, i) {
  return `<fieldset data-question="${i}"><legend>Pregunta ${i + 1}</legend>${field("statement_md", "Enunciado", x.statement_md)}<div class="form-grid">${select("kind", "Tipo", ["single", "multiple"], x.kind)}${field("points", "Puntos", x.points, "number")}</div>${field("explanation_md", "Explicación", x.explanation_md || "", "text", false)}${x.options.map(optionEditorHTML).join("")}<div class="actions"><button type="button" class="outline" data-add-option="${i}">Agregar opción</button><button type="button" class="text danger" data-remove-question="${i}">Quitar pregunta</button></div></fieldset>`;
}
async function quizEditor(id) {
  if (!requireUser(["teacher", "admin"])) return;
  let q = {
    max_attempts: 3,
    time_limit_seconds: null,
    pass_score: 70,
    feedback_policy: "score_only",
    shuffle: false,
    partial_credit: false,
    questions: [],
  };
  try {
    q = await api.get(`/resources/${id}/quiz`);
  } catch (err) {
    if (err.status !== 404) throw err;
  }
  show(
    title("Estudio · evaluación", "Diseña una evaluación.") +
      `<form id="quiz-config"><div class="form-grid">${field("max_attempts", "Máximo de intentos", q.max_attempts, "number")}${field("pass_score", "Puntaje para aprobar", q.pass_score, "number")}${field("time_limit_seconds", "Límite en segundos (opcional)", q.time_limit_seconds || "", "number", false)}${select("feedback_policy", "Retroalimentación", ["none", "score_only", "correctness", "full"], q.feedback_policy)}</div>${check("shuffle", "Mezclar preguntas", q.shuffle)}${check("partial_credit", "Crédito parcial", q.partial_credit)}<div id="questions"></div>${button("Agregar pregunta", "add-question", "outline")}<p><button class="primary" type="submit">Guardar evaluación</button></p></form>`,
  );
  let questions = q.questions.map((x) => ({
    ...x,
    options: x.options.map((o) => ({ ...o })),
  }));
  function draw() {
    $("#questions").innerHTML = questions.map(questionEditorHTML).join("");
    document.querySelectorAll("[data-add-option]").forEach(
      (b) =>
        (b.onclick = () => {
          collect();
          questions[Number(b.dataset.addOption)].options.push({
            text_md: "",
            is_correct: false,
          });
          draw();
        }),
    );
    document.querySelectorAll("[data-remove-question]").forEach(
      (b) =>
        (b.onclick = () => {
          collect();
          questions.splice(Number(b.dataset.removeQuestion), 1);
          draw();
        }),
    );
  }
  function collect() {
    questions = [...document.querySelectorAll("[data-question]")].map((el) => ({
      statement_md: el.querySelector("[name=statement_md]").value,
      kind: el.querySelector("[name=kind]").value,
      points: Number(el.querySelector("[name=points]").value),
      explanation_md: el.querySelector("[name=explanation_md]").value || null,
      options: [...el.querySelectorAll("[data-option]")].map((o) => ({
        text_md: o.querySelector("[name=text_md]").value,
        is_correct: o.querySelector("[name=is_correct]").checked,
      })),
    }));
  }
  action("add-question", () => {
    collect();
    questions.push({
      statement_md: "",
      kind: "single",
      points: 1,
      options: [
        { text_md: "", is_correct: true },
        { text_md: "", is_correct: false },
      ],
    });
    draw();
  });
  draw();
  bindForm("#quiz-config", async (data, form) => {
    collect();
    await api.request(`/resources/${id}/quiz`, {
      method: "PUT",
      body: {
        max_attempts: Number(data.max_attempts),
        pass_score: Number(data.pass_score),
        time_limit_seconds: data.time_limit_seconds
          ? Number(data.time_limit_seconds)
          : null,
        feedback_policy: data.feedback_policy,
        shuffle: form.elements.shuffle.checked,
        partial_credit: form.elements.partial_credit.checked,
        questions,
      },
    });
    notice("Evaluación guardada.");
  });
}
function recordHTML(r, section) {
  const requeue =
    section === "jobs" && ["dead", "failed"].includes(r.status)
      ? '<button class="outline" data-job="' + e(r.id) + '">Reencolar</button>'
      : "";
  return `<article class="row"><div><h3>${e(r.type || r.action || r.job_key)}</h3><div class="meta">${e(status(r.status))} · ${date(r.occurred_at || r.created_at)}</div><details><summary>Ver detalles</summary><pre>${e(JSON.stringify(r, null, 2))}</pre></details></div>${requeue}</article>`;
}
const sessionRowHTML = (s) =>
  `<div class="row"><div><p>${e(s.user_agent || s.agent || "Sesión")}</p><span class="meta">${e(s.ip)} · ${date(s.created_at)}</span></div></div>`;
async function admin(section = "users") {
  if (!requireUser(["admin"])) return;
  show(
    title("Administración", "Una plataforma en marcha.") +
      `<div class="tabs">${[
        ["users", "Usuarios"],
        ["audit", "Auditoría"],
        ["jobs", "Trabajos"],
        ["badges", "Insignias"],
      ]
        .map(([s, t]) => link("/admin/" + s, t, section === s ? "active" : ""))
        .join("")}</div><div id="admin-content"></div>`,
  );
  const target = $("#admin-content");
  if (section === "badges") {
    target.innerHTML = `<form id="revoke-badge" class="panel">${field("id", "ID de la insignia")}${field("reason", "Motivo de revocación")}<button class="primary" type="submit">Revocar insignia</button></form>`;
    bindForm("#revoke-badge", async (data) => {
      await api.post(`/badges/${encodeURIComponent(data.id)}/revoke`, {
        reason: data.reason,
      });
      notice("Insignia revocada.");
    });
    return;
  }
  if (section === "users") {
    target.innerHTML = `<form id="user-search" class="filters">${field("q", "Buscar usuario", "", "search", false)}${select("role", "Rol", ["", "student", "teacher", "admin"], "")}${select("status", "Estado", ["", "active", "pending_verification", "suspended"], "")}<button class="primary" type="submit">Buscar</button></form><details class="panel"><summary>Invitar docente</summary><form id="invite">${field("full_name", "Nombre completo")}${field("email", "Correo", "", "email")}<button class="primary" type="submit">Enviar invitación</button></form></details><div id="users"></div>`;
    const load = async (query = "") => {
      const d = await api.get(`/admin/users?limit=100&${query}`);
      $("#users").innerHTML =
        d.items
          .map(
            (u) =>
              `<div class="row"><div><h3>${e(u.full_name)}</h3><p>${e(u.email)}</p><span class="meta">${e(u.role)} · ${e(status(u.status))}</span></div>${link("/admin-user/" + u.id, "Administrar ↗", "text")}</div>`,
          )
          .join("") || empty("No se encontraron usuarios.");
    };
    bindForm("#user-search", async (data) => load(new URLSearchParams(data)));
    bindForm("#invite", async (data) => {
      await api.post("/admin/users", { ...data, role: "teacher" });
      notice("Invitación enviada.");
      await load();
    });
    await load();
    return;
  }
  target.innerHTML = `<form id="admin-filter" class="filters">${field(section === "jobs" ? "status" : "action", section === "jobs" ? "Estado" : "Acción", "", "text", false)}<button class="primary" type="submit">Filtrar</button></form><div id="records"></div>`;
  const load = async (query = "") => {
    const d = await api.get(`/admin/${section}?limit=100&${query}`);
    $("#records").innerHTML =
      d.items.map((r) => recordHTML(r, section)).join("") ||
      empty("No hay registros con este filtro.");
    document.querySelectorAll("[data-job]").forEach(
      (b) =>
        (b.onclick = async () => {
          b.disabled = true;
          try {
            await api.post(`/admin/jobs/${b.dataset.job}/requeue`);
            notice("Trabajo reencolado.");
            await load(query);
          } catch (err) {
            if (err.name !== "AbortError") notice(err.message);
          } finally {
            b.disabled = false;
          }
        }),
    );
  };
  bindForm("#admin-filter", async (data) => load(new URLSearchParams(data)));
  await load();
}
async function adminUser(id) {
  if (!requireUser(["admin"])) return;
  const [u, ss] = await Promise.all([
    api.get(`/admin/users/${id}`),
    api.get(`/admin/users/${id}/sessions`),
  ]);
  show(
    link("/admin", "← Usuarios", "text") +
      title("Usuario", u.full_name, u.email) +
      `<form id="user-update" class="panel"><div class="form-grid">${select("role", "Rol", ["student", "teacher", "admin"], u.role)}${select("status", "Estado", [...(u.status === "pending_verification" ? ["pending_verification"] : []), "active", "suspended", "deleted"], u.status)}</div><button class="primary" type="submit">Guardar cambios</button></form><h2>Sesiones activas</h2>${
        ss.items
          .filter((s) => s.active)
          .map(sessionRowHTML)
          .join("") || empty("No tiene sesiones activas.")
      }${button("Revocar todas las sesiones", "revoke-sessions", "outline danger")}`,
  );
  bindForm("#user-update", async (data) => {
    const changes = Object.fromEntries(
      Object.entries(data).filter(([key, value]) => u[key] !== value),
    );
    if (!Object.keys(changes).length) {
      notice("No hay cambios para guardar.");
      return;
    }
    await api.patch(`/admin/users/${id}`, changes);
    notice("Usuario actualizado.");
    await adminUser(id);
  });
  action("revoke-sessions", async () => {
    modal(
      `<h2>Revocar sesiones</h2><p>El usuario deberá iniciar sesión nuevamente.</p>${button("Confirmar revocación", "confirm-revoke")}`,
    );
    action("confirm-revoke", async () => {
      await api.request(`/admin/users/${id}/sessions`, { method: "DELETE" });
      $("#modal").close();
      await adminUser(id);
    });
  });
}
// SHA-256 incremental, por bloques de 8 MiB y en orden: el archivo nunca
// está entero en memoria.
async function hashFile(
  file,
  output,
  offset = 0,
  hash = window.sha256.create(),
) {
  if (offset >= file.size) return hash.hex();
  const block = 8 * 1024 * 1024;
  hash.update(await file.slice(offset, offset + block).arrayBuffer());
  const pct = Math.round(
    (Math.min(offset + block, file.size) / file.size) * 100,
  );
  output.textContent = "Verificando archivo… " + pct + "%";
  return hashFile(file, output, offset + block, hash);
}
async function uploadPart(file, ticket, number) {
  const signed = await api.post(`/assets/${ticket.asset_id}/upload/parts`, {
    parts: [number],
  });
  const response = await fetch(safeURL(signed.parts[0].url), {
    method: "PUT",
    body: file.slice(
      (number - 1) * ticket.part_size,
      number * ticket.part_size,
    ),
  });
  if (!response.ok)
    throw new Error(
      "No se pudo cargar esta parte. Puedes reanudar con el mismo archivo y el ID de carga.",
    );
  const etag = response.headers.get("ETag");
  if (!etag)
    throw new Error(
      "El almacén no expone ETag. Revisa su configuración CORS y reanuda la carga.",
    );
  return etag;
}
// Sube las partes que faltan una tras otra: si la carga se corta, las que
// ya entraron quedan registradas y se reanuda desde la siguiente.
async function uploadParts(file, ticket, missing, received, output) {
  if (!missing.length) return received;
  const [number, ...rest] = missing;
  output.textContent = `Cargando parte ${number} de ${ticket.total_parts} · ID ${ticket.asset_id}`;
  received.push({
    part_number: number,
    etag: await uploadPart(file, ticket, number),
  });
  return uploadParts(file, ticket, rest, received, output);
}
async function media() {
  if (!requireUser(["teacher", "admin"])) return;
  const uploadKey = "mooc.upload." + user.id;
  show(
    link("/studio", "← Estudio", "text") +
      title(
        "Biblioteca de medios",
        "De tu archivo al aula.",
        "Carga videos y documentos. El servidor verifica y procesa cada archivo antes de publicarlo.",
      ) +
      `<form id="upload" class="panel"><label>Archivo<input type="file" name="file" required ></label>${select("kind", "Tipo", ["video", "audio", "image", "pdf", "file"], "video")}${field("asset_id", "ID de carga para reanudar (opcional)", sessionStorage.getItem(uploadKey) || "", "text", false)}<button class="primary" type="submit">Cargar archivo ↗</button><output id="upload-status"></output></form><form id="asset-search">${field("id", "Consultar archivo por ID")}<button class="primary" type="submit">Consultar estado</button></form><div id="asset"></div>`,
  );
  bindForm("#upload", async (data, form) => {
    const file = form.elements.file.files[0];
    const output = $("#upload-status");
    output.textContent = "Verificando archivo…";
    const sha256 = await hashFile(file, output);
    let ticket;
    if (data.asset_id) {
      ticket = await api.get(
        `/assets/${encodeURIComponent(data.asset_id)}/upload`,
      );
      const asset = await api.get(
        `/assets/${encodeURIComponent(data.asset_id)}`,
      );
      if (asset.size_bytes !== file.size || asset.declared_sha256 !== sha256)
        throw new Error("Selecciona el archivo original para reanudar.");
    } else {
      ticket = await api.post("/assets/init", {
        filename: file.name,
        content_type: file.type || "application/octet-stream",
        size_bytes: file.size,
        sha256,
        kind: data.kind,
      });
      form.elements.asset_id.value = ticket.asset_id;
      sessionStorage.setItem(uploadKey, ticket.asset_id);
    }
    const missing =
      ticket.missing_parts || ticket.parts.map((p) => p.part_number);
    const received = await uploadParts(
      file,
      ticket,
      missing,
      ticket.received_parts || [],
      output,
    );
    await api.post(`/assets/${ticket.asset_id}/complete`, {
      parts: received.map((p) => ({
        part_number: p.part_number,
        etag: p.etag,
      })),
    });
    output.textContent = `Carga completa. ID ${ticket.asset_id}. El servidor está procesando el archivo.`;
    form.elements.file.value = "";
    form.elements.asset_id.value = "";
    sessionStorage.removeItem(uploadKey);
  });
  bindForm("#asset-search", async (data) => {
    const a = await api.get(`/assets/${encodeURIComponent(data.id)}`);
    const failure = a.last_error
      ? '<p class="error">' + e(a.last_error) + "</p>"
      : "";
    $("#asset").innerHTML =
      `<div class="panel"><h2>${e(a.original_filename)}</h2><span class="chip">${e(status(a.status))}</span><p>ID: ${e(a.id)}</p>${failure}<p>Usa este ID al agregar un recurso al curso.</p><div class="actions">${button("Actualizar estado", "refresh-asset", "outline")}${button("Abrir contenido", "asset-content", "text")}${button("Cancelar carga", "abort-upload", "text danger")}</div></div>`;
    action("refresh-asset", () => $("#asset-search").requestSubmit());
    action("abort-upload", async () => {
      await api.post(`/assets/${encodeURIComponent(data.id)}/abort`);
      notice("Carga cancelada.");
    });
    action("asset-content", async () => {
      const response = await fetch(
        `/api/v1/assets/${encodeURIComponent(data.id)}/content`,
        { headers: { Authorization: `Bearer ${api.token}` } },
      );
      if (!response.ok)
        throw new Error("El contenido todavía no está disponible.");
      const url = safeURL(response.url);
      await response.body?.cancel();
      modal(
        `<h2>Contenido del archivo</h2><a href="${url}" download="${e(a.original_filename)}" class="button">Descargar archivo</a>`,
      );
    });
  });
}
async function route() {
  const run = ++pageRun;
  api.beginPage();
  void cleanup();
  cleanup = async () => {};
  header();
  show('<div class="loading">Abriendo tu espacio</div>');
  const parts = (location.hash.slice(1) || "/").split("/").filter(Boolean);
  try {
    parts.forEach((p, i) => (parts[i] = decodeURIComponent(p)));
    switch (parts[0]) {
      case undefined:
        await home();
        break;
      case "catalog":
        await catalog();
        break;
      case "course":
        await course(parts[1]);
        break;
      case "login":
      case "register":
      case "verify-email":
        await auth(parts[0]);
        break;
      case "learning":
        await learning();
        break;
      case "learn":
        await learn(parts[1]);
        break;
      case "attempt":
        await attempt(parts[1]);
        break;
      case "badges":
        await badges();
        break;
      case "verify":
        await verify(parts[1]);
        break;
      case "studio":
        if (parts[2]) await version(parts[1], parts[2]);
        else if (parts[1]) await studioCourse(parts[1]);
        else await studio();
        break;
      case "quiz-editor":
        await quizEditor(parts[1]);
        break;
      case "media":
        await media();
        break;
      case "admin":
        await admin(parts[1]);
        break;
      case "admin-user":
        await adminUser(parts[1]);
        break;
      default:
        show(
          title("404", "Este camino aún no existe.") +
            link("/", "Volver al inicio"),
        );
    }
  } catch (err) {
    if (err.name === "AbortError") return;
    if (run === pageRun) {
      show(
        title("Un momento", "No pudimos abrir esta página.") +
          errorHTML(err) +
          button("Volver a intentar", "retry", "outline"),
      );
      action("retry", route);
      if (!api.token) {
        user = null;
        header();
      }
    }
  }
  if (run === pageRun) {
    header();
    document.title = `${$("h1", main)?.textContent || "Aprender nos transforma"} · MOOC`;
    main.focus({ preventScroll: true });
    window.scrollTo(0, 0);
  }
}
window.addEventListener("hashchange", route);
window.addEventListener("pagehide", () => cleanup());
if (api.token) {
  try {
    user = await api.get("/me");
  } catch {
    api.setToken(null);
  }
}
await route();
