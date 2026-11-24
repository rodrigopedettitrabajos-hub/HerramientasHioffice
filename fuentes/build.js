// Equivalente de build.py (parte HTML). Uso: node build.js <carpeta fuentes>
const fs = require('fs'), path = require('path');
const H = process.argv[2], src = path.join(H, 'src'), dist = path.join(H, 'dist');
fs.mkdirSync(dist, { recursive: true });
const r = f => fs.readFileSync(path.join(src, f), 'utf8').trim().split('</').join('<\\/');
const d = JSON.stringify(JSON.parse(fs.readFileSync(path.join(src, 'diccionario_nombres.json'), 'utf8')), null, 1).split('<').join('\\u003c');
let html = fs.readFileSync(path.join(src, 'page.template.html'), 'utf8');
html = html.split('__TPL_EXPORT__').join(r('plantilla_exportacion.json'))
  .split('__TPL_DASH__').join(r('plantilla_informe.json'))
  .split('__DICT__').join(d);
// Herramientas incluidas (cada una se muestra en su pestaña dentro de un iframe)
const tool = f => JSON.stringify(fs.readFileSync(path.join(src, 'herramientas', f), 'utf8')).split('<').join('\\u003c');
html = html.split('__TOOL_CFE__').join(tool('lector_cfe.html'))
  .split('__TOOL_LIQ__').join(tool('unificador_liquidacion.html'));
for (const p of [path.join(dist, 'Herramientas_HiOffice.html'), path.join(src, 'goapp', 'Herramientas_HiOffice.html')])
  fs.writeFileSync(p, html, 'utf8');
console.log('HTML ok', html.length);
