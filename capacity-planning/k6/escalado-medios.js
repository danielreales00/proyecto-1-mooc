// Ciclo de escalado de los workers (Sesión C, C4): N videos del perfil C
// subidos a la vez. Cada usuario virtual sube uno y espera su estado terminal;
// la consola deja la hora de cada confirmación y de cada final.
//
//   k6 run -e BASE_URL=https://<host> -e CLAVE=... [-e VIDEOS=5] escalado-medios.js
//
// Los videos y su manifiesto salen de make carga-medios, montados en /medios.
import { open as abrir } from 'k6/experimental/fs';
import exec from 'k6/execution';
import { login } from './comun.js';
import { subir, esperar } from './medios.js';

const N = Number(__ENV.VIDEOS || 5);
const meta = open('/medios/manifiesto.jsonl').trim().split('\n').map((l) => JSON.parse(l)).find((m) => m.perfil === 'c');
const archivo = await abrir('/medios/c.mp4');

export const options = {
  scenarios: { videos: { executor: 'per-vu-iterations', vus: N, iterations: 1, maxDuration: '60m' } },
  setupTimeout: '2m',
};

export function setup() {
  return { token: login(__ENV.EMAIL || 'profesor@mooc.local') };
}

export default async function (d) {
  const n = exec.vu.idInTest;
  const a = await subir(archivo, meta, d.token, 'c');
  if (!a) {
    console.log(`video ${n}: la subida falló`);
    return;
  }
  console.log(`video ${n}: asset ${a.id} confirmado ${new Date(a.confirmada).toISOString()}`);
  const estado = esperar(a, d.token, 'c', 55 * 60);
  console.log(`video ${n}: asset ${a.id} terminó en ${estado} ${new Date().toISOString()}`);
}
