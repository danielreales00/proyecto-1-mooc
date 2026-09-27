// Variante separada del escenario 1: ráfaga de inicios de sesión (§4 y §5.4).
//
// Se mide aparte porque cada login es un argon2id de 64 MiB en el Web Server:
// mezclarlo con la actividad académica le atribuiría ese costo a todo. De 1 a
// HASTA inicios por segundo en RAMPA; se detiene sola si el p95 pasa de 5 s.
// La memoria disponible del Web Server se vigila fuera (Cloud Monitoring).
import { check } from 'k6';
import { post, json, iteracion, resumir, CLAVE } from './comun.js';

const HASTA = Number(__ENV.HASTA || 20);
const RAMPA = __ENV.RAMPA || '3m';
const ESTUDIANTES = Number(__ENV.ESTUDIANTES || 600);
const ETIQUETA = __ENV.ETIQUETA || 'e1-rafaga-login';
const OP = 'POST /auth/login';

export const options = {
  scenarios: {
    medicion: {
      executor: 'ramping-arrival-rate',
      startRate: 1,
      timeUnit: '1s',
      preAllocatedVUs: HASTA * 4,
      maxVUs: HASTA * 20,
      stages: [{ target: HASTA, duration: RAMPA }],
      exec: 'entrar',
    },
  },
  thresholds: {
    [`http_req_duration{scenario:medicion,name:${OP}}`]: [{ threshold: 'p(95)<5000', abortOnFail: true, delayAbortEval: '20s' }],
    'http_req_duration{scenario:medicion}': ['p(95)>=0'],
    'http_reqs{scenario:medicion}': ['count>=0'],
    'checks{scenario:medicion}': ['rate>=0'],
    'errores_5xx{scenario:medicion}': ['count>=0'],
    'timeouts{scenario:medicion}': ['count>=0'],
    'limitados_429{scenario:medicion}': ['count>=0'],
    'rechazos_4xx{scenario:medicion}': ['count>=0'],
    dropped_iterations: ['count>=0'],
  },
  summaryTrendStats: ['med', 'p(95)', 'p(99)', 'avg', 'max'],
};

// Cuentas en rotación: a 20/s en 3 minutos cada cuenta entra unas tres veces,
// por debajo del límite de 5 por minuto y cuenta, que no se toca.
export function entrar() {
  const n = (iteracion() % ESTUDIANTES) + 1;
  const email = `estudiante-carga-${String(n).padStart(4, '0')}@carga.local`;
  const r = post('/auth/login', { email, password: CLAVE }, null, OP);
  const d = json(r);
  check(r, { 'sesión emitida': () => r.status === 200 && d && d.token });
}

export function handleSummary(d) {
  return resumir(d, ETIQUETA, [OP]);
}
