// Prepara el escenario 2: sube una vez cada perfil, espera a que estén en
// `ready`, crea un curso publicado con los tres videos e inscribe a los
// espectadores. Deja /resultados/medios.json (con tokens: no se versiona).
//
// Necesita /resultados/datos.json de preparar.js (las sesiones de los
// estudiantes) y los archivos de make medios-carga en MEDIOS (/medios).
import { fail } from 'k6';
import { open as abrir } from 'k6/experimental/fs';
import { post, json, login } from './comun.js';
import { subir, esperar } from './medios.js';

const MEDIOS = __ENV.MEDIOS || '/medios';
const ESPECTADORES = Number(__ENV.ESPECTADORES || 100);
const manifiesto = open(`${MEDIOS}/manifiesto.jsonl`).trim().split('\n').map((l) => JSON.parse(l));
const datos = JSON.parse(open(__ENV.DATOS || '/resultados/datos.json'));
const archivos = {};
for (const m of manifiesto) archivos[m.perfil] = await abrir(`${MEDIOS}/${m.perfil}.mp4`);

export const options = {
  setupTimeout: '90m',
  scenarios: { nada: { executor: 'shared-iterations', vus: 1, iterations: 1 } },
};

export async function setup() {
  const prof = login('profesor-carga-01@carga.local');
  const videos = [];
  for (const m of manifiesto) {
    const a = await subir(archivos[m.perfil], m, prof, m.perfil);
    if (!a) fail(`no se pudo subir el perfil ${m.perfil}`);
    const estado = esperar(a, prof, m.perfil, 80 * 60);
    if (estado !== 'ready') fail(`el perfil ${m.perfil} terminó en ${estado}`);
    videos.push({ perfil: m.perfil, asset: a.id });
    console.log(`perfil ${m.perfil} listo`);
  }

  let r = post('/courses', { title: 'Curso multimedia de carga', summary: 'Videos del escenario 2.', category: 'carga-medios', language: 'es' }, prof, 'POST /courses', true);
  const c = json(r);
  r = post(`/versions/${c.version.id}/modules`, { title: 'Videos' }, prof, 'POST /modules');
  r = post(`/modules/${json(r).id}/units`, { title: 'Perfiles A, B y C' }, prof, 'POST /units');
  const unidad = json(r).id;
  for (const v of videos) {
    r = post(`/units/${unidad}/resources`, { title: `Video perfil ${v.perfil}`, type: 'video', asset_id: v.asset }, prof, 'POST /resources');
    if (r.status !== 201) fail(`recurso de video: ${r.status} ${r.body}`);
    v.recurso = json(r).stable_id;
  }
  r = post(`/courses/${c.course.id}/versions/1/publish`, null, prof, 'POST /publish', true);
  if (r.status !== 200) fail(`publicar: ${r.status} ${r.body}`);

  const espectadores = [];
  for (const s of datos.estudiantes.slice(0, ESPECTADORES)) {
    r = post('/enrollments', { course_id: c.course.id }, s.token, 'POST /enrollments', true);
    if (r.status !== 201) fail(`inscribir espectador: ${r.status} ${r.body}`);
    espectadores.push({ token: s.token, inscripcion: json(r).id });
  }
  console.log(`curso multimedia listo: ${videos.length} videos, ${espectadores.length} espectadores`);
  return { curso: c.course.id, videos, espectadores };
}

export default function () {}

export function handleSummary(resumen) {
  if (!resumen.setup_data) return {};
  return { [__ENV.SALIDA || '/resultados/medios.json']: JSON.stringify(resumen.setup_data) };
}
