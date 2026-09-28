// Escenario 1 — Actividad académica concurrente
// (capacity-planning/pruebas_de_carga_entrega2.md, §5).
//
// Modelo abierto: la tasa de llegada no depende de lo que tarde el servidor,
// que es lo que deja ver la saturación. Un nivel es:
//
//   k6 run -e TASA=8 -e DURACION=8m -e ETIQUETA=e1-L2 escenario1.js
//
// TASA en iteraciones por segundo; 2 minutos de calentamiento a la misma tasa,
// etiquetados aparte (scenario:calentamiento) y fuera de la medición.
// INTEGRIDAD=1 añade las comprobaciones de §5.5 durante el nivel.
import http from 'k6/http';
import { sleep, check } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { BASE, get, post, patch, json, pausa, clave, iteracion, resumir, submetricas } from './comun.js';

const TASA = Number(__ENV.TASA || 1);
const DURACION = __ENV.DURACION || '8m';
const CALENTAMIENTO = __ENV.CALENTAMIENTO || '2m';
const ETIQUETA = __ENV.ETIQUETA || `e1-${TASA}`;
// Una iteración dura del orden de 30 s: con esto no faltan usuarios virtuales.
const VUS = Math.ceil(TASA * 40);

const OPERACIONES = [
  'GET /catalog/courses', 'GET /catalog/courses/{slug}', 'POST /enrollments',
  'GET /content', 'POST /progress', 'GET /progress',
  'POST /attempts', 'PATCH /answers', 'POST /submit',
];

const datos = new SharedArray('datos', () => {
  const d = JSON.parse(open(__ENV.DATOS || '/resultados/datos.json'));
  return [d];
});

const escenario = (inicio, duracion) => ({
  executor: 'constant-arrival-rate',
  // Por minuto: k6 exige una tasa entera, y así caben tasas como 0,5/s.
  rate: Math.round(TASA * 60),
  timeUnit: '1m',
  duration: duracion,
  startTime: inicio,
  preAllocatedVUs: VUS,
  maxVUs: VUS * 2,
  exec: 'estudiar',
  gracefulStop: '45s',
});

export const options = {
  scenarios: Object.assign(
    { calentamiento: escenario('0s', CALENTAMIENTO), medicion: escenario(CALENTAMIENTO, DURACION) },
    __ENV.INTEGRIDAD === '1' ? {
      integridad: {
        executor: 'constant-arrival-rate', rate: 6, timeUnit: '1m', duration: DURACION,
        startTime: CALENTAMIENTO, preAllocatedVUs: 4, maxVUs: 8, exec: 'integridad',
      },
    } : {},
  ),
  thresholds: submetricas(OPERACIONES),
  summaryTrendStats: ['med', 'p(95)', 'p(99)', 'avg', 'max'],
};

// Cada estudiante con su cuenta, su curso y su intento: dos iteraciones
// concurrentes no comparten fila salvo cuando la prueba quiere contención.
//
// El índice se permuta: multiplicar por un primo coprimo con el número de
// cuentas recorre todas sin repetir en N iteraciones seguidas, y reparte
// igual las ya inscritas y las que se inscriben. Con el índice tal cual, cada
// nivel empezaría por las primeras cuentas, todas inscritas, y la mezcla
// cambiaría de un nivel a otro. La medición arranca desplazada para no
// coincidir con las iteraciones del calentamiento que aún terminan.
const PRIMO = 7919;
function estudiante() {
  const e = datos[0].estudiantes;
  const desp = exec.scenario.name === 'medicion' ? Math.floor(e.length / 2) : 0;
  return e[((iteracion() + desp) * PRIMO) % e.length];
}

// Pausa de lectura entre acciones, 1 a 3 s (§5.2).
const leer = () => sleep(pausa(1, 3));

export function estudiar() {
  const s = estudiante();
  const t = s.token;

  // 1-2. Catálogo y ficha del curso.
  let r = get('/catalog/courses?category=carga&limit=50', null, 'GET /catalog/courses');
  const cat = json(r);
  check(r, { 'catálogo con cursos': () => r.status === 200 && cat && cat.items.length > 0 });
  const curso = cat && cat.items.find((c) => c.id === s.curso);
  leer();
  if (curso) {
    r = get(`/catalog/courses/${curso.slug}`, null, 'GET /catalog/courses/{slug}');
    check(r, { 'ficha del curso': () => r.status === 200 });
    leer();
  }

  // 3. Inscribirse, solo quien no lo estaba (el 15 % de las cuentas). Es un
  // upsert: repetirlo devuelve la misma inscripción.
  let inscripcion = s.inscripcion;
  if (!inscripcion) {
    r = post('/enrollments', { course_id: s.curso }, t, 'POST /enrollments', true);
    const e = json(r);
    check(r, { 'inscripción creada': () => r.status === 201 && e && e.id });
    if (!e || !e.id) return;
    inscripcion = e.id;
    leer();
  }

  // 4. Contenido del curso.
  r = get(`/enrollments/${inscripcion}/content`, t, 'GET /content');
  const cont = json(r);
  const lecturas = cont ? cont.resources.filter((x) => x.type === 'rich_text') : [];
  check(r, { 'contenido con lecturas': () => r.status === 200 && lecturas.length > 1 });
  if (lecturas.length < 2) return;
  leer();

  // 5-6. Abrir dos lecturas y registrar latidos. La cadencia mínima es 10 s
  // por recurso (ADR-0012): se espera lo que falte en vez de chocar con ella,
  // porque un latido rechazado por cadencia sería un fallo del guion.
  const a = lecturas[Math.floor(Math.random() * lecturas.length)];
  let b = lecturas[Math.floor(Math.random() * lecturas.length)];
  if (b.stable_id === a.stable_id) b = lecturas[(lecturas.indexOf(a) + 1) % lecturas.length];
  const ultimo = {};
  const evidencia = (rec, kind) => {
    const espera = ultimo[rec.stable_id] ? 10500 - (Date.now() - ultimo[rec.stable_id]) : 0;
    if (espera > 0) sleep(espera / 1000);
    const x = post(`/enrollments/${inscripcion}/progress`, { resource_stable_id: rec.stable_id, kind },
      t, 'POST /progress', true);
    ultimo[rec.stable_id] = Date.now();
    const ack = json(x);
    check(x, { [`progreso ${kind} aceptado`]: () => x.status >= 200 && x.status < 300 && ack && ack.accepted === true });
  };
  evidencia(a, 'open');
  leer();
  evidencia(b, 'open');
  leer();
  evidencia(a, 'heartbeat');
  leer();
  evidencia(b, 'heartbeat');
  leer();
  evidencia(a, 'heartbeat');
  leer();

  // 7. Consultar el progreso, calculado por el servidor.
  r = get(`/enrollments/${inscripcion}/progress`, t, 'GET /progress');
  const pr = json(r);
  check(r, { 'progreso calculado por el servidor': () => r.status === 200 && pr && typeof pr.progress_pct === 'number' });

  // 8. Presentar un quiz, tres de cada diez iteraciones.
  if (Math.random() < 0.3) {
    leer();
    presentar(inscripcion, t, cont.resources);
  }
}

function presentar(inscripcion, t, recursos) {
  const quizzes = recursos.filter((x) => x.type === 'quiz');
  if (!quizzes.length) return null;
  const q = quizzes[Math.floor(Math.random() * quizzes.length)];
  let r = post(`/enrollments/${inscripcion}/quizzes/${q.stable_id}/attempts`, null, t, 'POST /attempts', true);
  const intento = json(r);
  // 409: la cuenta ya tiene un intento abierto en ese quiz, de una iteración
  // anterior que se cortó a mitad (una corrida saturada). La regla que lo
  // rechaza funciona: es un rechazo de negocio, no una comprobación fallida.
  if (r.status === 409) return null;
  if (!check(r, { 'intento iniciado': () => r.status === 201 && intento && intento.questions.length > 0 })) return null;
  const respuestas = intento.questions.map((p) => ({
    question_stable_id: p.stable_id,
    selected_option_stable_ids: [p.options[Math.floor(Math.random() * p.options.length)].stable_id],
  }));
  r = patch(`/attempts/${intento.id}/answers`, { answers: respuestas }, t, 'PATCH /answers');
  check(r, { 'respuestas guardadas': () => r.status === 200 });
  sleep(pausa(1, 3));
  r = post(`/attempts/${intento.id}/submit`, null, t, 'POST /submit', true);
  const res = json(r);
  check(r, { 'intento calificado': () => r.status === 200 && res && res.score !== null });
  return intento;
}

// §5.5 — integridad bajo concurrencia, durante el nivel y no en reposo.
export function integridad() {
  const lista = datos[0].estudiantes.filter((e) => e.inscripcion);
  const s = lista[Math.floor(Math.random() * lista.length)];
  const otro = lista[Math.floor(Math.random() * lista.length)];
  const t = s.token;

  const cont = json(get(`/enrollments/${s.inscripcion}/content`, t, 'integridad'));
  const quiz = cont && cont.resources.find((x) => x.type === 'quiz');
  if (!quiz) return;

  // La clave nunca sale del servidor, ni en el snapshot del intento (ADR-0013).
  let r = post(`/enrollments/${s.inscripcion}/quizzes/${quiz.stable_id}/attempts`, null, t, 'integridad', true);
  const intento = json(r);
  if (!intento || !intento.id) return;
  check(r, { 'integridad: el snapshot no trae la clave': () => !/is_correct|correct_option/.test(r.body) });
  patch(`/attempts/${intento.id}/answers`, {
    answers: intento.questions.map((p) => ({
      question_stable_id: p.stable_id, selected_option_stable_ids: [p.options[0].stable_id],
    })),
  }, t, 'integridad');

  // Intento ajeno: otro estudiante no puede enviarlo.
  if (otro.email !== s.email) {
    r = post(`/attempts/${intento.id}/submit`, null, otro.token, 'integridad', true);
    check(r, { 'integridad: intento ajeno rechazado': () => r.status === 403 || r.status === 404 });
  }

  // Dos envíos simultáneos del mismo intento: una sola nota.
  const [x, y] = http.batch([
    ['POST', `${BASE}/attempts/${intento.id}/submit`, null, { headers: { Authorization: `Bearer ${t}`, 'Idempotency-Key': clave() }, tags: { name: 'integridad' } }],
    ['POST', `${BASE}/attempts/${intento.id}/submit`, null, { headers: { Authorization: `Bearer ${t}`, 'Idempotency-Key': clave() }, tags: { name: 'integridad' } }],
  ]);
  const notas = [x, y].filter((z) => z.status === 200).map((z) => json(z).score);
  check(null, {
    'integridad: envío simultáneo, una sola calificación': () =>
      notas.length >= 1 && notas.every((n) => n === notas[0]) &&
      [x, y].every((z) => z.status === 200 || z.status === 409),
  });

  // Reenvío con la misma Idempotency-Key: repite la respuesta.
  const k = clave();
  const p1 = post(`/attempts/${intento.id}/submit`, null, t, 'integridad', k);
  const p2 = post(`/attempts/${intento.id}/submit`, null, t, 'integridad', k);
  check(null, { 'integridad: reenvío idempotente': () => p1.status === p2.status && p1.body === p2.body });

  // Un porcentaje enviado por el cliente se rechaza (ADR-0012).
  const lectura = cont.resources.find((z) => z.type === 'rich_text');
  r = post(`/enrollments/${s.inscripcion}/progress`,
    { resource_stable_id: lectura.stable_id, kind: 'heartbeat', progress_percent: 100 }, t, 'integridad', true);
  const ack = json(r);
  check(r, { 'integridad: porcentaje del cliente rechazado': () => r.status >= 400 || (ack && ack.accepted === false) });
}

export function handleSummary(d) {
  return resumir(d, ETIQUETA, OPERACIONES);
}
