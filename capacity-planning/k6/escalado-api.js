// Ciclo de escalado de la API (Sesión C, C3). Lecturas anónimas del catálogo a
// tasa constante contra el balanceador; cada respuesta suma en un contador
// etiquetado con la X-Instancia que la atendió.
//
//   k6 run -e BASE_URL=https://<host> -e TASA=50 -e DURACION=30m escalado-api.js
import http from 'k6/http';
import { Counter } from 'k6/metrics';
import { elegir } from './comun.js';

const TASA = Number(__ENV.TASA || 50);
const DURACION = __ENV.DURACION || '30m';
const BASE = (__ENV.BASE_URL || 'http://localhost:8090') + '/api/v1';

const porInstancia = new Counter('respuestas_por_instancia');
const fallos = new Counter('fallos');

export const options = {
  scenarios: {
    carga: {
      executor: 'constant-arrival-rate',
      rate: TASA, timeUnit: '1s', duration: DURACION,
      preAllocatedVUs: TASA * 2, maxVUs: TASA * 6,
    },
  },
  summaryTrendStats: ['med', 'p(95)', 'p(99)', 'max'],
};

const RUTAS = ['/catalog/courses', '/catalog/courses?limit=5', '/catalog/courses/fundamentos-de-cloud'];

export default function pedirCatalogo() {
  const r = http.get(BASE + elegir(RUTAS), { timeout: '20s' });
  porInstancia.add(1, { instancia: r.headers['X-Instancia'] || 'ninguna', estado: String(r.status) });
  if (r.status >= 500 || r.status === 0) {
    fallos.add(1, { estado: String(r.status) });
    console.log(`${new Date().toISOString()} fallo ${r.status} ${r.headers['X-Instancia'] || '-'}`);
  }
}

// El reparto por instancia y por minuto sale del CSV (--out csv), no del
// resumen: k6 solo resume submétricas con umbral, y los nombres de las
// instancias no se conocen antes.
