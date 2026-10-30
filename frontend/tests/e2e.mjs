import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
const { chromium } = await import(
  process.env.PLAYWRIGHT_MODULE || "playwright"
);
const base = process.env.BASE_URL || "http://localhost:8090";
// Las capturas finales: /tmp en local, la carpeta de evidencias en la nube.
const evidencias = process.env.EVIDENCIAS || "/tmp";
const browser = await chromium.launch({
  headless: true,
  ...(process.env.CHROME_PATH
    ? { executablePath: process.env.CHROME_PATH }
    : {}),
});
const context = await browser.newContext({
  viewport: { width: 1440, height: 1000 },
});
const page = await context.newPage();
// Capturas del recorrido, numeradas en orden. La matriz de
// `docs/entrega3/verificacion-frontend.md` se lee contra estos archivos: el
// enunciado no exige capturas, pero pide evidencia de verificación funcional, y
// dos imágenes del final no enseñan los veinte flujos que la prueba recorre.
let nCaptura = 0;
async function captura(nombre) {
  nCaptura += 1;
  const n = String(nCaptura).padStart(2, "0");
  await page.screenshot({ path: `${evidencias}/${n}-${nombre}.png` });
}
const errors = [];
page.on("pageerror", (err) => errors.push(err.message));
const name = `Curso navegador ${Date.now()}`;
async function goto(path, heading) {
  await page.goto(`${base}/#${path}`);
  if (heading)
    await page.getByRole("heading", { name: heading, exact: true }).waitFor();
}
async function login(email) {
  await goto("/login", "Qué bueno verte de nuevo.");
  await page.getByLabel("Correo electrónico").fill(email);
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill(process.env.DEMO_PASSWORD || "Contrasena-Demo-2026");
  await page
    .getByRole("button", { name: "Iniciar sesión ↗", exact: true })
    .click();
  await page.getByRole("heading", { name: /Sigue avanzando/ }).waitFor();
}
async function logout() {
  await page.getByRole("button", { name: "Salir", exact: true }).click();
  await page
    .getByRole("heading", { name: "Aprender nos transforma." })
    .waitFor();
}
// Llama a la API con la sesión de la pestaña, como lo haría alguien que se
// salta la interfaz. Devuelve solo el código HTTP.
async function apiStatus(method, path, body) {
  return page.evaluate(
    async ([m, p, b]) => {
      const response = await fetch(`/api/v1${p}`, {
        method: m,
        headers: {
          Authorization: `Bearer ${sessionStorage.getItem("mooc.session")}`,
          "Content-Type": "application/json",
          "Idempotency-Key": crypto.randomUUID(),
        },
        body: b === undefined ? undefined : JSON.stringify(b),
      });
      return response.status;
    },
    [method, path, body],
  );
}
async function settle() {
  await page.waitForTimeout(350);
  const error = await page.locator(".form-error .error").allTextContents();
  assert.deepEqual(error, []);
}
try {
  await goto("/", "Aprender nos transforma.");
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  const signupEmail = `navegador-${Date.now()}@mooc.local`;
  await goto("/register", "Abre nuevas posibilidades.");
  await page.getByLabel("Nombre completo").fill("Persona de prueba");
  await page.getByLabel("Correo electrónico").fill(signupEmail);
  await page
    .getByLabel("Contraseña", { exact: true })
    .fill("Contrasena-Demo-2026");
  await page.getByRole("button", { name: "Crear mi cuenta ↗" }).click();
  await page.getByRole("heading", { name: "Revisa tu correo." }).waitFor();
  const mailpit = process.env.MAILPIT_URL || "http://localhost:8026";
  let token;
  for (let i = 0; i < 30 && !token; i++) {
    const inbox = await (
      await fetch(
        `${mailpit}/api/v1/search?query=${encodeURIComponent(`to:${signupEmail}`)}`,
      )
    ).json();
    const message = inbox.messages?.[0];
    if (message) {
      const mail = await (
        await fetch(`${mailpit}/api/v1/message/${message.ID}`)
      ).json();
      token = mail.Text.match(/Token de verificación:\s*(\S+)/u)?.[1];
    }
    if (!token) await page.waitForTimeout(500);
  }
  assert.ok(token, "el worker debe enviar el código de verificación");
  await goto("/verify-email", "Verifica tu correo");
  await page.getByLabel("Código de verificación").fill(token);
  await page
    .getByRole("button", { name: "Verificar correo", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "Qué bueno verte de nuevo." })
    .waitFor();
  await login("profesor@mooc.local");
  await goto("/studio", "Comparte lo que sabes.");
  await captura("profesor-estudio");
  await page.getByText("Crear un curso", { exact: true }).click();
  const newCourse = page.locator("#new-course");
  await newCourse.getByLabel("Título", { exact: true }).fill(name);
  await newCourse.getByLabel("Categoría").fill("cloud");
  await newCourse
    .getByLabel("Descripción")
    .fill("Curso sintético creado por la prueba del frontend.");
  await newCourse.getByRole("button", { name: "Crear curso ↗" }).click();
  await page.getByRole("heading", { name: name, exact: true }).waitFor();
  const courseId = page.url().split("/").at(-1);
  await page
    .getByRole("link", { name: "Editar contenido ↗", exact: true })
    .click();
  await page.getByRole("button", { name: "Publicar versión ↗" }).waitFor();
  await page.getByLabel("Nuevo módulo").fill("Fundamentos");
  await page
    .getByRole("button", { name: "Agregar módulo", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "Fundamentos", exact: true })
    .waitFor();
  await page.getByLabel("Nueva unidad").fill("Primeros pasos");
  await page
    .getByRole("button", { name: "Agregar unidad", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "Primeros pasos", exact: true })
    .waitFor();
  async function resource(title, type, text) {
    await page
      .locator("summary")
      .filter({ hasText: /^Agregar recurso$/ })
      .click();
    const form = page.locator('form[id^="add-resource-"]');
    await form.getByLabel("Título", { exact: true }).fill(title);
    await form.getByLabel("Tipo", { exact: true }).selectOption(type);
    if (text) await form.getByLabel("Contenido Markdown").fill(text);
    await form
      .getByRole("button", { name: "Agregar recurso", exact: true })
      .click();
    await page
      .locator(".resource-editor summary")
      .filter({ hasText: title })
      .waitFor();
  }
  await resource(
    "Lectura de bienvenida",
    "rich_text",
    "# Bienvenido\n\nAprendemos usando infraestructura existente.",
  );
  await goto("/media", "De tu archivo al aula.");
  await captura("profesor-biblioteca-de-medios");
  await page.locator("[name=file]").setInputFiles({
    name: "prueba.png",
    mimeType: "image/png",
    buffer: Buffer.from(
      "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aI1sAAAAASUVORK5CYII=",
      "base64",
    ),
  });
  await page.locator("#upload [name=kind]").selectOption("image");
  await page.getByRole("button", { name: "Cargar archivo ↗" }).click();
  await page.getByText(/Carga completa. ID/).waitFor();
  const assetId = (await page.locator("#upload-status").textContent()).match(
    /ID ([0-9a-f-]+)/,
  )[1];
  await page.locator("#asset-search [name=id]").fill(assetId);
  let clean = false;
  for (let i = 0; i < 40; i++) {
    await page
      .locator("#asset-search")
      .getByRole("button", { name: "Consultar estado" })
      .click();
    await page.waitForTimeout(500);
    if (
      await page
        .locator("#asset .chip")
        .filter({ hasText: /Verificado|clean/ })
        .count()
    ) {
      clean = true;
      break;
    }
    await page.waitForTimeout(1000);
  }
  assert.ok(clean, "el archivo debe completar la verificación y escaneo");
  await page
    .getByRole("button", { name: "Abrir contenido", exact: true })
    .click();
  await page
    .getByRole("heading", { name: "Contenido del archivo", exact: true })
    .waitFor();
  // El enlace tiene que venir firmado, nunca ser una URL pública del bucket.
  // El nombre del parámetro lo pone el almacén: `X-Amz-Signature` en MinIO y
  // `X-Goog-Signature` en Cloud Storage. Exigir solo el de MinIO hacía que esta
  // prueba fallara en la nube pese a estar el enlace bien firmado.
  assert.match(
    await page
      .getByRole("link", { name: "Descargar archivo", exact: true })
      .getAttribute("href"),
    /X-(Amz|Goog)-Signature=[^&]/,
  );
  await page.getByRole("button", { name: "Cerrar", exact: true }).click();
  await goto(`/studio/${courseId}/1`, name);
  await page
    .locator("summary")
    .filter({ hasText: /^Agregar recurso$/ })
    .click();
  const imageForm = page.locator('form[id^="add-resource-"]');
  await imageForm
    .getByLabel("Título", { exact: true })
    .fill("Imagen de prueba");
  await imageForm.getByLabel("Tipo", { exact: true }).selectOption("image");
  await imageForm.getByLabel("ID del archivo").fill(assetId);
  await imageForm.getByLabel("Obligatorio", { exact: true }).uncheck();
  await imageForm
    .getByRole("button", { name: "Agregar recurso", exact: true })
    .click();
  await page
    .locator("summary")
    .filter({ hasText: "Imagen de prueba" })
    .waitFor();
  await goto("/media", "De tu archivo al aula.");
  await page
    .locator("[name=file]")
    .setInputFiles(
      fileURLToPath(new URL("./fixtures/video.mp4", import.meta.url)),
    );
  await page.locator("#upload [name=kind]").selectOption("video");
  await page.getByRole("button", { name: "Cargar archivo ↗" }).click();
  await page.getByText(/Carga completa. ID/).waitFor();
  const videoId = (await page.locator("#upload-status").textContent()).match(
    /ID ([0-9a-f-]+)/,
  )[1];
  await page.locator("#asset-search [name=id]").fill(videoId);
  let ready = false;
  for (let i = 0; i < 60; i++) {
    await page
      .locator("#asset-search")
      .getByRole("button", { name: "Consultar estado" })
      .click();
    await page.waitForTimeout(500);
    if (
      await page.locator("#asset .chip").filter({ hasText: "Listo" }).count()
    ) {
      ready = true;
      break;
    }
    await page.waitForTimeout(1000);
  }
  assert.ok(ready, "el video debe completar escaneo y transcodificación");
  await goto(`/studio/${courseId}/1`, name);
  await page
    .locator("summary")
    .filter({ hasText: /^Agregar recurso$/ })
    .click();
  const videoForm = page.locator('form[id^="add-resource-"]');
  await videoForm.getByLabel("Título", { exact: true }).fill("Video de prueba");
  await videoForm.getByLabel("Tipo", { exact: true }).selectOption("video");
  await videoForm.getByLabel("ID del archivo").fill(videoId);
  await videoForm.getByLabel("Obligatorio", { exact: true }).uncheck();
  await videoForm
    .getByRole("button", { name: "Agregar recurso", exact: true })
    .click();
  await page
    .locator("summary")
    .filter({ hasText: "Video de prueba" })
    .waitFor();
  await resource("Evaluación inicial", "quiz");
  await page
    .locator(".resource-editor summary")
    .filter({ hasText: "Evaluación inicial" })
    .click();
  await page.getByRole("link", { name: "Configurar evaluación ↗" }).click();
  await page.getByRole("heading", { name: "Diseña una evaluación." }).waitFor();
  await page
    .getByRole("button", { name: "Agregar pregunta", exact: true })
    .click();
  await page
    .getByLabel("Enunciado")
    .fill("¿Dónde vive el estado transaccional?");
  await page.getByLabel("Opción 1", { exact: true }).fill("PostgreSQL");
  await page.getByLabel("Opción 2", { exact: true }).fill("En el navegador");
  await page.getByRole("button", { name: "Guardar evaluación" }).click();
  await page
    .getByRole("status")
    .filter({ hasText: "Evaluación guardada." })
    .waitFor();
  await goto(`/studio/${courseId}/1`, name);
  await captura("profesor-version-antes-de-publicar");
  await page.getByRole("button", { name: "Validar contenido" }).click();
  await page.getByText("Todo listo para publicar.").waitFor();
  await page.getByRole("button", { name: "Publicar versión ↗" }).click();
  await page.getByRole("button", { name: "Confirmar publicación" }).click();
  await page
    .getByText("Esta versión es inmutable.", { exact: false })
    .waitFor();
  await logout();
  // Una profesora no puede modificar un curso ajeno: el servidor lo rechaza
  // aunque la petición no pase por la interfaz.
  await login("profesora2@mooc.local");
  await goto("/studio", "Comparte lo que sabes.");
  assert.equal(
    await page.getByRole("heading", { name, exact: true }).count(),
    0,
    "el curso ajeno no aparece entre los cursos de la profesora",
  );
  assert.equal(
    await apiStatus("PATCH", `/courses/${courseId}`, { title: "Ajeno" }),
    403,
    "editar los datos de un curso ajeno",
  );
  assert.equal(
    await apiStatus("POST", `/courses/${courseId}/versions`),
    403,
    "crear una versión de un curso ajeno",
  );
  assert.equal(
    await apiStatus("POST", `/courses/${courseId}/versions/1/unpublish`),
    403,
    "retirar la publicación de un curso ajeno",
  );
  await logout();
  await login("estudiante2@mooc.local");
  // Un estudiante no ve el estudio ni puede usar sus operaciones.
  await goto("/studio", "Esta sección requiere otro rol");
  assert.equal(
    await page.getByRole("link", { name: "Estudio", exact: true }).count(),
    0,
    "el menú del estudiante no ofrece el estudio",
  );
  assert.equal(
    await apiStatus("POST", "/courses", {
      title: "Curso de un estudiante",
      summary: "No debe crearse.",
      category: "prueba",
      language: "es",
    }),
    403,
    "un estudiante crea un curso",
  );
  assert.equal(
    await apiStatus("PATCH", `/courses/${courseId}`, { title: "Ajeno" }),
    403,
    "un estudiante edita un curso",
  );
  assert.equal(
    await apiStatus("POST", `/courses/${courseId}/versions/1/publish`),
    403,
    "un estudiante publica un curso",
  );
  await goto("/catalog", "Siempre hay algo por descubrir.");
  await captura("estudiante-catalogo");
  await page.getByLabel("Buscar cursos").fill(name);
  await page.getByRole("button", { name: "Buscar ↗", exact: true }).click();
  await page.getByRole("heading", { name: name, exact: true }).waitFor();
  // Atado a la tarjeta de ESTE curso. «Explorar curso ↗» está en todas, y con
  // el catálogo poblado por los datos de carga el localizador suelto encuentra
  // veinte y no sabe cuál pulsar.
  await page
    .locator("article")
    .filter({ has: page.getByRole("heading", { name: name, exact: true }) })
    .getByRole("link", { name: "Explorar curso ↗", exact: true })
    .click();
  await captura("estudiante-detalle-del-curso");
  await page.getByRole("button", { name: "Inscribirme al curso ↗" }).click();
  await page
    .getByRole("heading", { name: "Lectura de bienvenida", exact: true })
    .waitFor();
  await page
    .getByRole("heading", { name: "Bienvenido", exact: true })
    .waitFor();
  // Ya con la leccion cargada: esto si es el aula.
  await captura("estudiante-aula-con-la-leccion");
  await page.waitForTimeout(16500);
  await page.getByRole("button", { name: /Imagen de prueba/ }).click();
  await page.locator("#reader img").waitFor();
  await page.waitForFunction(
    () => document.querySelector("#reader img")?.naturalWidth > 0,
  );
  let playbackSessions = 0;
  await page.route("**/enrollments/*/media-sessions", async (route) => {
    const response = await route.fetch();
    const body = await response.json();
    body.expires_at = new Date(Date.now() + 68000).toISOString();
    playbackSessions++;
    await route.fulfill({ response, json: body });
  });
  await page.getByRole("button", { name: /Video de prueba/ }).click();
  await page.locator("#reader video").waitFor();
  await page.waitForFunction(
    () => document.querySelector("video")?.readyState >= 2,
  );
  await page.evaluate(async () => {
    const video = document.querySelector("video");
    video.muted = true;
    await Promise.race([
      video.play(),
      new Promise((_, reject) =>
        setTimeout(
          () =>
            reject(
              new Error("El video no comenzó a reproducirse en 15 segundos"),
            ),
          15000,
        ),
      ),
    ]);
  });
  await page.waitForFunction(
    () => document.querySelector("video")?.currentTime > 0.5,
  );
  await page.waitForFunction(
    () => document.querySelector("video")?.dataset.playbackError === undefined,
  );
  for (let i = 0; i < 20 && playbackSessions < 2; i++)
    await page.waitForTimeout(500);
  assert.ok(
    playbackSessions >= 2,
    "la credencial de reproducción debe renovarse",
  );
  await page.unroute("**/enrollments/*/media-sessions");
  await page.getByRole("button", { name: /Evaluación inicial/ }).click();
  await page
    .getByRole("button", { name: "Iniciar intento", exact: true })
    .click();
  await page
    .getByText("¿Dónde vive el estado transaccional?", { exact: false })
    .waitFor();
  await page.getByLabel("PostgreSQL", { exact: true }).check();
  await page
    .getByRole("button", { name: "Guardar respuestas", exact: true })
    .click();
  await page
    .getByRole("status")
    .filter({ hasText: "Respuestas guardadas." })
    .waitFor();
  await page.getByRole("button", { name: "Entregar evaluación ↗" }).click();
  await page.getByRole("button", { name: "Entregar", exact: true }).click();
  await page
    .getByRole("heading", { name: "Objetivo alcanzado.", exact: true })
    .waitFor();
  // El porcentaje lo calcula el servidor con las evidencias de la lección.
  await goto("/learning");
  await page.getByRole("heading", { name: /Sigue avanzando/ }).waitFor();
  const progreso = await page
    .locator("article.row", { hasText: name })
    .locator(".meta")
    .textContent();
  assert.ok(
    Number(progreso.match(/([\d.]+)% completado/)?.[1]) > 0,
    `el curso debe mostrar progreso registrado: ${progreso}`,
  );
  await goto("/badges", "Lo aprendido deja huella.");
  for (
    let i = 0;
    i < 20 &&
    !(await page.getByRole("heading", { name: name, exact: true }).count());
    i++
  ) {
    await page.waitForTimeout(1000);
    await page.reload();
    await page
      .getByRole("heading", { name: "Lo aprendido deja huella." })
      .waitFor();
  }
  await page.getByRole("heading", { name: name, exact: true }).waitFor();
  await page
    .getByRole("link", { name: "Verificar y compartir ↗" })
    .last()
    .click();
  await page
    .getByRole("heading", { name: "Estudiante Dos", exact: true })
    .waitFor();
  await logout();
  await login("admin@mooc.local");
  await goto("/admin", "Una plataforma en marcha.");
  await page.getByRole("link", { name: "Administrar ↗" }).first().waitFor();
  await page.getByRole("link", { name: "Administrar ↗" }).first().click();
  await page.getByRole("heading", { name: "Sesiones activas" }).waitFor();
  await goto("/admin/audit", "Una plataforma en marcha.");
  await page.locator("#records article").first().waitFor();
  await goto("/admin/jobs", "Una plataforma en marcha.");
  await page.locator("#records article").first().waitFor();
  await goto("/media", "De tu archivo al aula.");
  await page.setViewportSize({ width: 390, height: 844 });
  await goto("/", "Aprender nos transforma.");
  assert.equal(
    await page.evaluate(
      () => document.documentElement.scrollWidth > innerWidth,
    ),
    false,
  );
  await page.screenshot({ path: `${evidencias}/movil.png`, fullPage: true });
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.screenshot({
    path: `${evidencias}/escritorio.png`,
    fullPage: true,
  });
  assert.deepEqual(errors, []);
  console.log(
    JSON.stringify({
      passed: true,
      courseId,
      course: name,
      browserErrors: errors,
    }),
  );
} catch (err) {
  console.error(
    JSON.stringify({
      url: page.url(),
      errors,
      alerts: await page.locator(".error").allTextContents(),
      video: await page.locator("#video-info").allTextContents(),
      media: await page.locator("video").evaluateAll((vs) =>
        vs.map((v) => ({
          failure: v.dataset.playbackError,
          type: v.dataset.playbackErrorType,
          code: v.error?.code,
          readyState: v.readyState,
        })),
      ),
      headings: await page.locator("h1,h2").allTextContents(),
    }),
  );
  await page.screenshot({ path: "/tmp/mooc-e2e-failure.png", fullPage: true });
  throw err;
} finally {
  await browser.close();
}
