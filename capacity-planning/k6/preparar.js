// Prepara los datos del escenario 1 por la API: cursos publicados, sesiones
// de los estudiantes, inscripciones e intentos previos. Las cuentas ya existen
// (make semilla-nube con SEED_CARGA_*), escritas en la base.
//
// Un solo usuario virtual, a ritmo controlado: cada inicio de sesión es un
// argon2id de 64 MiB en el Web Server, y la preparación no debe tumbar la
// máquina que luego se mide. Deja el resultado en $SALIDA (por defecto
// /resultados/datos.json), que incluye tokens de sesión: se queda en la
// máquina del generador y NUNCA se versiona.
//
// Variables: ESTUDIANTES (600), PROFESORES (10), CURSOS (20), MODULOS (3),
// UNIDADES (4), RECURSOS (5), PREGUNTAS (10), INSCRITOS (0.85), INTENTOS (200),
// LOGINS_POR_SEGUNDO (4).
import { sleep, fail } from 'k6';
import { get, post, put, patch, json, login, esperado } from './comun.js';

const N = (k, d) => Number(__ENV[k] || d);
const ESTUDIANTES = N('ESTUDIANTES', 600);
const PROFESORES = N('PROFESORES', 10);
const CURSOS = N('CURSOS', 20);
const MODULOS = N('MODULOS', 3);
const UNIDADES = N('UNIDADES', 4);
const RECURSOS = N('RECURSOS', 5);
const PREGUNTAS = N('PREGUNTAS', 10);
const INSCRITOS = N('INSCRITOS', 0.85);
const INTENTOS = N('INTENTOS', 200);
const LOGINS_POR_SEGUNDO = N('LOGINS_POR_SEGUNDO', 4);

// Todo ocurre en setup(): es lo único cuyo resultado llega a handleSummary
// (como setup_data), que es lo único que puede escribir un archivo.
export const options = {
  setupTimeout: '60m',
  scenarios: { nada: { executor: 'shared-iterations', vus: 1, iterations: 1 } },
};

const pad = (n, w) => String(n).padStart(w, '0');

function crearCurso(token, i) {
  let r = post('/courses', {
    title: `Curso de carga ${pad(i, 2)}`,
    summary: `Curso sintético ${i} para las pruebas de capacidad.`,
    category: 'carga',
    language: 'es',
  }, token, 'POST /courses', true);
  if (r.status !== 201) fail(`crear curso ${i}: ${r.status} ${r.body}`);
  const c = json(r);
  const cursoId = c.course.id;
  const versionId = c.version.id;

  for (let m = 1; m <= MODULOS; m++) {
    r = post(`/versions/${versionId}/modules`, { title: `Módulo ${m}` }, token, 'POST /modules');
    if (r.status !== 201) fail(`módulo: ${r.status} ${r.body}`);
    const moduloId = json(r).id;
    for (let u = 1; u <= UNIDADES; u++) {
      r = post(`/modules/${moduloId}/units`, { title: `Unidad ${m}.${u}` }, token, 'POST /units');
      if (r.status !== 201) fail(`unidad: ${r.status} ${r.body}`);
      const unidadId = json(r).id;
      for (let k = 1; k <= RECURSOS; k++) {
        r = post(`/units/${unidadId}/resources`, {
          title: `Lectura ${m}.${u}.${k}`,
          type: 'rich_text',
          content_md: `# Lectura ${m}.${u}.${k}\n\nTexto de la lectura ${k} de la unidad ${u}, módulo ${m}.\n\n* Punto uno\n* Punto dos\n`,
        }, token, 'POST /resources');
        if (r.status !== 201) fail(`recurso: ${r.status} ${r.body}`);
      }
      // Un quiz por módulo, en su última unidad.
      if (u === UNIDADES) {
        r = post(`/units/${unidadId}/resources`, { title: `Evaluación del módulo ${m}`, type: 'quiz' }, token, 'POST /resources');
        if (r.status !== 201) fail(`quiz: ${r.status} ${r.body}`);
        const quizId = json(r).id;
        const preguntas = [];
        for (let q = 1; q <= PREGUNTAS; q++) {
          preguntas.push({
            statement_md: `Pregunta ${q} del módulo ${m}`,
            kind: 'single',
            points: 1,
            options: [
              { text_md: 'Correcta', is_correct: true },
              { text_md: 'Incorrecta A', is_correct: false },
              { text_md: 'Incorrecta B', is_correct: false },
            ],
          });
        }
        // max_attempts 0 = ilimitados: cada estudiante presenta el quiz varias
        // veces a lo largo de los niveles, y agotar intentos mediría la regla.
        r = put(`/resources/${quizId}/quiz`, {
          max_attempts: 0, pass_score: 70, feedback_policy: 'correctness', questions: preguntas,
        }, token, 'PUT /quiz');
        if (r.status !== 200) fail(`configurar quiz: ${r.status} ${r.body}`);
      }
    }
  }
  r = post(`/courses/${cursoId}/versions/1/publish`, null, token, 'POST /publish', true);
  if (r.status !== 200) fail(`publicar curso ${i}: ${r.status} ${r.body}`);
  return cursoId;
}

export function setup() {
  const t0 = Date.now();

  // 1. Cursos, repartidos entre los profesores.
  const tokensProf = [];
  for (let p = 1; p <= PROFESORES; p++) {
    tokensProf.push(login(`profesor-carga-${pad(p, 2)}@carga.local`));
    sleep(1 / LOGINS_POR_SEGUNDO);
  }
  const cursos = [];
  for (let i = 1; i <= CURSOS; i++) {
    cursos.push(crearCurso(tokensProf[(i - 1) % PROFESORES], i));
  }
  console.log(`cursos publicados: ${cursos.length} en ${Math.round((Date.now() - t0) / 1000)} s`);

  // 2. Sesiones. A ritmo fijo: cada login es un argon2id de 64 MiB.
  const estudiantes = [];
  for (let e = 1; e <= ESTUDIANTES; e++) {
    const email = `estudiante-carga-${pad(e, 4)}@carga.local`;
    estudiantes.push({ email, token: login(email), curso: cursos[(e - 1) % cursos.length], inscripcion: null });
    sleep(1 / LOGINS_POR_SEGUNDO);
  }
  console.log(`sesiones: ${estudiantes.length}`);

  // 3. Inscripciones previas: el resto se inscribe durante la corrida.
  const inscritos = Math.round(ESTUDIANTES * INSCRITOS);
  for (let e = 0; e < inscritos; e++) {
    const s = estudiantes[e];
    const r = post('/enrollments', { course_id: s.curso }, s.token, 'POST /enrollments', true);
    if (!esperado(r, 201, 'inscripción previa')) fail(`inscribir: ${r.status} ${r.body}`);
    s.inscripcion = json(r).id;
  }
  console.log(`inscripciones previas: ${inscritos}`);

  // 4. Intentos previos, enviados: el escenario no empieza con la tabla vacía.
  let hechos = 0;
  for (let e = 0; e < inscritos && hechos < INTENTOS; e++) {
    hechos += intentoPrevio(estudiantes[e]) ? 1 : 0;
  }
  console.log(`intentos previos: ${hechos}`);

  console.log(`preparación completa en ${Math.round((Date.now() - t0) / 1000)} s`);
  return { creado: new Date().toISOString(), cursos, estudiantes, profesores: tokensProf };
}

export default function () {}

function intentoPrevio(s) {
  const r = get(`/enrollments/${s.inscripcion}/content`, s.token, 'GET /content');
  const quiz = (json(r) || { resources: [] }).resources.find((x) => x.type === 'quiz');
  if (!quiz) return false;
  const a = post(`/enrollments/${s.inscripcion}/quizzes/${quiz.stable_id}/attempts`, null, s.token, 'POST /attempts', true);
  if (a.status !== 201) return false;
  const intento = json(a);
  const respuestas = intento.questions.map((q) => ({
    question_stable_id: q.stable_id,
    selected_option_stable_ids: [q.options[Math.floor(Math.random() * q.options.length)].stable_id],
  }));
  patch(`/attempts/${intento.id}/answers`, { answers: respuestas }, s.token, 'PATCH /answers');
  const env = post(`/attempts/${intento.id}/submit`, null, s.token, 'POST /submit', true);
  return env.status === 200;
}

export function handleSummary(resumen) {
  if (!resumen.setup_data) return {};
  return { [__ENV.SALIDA || '/resultados/datos.json']: JSON.stringify(resumen.setup_data) };
}
