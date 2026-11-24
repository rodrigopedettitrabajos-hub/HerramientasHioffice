#!/usr/bin/env python3
"""Genera dist/Herramientas_HiOffice.html y compila el .exe (requiere Go 1.22+).
Uso: python build.py"""
import json, os, subprocess, shutil
H = os.path.dirname(os.path.abspath(__file__)); src = os.path.join(H, 'src'); dist = os.path.join(H, 'dist')
os.makedirs(dist, exist_ok=True)
r = lambda f: open(os.path.join(src, f), encoding='utf-8').read().strip().replace('</', '<\\/')
d = json.dumps(json.load(open(os.path.join(src, 'diccionario_nombres.json'), encoding='utf-8')), ensure_ascii=False, indent=1).replace('<', '\\u003c')
html = open(os.path.join(src, 'page.template.html'), encoding='utf-8').read()
html = html.replace('__TPL_EXPORT__', r('plantilla_exportacion.json')).replace('__TPL_DASH__', r('plantilla_informe.json')).replace('__DICT__', d)
for p in (os.path.join(dist, 'Herramientas_HiOffice.html'), os.path.join(src, 'goapp', 'Herramientas_HiOffice.html')):
    open(p, 'w', encoding='utf-8').write(html)
if shutil.which('go'):
    env = dict(os.environ, GOOS='windows', GOARCH='amd64', CGO_ENABLED='0')
    subprocess.check_call(['go', 'build', '-trimpath', '-ldflags', '-s -w -H windowsgui', '-o', os.path.join(dist, 'Herramientas_HiOffice.exe'), '.'], cwd=os.path.join(src, 'goapp'), env=env)
    print('OK: dist/Herramientas_HiOffice.html y dist/Herramientas_HiOffice.exe')
else:
    print('OK: dist/Herramientas_HiOffice.html (Go no instalado: no se compiló el .exe)')
