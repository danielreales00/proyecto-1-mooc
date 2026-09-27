// Lo que comparten los guiones de carga (capacity-planning/pruebas_de_carga_entrega2.md).
//
// Variables de entorno:
//   BASE_URL        origen de la API, sin /api/v1
//   CLAVE           contraseña de las cuentas sintéticas (de Secret Manager en
//                   la nube; nunca en el repositorio ni en los resultados)
//   DATOS           archivo que deja preparar.js con cuentas, cursos y sesiones
import http from 'k6/http';
import { check, fail } from 'k6';
import exec from 'k6/execution';
import { Counter } from 'k6/metrics';

// El informe separa lo que falla de lo que se rechaza con razón (§8): un 429
// o una regla de negocio no son errores del sistema, y mezclarlos esconde los
// fallos reales.
export const errores5xx = new Counter('errores_5xx');
export const timeouts = new Counter('timeouts');
export const limitados429 = new Counter('limitados_429');
export const rechazos4xx = new Counter('rechazos_4xx');

function registrar(r) {
  if (r.status === 0) timeouts.add(1);
  else if (r.status >= 500) errores5xx.add(1);
  else if (r.status === 429) limitados429.add(1);
  else if (r.status >= 400) rechazos4xx.add(1);
  return r;
}

export const BASE = (__ENV.BASE_URL || 'http://localhost:8090') + '/api/v1';
export const CLAVE = __ENV.CLAVE || '';

// Idempotency-Key única por petición: un reintento del generador no debe
// contar como la misma operación, salvo cuando la prueba quiere justo eso.
export function clave() {
  // UUID v4 suficiente para claves de idempotencia; no es criptografía.
  return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
    const r = (Math.random() * 16) | 0;
    return (c === 'x' ? r : (r & 0x3) | 0x8).toString(16);
  });
}

export function cabeceras(token, extra) {
  const h = { 'Content-Type': 'application/json' };
  if (token) h.Authorization = `Bearer ${token}`;
  return Object.assign(h, extra || {});
}

// nombre agrupa las URL con identificadores en una sola serie por operación:
// sin él, cada UUID sería una métrica distinta y el resumen sería ilegible.
export function get(url, token, nombre) {
  return registrar(http.get(BASE + url, { headers: cabeceras(token), tags: { name: nombre } }));
}

export function post(url, cuerpo, token, nombre, idem) {
  const h = cabeceras(token, idem ? { 'Idempotency-Key': idem === true ? clave() : idem } : {});
  return registrar(http.post(BASE + url, cuerpo === null ? null : JSON.stringify(cuerpo), {
    headers: h,
    tags: { name: nombre },
  }));
}

export function patch(url, cuerpo, token, nombre) {
  return registrar(http.patch(BASE + url, JSON.stringify(cuerpo), { headers: cabeceras(token), tags: { name: nombre } }));
}

export function put(url, cuerpo, token, nombre) {
  return registrar(http.put(BASE + url, JSON.stringify(cuerpo), { headers: cabeceras(token), tags: { name: nombre } }));
}

export function json(r) {
  try {
    return r.json();
  } catch (_) {
    return null;
  }
}

// Inicia sesión y devuelve el token, o falla la preparación: sin sesión no
// hay recorrido que medir.
export function login(email) {
  const r = post('/auth/login', { email, password: CLAVE }, null, 'POST /auth/login');
  if (r.status !== 200) fail(`login de ${email}: HTTP ${r.status} ${r.body}`);
  return json(r).token;
}

export function esperado(r, estado, nombre) {
  return check(r, { [`${nombre}: ${estado}`]: (x) => x.status === estado });
}

export function pausa(min, max) {
  return min + Math.random() * (max - min);
}

export function iteracion() {
  return exec.scenario.iterationInTest;
}

// Resumen propio: el de k6 no distingue calentamiento de medición. Escribe el
// JSON completo en /resultados y deja en pantalla lo que va a la tabla.
export function resumir(datos, etiqueta, operaciones) {
  const m = datos.metrics;
  const v = (n, k) => (m[n] && m[n].values ? m[n].values[k] : undefined);
  const ms = (x) => (x === undefined ? '-' : `${Math.round(x)} ms`);
  const lineas = [`== ${etiqueta}`];
  const dur = 'http_req_duration{scenario:medicion}';
  lineas.push(`peticiones medidas: ${v('http_reqs{scenario:medicion}', 'count')}  ` +
    `(${(v('http_reqs{scenario:medicion}', 'rate') || 0).toFixed(1)}/s)`);
  lineas.push(`latencia p50 ${ms(v(dur, 'med'))}  p95 ${ms(v(dur, 'p(95)'))}  p99 ${ms(v(dur, 'p(99)'))}`);
  for (const n of ['errores_5xx', 'timeouts', 'limitados_429', 'rechazos_4xx']) {
    lineas.push(`${n}: ${v(`${n}{scenario:medicion}`, 'count') || 0}`);
  }
  lineas.push(`iteraciones perdidas por el generador: ${v('dropped_iterations', 'count') || 0}`);
  const ch = v('checks{scenario:medicion}', 'rate');
  lineas.push(`comprobaciones funcionales: ${ch === undefined ? '-' : (ch * 100).toFixed(2) + ' %'}`);
  for (const op of operaciones || []) {
    const k = `http_req_duration{scenario:medicion,name:${op}}`;
    if (m[k]) lineas.push(`  ${op.padEnd(34)} p50 ${ms(v(k, 'med')).padStart(8)}  p95 ${ms(v(k, 'p(95)')).padStart(8)}  p99 ${ms(v(k, 'p(99)')).padStart(8)}`);
  }
  return {
    stdout: lineas.join('\n') + '\n',
    [`/resultados/${etiqueta}.json`]: JSON.stringify(datos, null, 1),
  };
}

// Umbrales «siempre verdaderos» que solo sirven para que k6 calcule las
// submétricas de la fase de medición; los criterios de éxito se aplican al
// leer el resumen, no abortando la corrida.
export function submetricas(operaciones) {
  const t = {
    'http_req_duration{scenario:medicion}': ['p(95)>=0'],
    'http_reqs{scenario:medicion}': ['count>=0'],
    'checks{scenario:medicion}': ['rate>=0'],
    'errores_5xx{scenario:medicion}': ['count>=0'],
    'timeouts{scenario:medicion}': ['count>=0'],
    'limitados_429{scenario:medicion}': ['count>=0'],
    'rechazos_4xx{scenario:medicion}': ['count>=0'],
    dropped_iterations: ['count>=0'],
  };
  for (const op of operaciones || []) t[`http_req_duration{scenario:medicion,name:${op}}`] = ['p(95)>=0'];
  return t;
}
