// Escenario 2 — Carga, procesamiento y consumo multimedia (§6).
//
//   k6 run -e CARGAS=3 -e MEZCLA=2/1/0 -e ESPECTADORES=20 -e DURACION=15m \
//          -e ETIQUETA=e2-M1 escenario2.js
//
// CARGAS por minuto con la mezcla A/B/C del nivel; ESPECTADORES reproduciendo
// a la cadencia del video (un segmento cada vez que termina el anterior).
// Las cargas sondean hasta un estado terminal, hasta ESPERA_MAX minutos
// después de confirmar: la corrida termina cuando la cola drenó, y eso es
// parte de lo que se mide.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { SharedArray } from 'k6/data';
import exec from 'k6/execution';
import { open as abrir } from 'k6/experimental/fs';
import { post, json } from './comun.js';
import { subir, esperar, resolver } from './medios.js';

const CARGAS = Number(__ENV.CARGAS || 1);
const MEZCLA = (__ENV.MEZCLA || '1/0/0').split('/').map(Number);
const ESPECTADORES = Number(__ENV.ESPECTADORES || 5);
const DURACION = __ENV.DURACION || '10m';
const ESPERA_MAX = Number(__ENV.ESPERA_MAX || 45);
const ETIQUETA = __ENV.ETIQUETA || `e2-${CARGAS}`;
const MEDIOS = __ENV.MEDIOS || '/medios';

// Secuencia de perfiles que respeta la mezcla: 2/1/0 → a, a, b, a, a, b…
const SECUENCIA = [];
['a', 'b', 'c'].forEach((p, i) => { for (let k = 0; k < MEZCLA[i]; k++) SECUENCIA.push(p); });

const manifiesto = open(`${MEDIOS}/manifiesto.jsonl`).trim().split('\n').map((l) => JSON.parse(l));
const meta = {};
const archivos = {};
for (const m of manifiesto) {
  meta[m.perfil] = m;
  if (SECUENCIA.includes(m.perfil)) archivos[m.perfil] = await abrir(`${MEDIOS}/${m.perfil}.mp4`);
}
const datos = new SharedArray('datos', () => [JSON.parse(open(__ENV.DATOS || '/resultados/datos.json'))]);
const medios = new SharedArray('medios', () => [JSON.parse(open(__ENV.DATOS_MEDIOS || '/resultados/medios.json'))]);

const OPS = ['POST /assets/init', 'POST /assets/complete', 'GET /assets/{id}', 'POST /media-sessions',
  'GET master', 'GET playlist', 'GET segmento (almacén)', 'PUT parte (almacén)'];

export const options = {
  scenarios: Object.assign(
    CARGAS > 0 && SECUENCIA.length ? {
      cargas: {
        executor: 'constant-arrival-rate', rate: CARGAS, timeUnit: '1m', duration: DURACION,
        preAllocatedVUs: 10, maxVUs: 400, exec: 'cargar', gracefulStop: `${ESPERA_MAX}m`,
      },
    } : {},
    ESPECTADORES > 0 ? {
      espectadores: { executor: 'constant-vus', vus: ESPECTADORES, duration: DURACION, exec: 'ver', gracefulStop: '30s' },
    } : {},
  ),
  thresholds: Object.fromEntries(OPS.map((o) => [`http_req_duration{name:${o}}`, ['p(95)>=0']]).concat([
    ['http_req_failed{name:GET segmento (almacén)}', ['rate>=0']],
    ['http_req_failed{name:PUT parte (almacén)}', ['rate>=0']],
    ['completa_a_ready_s{perfil:a}', ['p(95)>=0']], ['completa_a_ready_s{perfil:b}', ['p(95)>=0']],
    ['completa_a_ready_s{perfil:c}', ['p(95)>=0']], ['transferencia_mib_s', ['p(95)>=0']],
    ['checks', ['rate>=0']], ['dropped_iterations', ['count>=0']],
  ])),
  summaryTrendStats: ['med', 'p(95)', 'p(99)', 'avg', 'max', 'count'],
};

export async function cargar() {
  const n = exec.scenario.iterationInTest;
  const perfil = SECUENCIA[n % SECUENCIA.length];
  // Los diez profesores de carga, en rotación: la carga es de varios autores.
  const prof = datos[0].profesores[n % datos[0].profesores.length];
  const asset = await subir(archivos[perfil], meta[perfil], prof, perfil);
  if (asset) esperar(asset, prof, perfil, ESPERA_MAX * 60);
}

// Un espectador: abre una sesión de reproducción, pide el master y la
// variante más baja, y descarga los segmentos a la cadencia del video.
export function ver() {
  const esp = medios[0].espectadores[(exec.vu.idInTest - 1) % medios[0].espectadores.length];
  const video = medios[0].videos[Math.floor(Math.random() * medios[0].videos.length)];
  let r = post(`/enrollments/${esp.inscripcion}/media-sessions`, { resource_stable_id: video.recurso },
    esp.token, 'POST /media-sessions', true);
  const ses = json(r);
  if (!check(r, { 'sesión de reproducción': () => (r.status === 200 || r.status === 201) && ses && ses.manifest_url })) {
    sleep(5);
    return;
  }
  r = http.get(ses.manifest_url, { tags: { name: 'GET master' } });
  if (!check(r, { 'master HLS': () => r.status === 200 && r.body.includes('#EXTM3U') })) return;
  const variantes = r.body.split('\n').filter((l) => l && !l.startsWith('#'));
  const url = resolver(ses.manifest_url, variantes[0].trim());
  r = http.get(url, { tags: { name: 'GET playlist' } });
  if (!check(r, { 'playlist HLS': () => r.status === 200 })) return;

  const lineas = r.body.split('\n');
  const maxSeg = Number(__ENV.SEGMENTOS_POR_SESION || 20);
  let seg = 0;
  for (let i = 0; i < lineas.length && seg < maxSeg; i++) {
    if (!lineas[i].startsWith('#EXTINF:')) continue;
    const dur = parseFloat(lineas[i].substring(8));
    const s = http.get(resolver(url, lineas[i + 1].trim()), { tags: { name: 'GET segmento (almacén)' }, responseType: 'none' });
    check(s, { 'segmento descargado': (x) => x.status === 200 });
    seg++;
    // Cadencia de reproducción: el siguiente segmento cuando se acaba este.
    const resto = dur - s.timings.duration / 1000;
    if (resto > 0) sleep(resto);
  }
}

export function handleSummary(d) {
  const m = d.metrics;
  const v = (n, k) => (m[n] && m[n].values ? m[n].values[k] : undefined);
  const s = (x, u) => (x === undefined ? '-' : `${Number(x).toFixed(u === 'ms' ? 0 : 1)} ${u}`);
  const l = [`== ${ETIQUETA}  (cargas/min ${CARGAS}, mezcla ${MEZCLA.join('/')}, espectadores ${ESPECTADORES})`];
  for (const o of OPS) {
    const k = `http_req_duration{name:${o}}`;
    if (m[k]) l.push(`  ${o.padEnd(26)} p50 ${s(v(k, 'med'), 'ms').padStart(9)}  p95 ${s(v(k, 'p(95)'), 'ms').padStart(9)}  n ${v(k, 'count')}`);
  }
  for (const p of ['a', 'b', 'c']) {
    const k = `completa_a_ready_s{perfil:${p}}`;
    if (m[k] && v(k, 'count')) l.push(`  completa→ready perfil ${p}: p50 ${s(v(k, 'med'), 's')}  p95 ${s(v(k, 'p(95)'), 's')}  máx ${s(v(k, 'max'), 's')}  n ${v(k, 'count')}`);
  }
  l.push(`  transferencia directa: p50 ${s(v('transferencia_mib_s', 'med'), 'MiB/s')}`);
  l.push(`  segmentos fallidos: ${s((v('http_req_failed{name:GET segmento (almacén)}', 'rate') || 0) * 100, '%')}`);
  l.push(`  estados finales: ${Object.keys(m).filter((k) => k.startsWith('estado_final')).map((k) => k).join(', ') || v('estado_final', 'count') || 0}`);
  l.push(`  comprobaciones: ${s((v('checks', 'rate') || 0) * 100, '%')}   iteraciones perdidas: ${v('dropped_iterations', 'count') || 0}`);
  return { stdout: l.join('\n') + '\n', [`/resultados/${ETIQUETA}.json`]: JSON.stringify(d, null, 1) };
}
