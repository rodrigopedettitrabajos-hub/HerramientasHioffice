// Prueba del paso "exportación de HiOffice -> CSV" (lo que después se importa en HiOffice).
// Uso (desde la carpeta fuentes, sin dependencias): node --test tests/prueba_csv.js
const test = require('node:test'), assert = require('node:assert'), fs = require('fs'), path = require('path');

// Toma las funciones tal cual están en la página, sin DOM.
const src = fs.readFileSync(path.join(__dirname, '..', 'src', 'page.template.html'), 'utf8');
const pick = name => { const i = src.indexOf('function ' + name + '('); assert.ok(i >= 0, 'falta ' + name); let d = 0, j = src.indexOf('{', i); for (; j < src.length; j++) { if (src[j] === '{') d++; else if (src[j] === '}' && !--d) break; } return src.slice(i, j + 1); };
const numTypes = src.match(/const NUM_TYPES = .*;/)[0];
const { hioTables, buildCsv, parseCsv } = new Function(numTypes + ['hioTables', 'csvNum', 'csvField', 'buildCsv', 'parseCsv'].map(pick).join('\n') + 'return { hioTables, buildCsv, parseCsv };')();

const exportacion = [{ data: [{ 0: '3', 1: 'Agua "con gas"', 2: '12.5' }, { 0: '7', 1: 'Pan; integral', 2: '3' }],
  columnTypes: ['STRING', 'STRING', 'FLOAT'], columnNames: ['Cód. Artículo', 'Descripción', 'Stock'], columnKeys: ['0', '1', '2'], index: 0 }];

test('pasa la exportación a CSV con ; y coma decimal', () => {
  const [t] = hioTables(exportacion);
  const cols = t.cols.map(c => ({ ...c, on: true, out: c.name }));
  const csv = buildCsv(t, cols, { sep: ';', dec: ',', header: true });
  assert.strictEqual(csv, 'Cód. Artículo;Descripción;Stock\r\n3;"Agua ""con gas""";12,5\r\n7;"Pan; integral";3\r\n');
});

test('respeta columnas quitadas, renombradas y reordenadas', () => {
  const [t] = hioTables(exportacion);
  const cols = [{ ...t.cols[2], on: true, out: 'Cantidad' }, { ...t.cols[1], on: false, out: 'x' }, { ...t.cols[0], on: true, out: 'Codigo' }];
  assert.strictEqual(buildCsv(t, cols, { sep: ',', dec: '.', header: true }), 'Cantidad,Codigo\r\n12.5,3\r\n3,7\r\n');
});

test('el CSV generado se vuelve a leer igual', () => {
  const [t] = hioTables(exportacion);
  const csv = buildCsv(t, t.cols.map(c => ({ ...c, on: true, out: c.name })), { sep: ';', dec: '.', header: true });
  const r = parseCsv(csv);
  assert.strictEqual(r.sep, ';');
  assert.deepStrictEqual(r.rows.slice(1), [['3', 'Agua "con gas"', '12.5'], ['7', 'Pan; integral', '3']]);
});

test('no confunde otro JSON con una exportación', () => {
  assert.strictEqual(hioTables({ dashboardsData: {} }), null);
  assert.strictEqual(hioTables([]), null);
});
