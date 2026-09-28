// Carga directa de archivos al almacenamiento, como la haría un cliente: la API
// solo autoriza y confirma; los bytes van por las URL firmadas de cada parte.
import http from 'k6/http';
import { check, sleep } from 'k6';
import { SeekMode } from 'k6/experimental/fs';
import { Trend, Counter } from 'k6/metrics';
import { BASE, post, get, json } from './comun.js';

export const transferencia = new Trend('transferencia_s');
export const tasaTransferencia = new Trend('transferencia_mib_s');
export const completaAReady = new Trend('completa_a_ready_s');
export const estadoFinal = new Counter('estado_final');

const PARALELAS = Number(__ENV.PARTES_EN_PARALELO || 4);
const TERMINALES = ['ready', 'failed', 'infected', 'rejected'];

async function leerParte(archivo, desde, cuantos) {
  const buf = new Uint8Array(cuantos);
  await archivo.seek(desde, SeekMode.Start);
  let leidos = 0;
  while (leidos < cuantos) {
    const n = await archivo.read(buf.subarray(leidos));
    if (n === null || n === 0) break;
    leidos += n;
  }
  return buf.buffer;
}

// subir hace init → PUT de cada parte → complete. Devuelve el asset o null.
export async function subir(archivo, meta, token, perfil) {
  let r = post('/assets/init', {
    filename: `carga-${perfil}.mp4`, content_type: 'video/mp4',
    size_bytes: meta.bytes, sha256: meta.sha256, kind: 'video',
  }, token, 'POST /assets/init', true);
  const t = json(r);
  if (!check(r, { 'carga autorizada': () => r.status === 201 && t && t.parts.length > 0 })) {
    console.error(`init de ${perfil}: HTTP ${r.status} ${String(r.body).slice(0, 300)}`);
    return null;
  }

  const inicio = Date.now();
  const etags = [];
  for (let i = 0; i < t.parts.length; i += PARALELAS) {
    const lote = t.parts.slice(i, i + PARALELAS);
    const cuerpos = [];
    for (const p of lote) {
      const desde = (p.part_number - 1) * t.part_size;
      cuerpos.push(await leerParte(archivo, desde, Math.min(t.part_size, meta.bytes - desde)));
    }
    const res = await Promise.all(lote.map((p, k) =>
      http.asyncRequest('PUT', p.url, cuerpos[k], { tags: { name: 'PUT parte (almacén)' } })));
    for (let k = 0; k < lote.length; k++) {
      if (!check(res[k], { 'parte subida al almacén': (x) => x.status === 200 })) {
        console.error(`parte ${lote[k].part_number} de ${perfil}: HTTP ${res[k].status} ${String(res[k].body).slice(0, 300)}`);
        return null;
      }
      etags.push({ part_number: lote[k].part_number, etag: (res[k].headers.Etag || res[k].headers.ETag || '').replace(/"/g, '') });
    }
  }
  const segundos = (Date.now() - inicio) / 1000;
  transferencia.add(segundos, { perfil });
  tasaTransferencia.add(meta.bytes / 1048576 / segundos, { perfil });

  r = post(`/assets/${t.asset_id}/complete`, { parts: etags }, token, 'POST /assets/complete', true);
  if (!check(r, { 'carga confirmada (202)': () => r.status === 202 })) {
    console.error(`complete de ${perfil}: HTTP ${r.status} ${String(r.body).slice(0, 300)}`);
    return null;
  }
  return { id: t.asset_id, confirmada: Date.now() };
}

// esperar sondea hasta un estado terminal. Un 202 de aceptación no es una
// transcodificación exitosa: esto es lo que distingue las dos cosas (§6.5).
export function esperar(asset, token, perfil, maxSegundos) {
  const limite = asset.confirmada + maxSegundos * 1000;
  let estado = 'desconocido';
  while (Date.now() < limite) {
    const r = get(`/assets/${asset.id}`, token, 'GET /assets/{id}');
    const a = json(r);
    if (a && TERMINALES.includes(a.status)) {
      estado = a.status;
      break;
    }
    sleep(5);
  }
  if (estado === 'desconocido') estado = 'sin_terminar';
  estadoFinal.add(1, { estado, perfil });
  if (estado === 'ready') completaAReady.add((Date.now() - asset.confirmada) / 1000, { perfil });
  check(null, { 'carga terminada en ready': () => estado === 'ready' });
  return estado;
}

// resolver una URI de un m3u8 contra la del manifiesto que la contiene.
export function resolver(base, uri) {
  if (/^https?:\/\//.test(uri)) return uri;
  const sinQuery = base.split('?')[0];
  if (uri.startsWith('/')) return sinQuery.replace(/^(https?:\/\/[^/]+).*$/, '$1') + uri;
  return sinQuery.substring(0, sinQuery.lastIndexOf('/') + 1) + uri;
}

export { BASE };
