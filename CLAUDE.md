# Herramientas HiOffice

Programa de escritorio (Windows) para trabajar con HiOffice (ERP en la nube de HIOPOS/ICG): transformar dashboards ⇄ exportaciones, validar JSON, pasar exportaciones a CSV, probar exportaciones e importaciones reales contra el Bridge, y herramientas sueltas (Lector de CFE, Unificador de liquidación).

Detalle completo (formatos JSON de HiOffice, diccionario de IDs, Bridge, pendientes): `fuentes/CONTEXTO_Herramientas_HiOffice.md`.

## Reglas (del usuario)

- Todo en **español** (código visible, mensajes, textos de la app y respuestas).
- **Repositorio:** `github.com/rodrigopedettitrabajos-hub/HerramientasHioffice` (código fuente). No se suben `datos/`, los `.exe` ni la salida de compilación (ver `.gitignore`). Nada que ver con `conciliaciones-Magma-Rodelu`: no tocarlo.
- Todo se guarda en esta carpeta.
- Entrega: un **`.exe` de doble clic** (Go). Nada de `.bat` ni de Python para el usuario.
- Si hay que guardar datos: **SQLite** (`modernc.org/sqlite`, Go puro, sin CGO).
- No empezar cambios grandes sin el visto bueno del usuario.

## Estructura

```
Herramientas_HiOffice.exe     programa instalado (lo que abre el usuario)
Herramientas_HiOffice.html    la misma página, para abrir suelta (sin Bridge)
LEEME.txt                     copia de fuentes/src/LEEME_usuario.txt
datos/                        configuracion.json, credencial.bin (DPAPI), registro.log, pruebas.db
fuentes/
  build.js                    arma el HTML
  dist/                       salida de la compilación
  tests/                      prueba_csv.js (node --test, sin dependencias)
  src/
    page.template.html        TODA la interfaz (HTML + CSS + JS, sin dependencias externas)
    plantilla_exportacion.json, plantilla_informe.json, diccionario_nombres.json
    herramientas/             lector_cfe.html, unificador_liquidacion.html (se muestran en iframe)
    LEEME_usuario.txt
    goapp/                    main.go (servidor local + Bridge proxy), db.go (SQLite), secret_*.go, main_test.go
```

## Compilar

Go está en `C:\Program Files\Go\bin\go.exe` (no está en el PATH).

```bash
cd fuentes && node build.js .
cd src/goapp && CGO_ENABLED=0 "/c/Program Files/Go/bin/go.exe" build -trimpath -ldflags "-s -w -H windowsgui" -o ../../dist/Herramientas_HiOffice.exe .
```

`build.js` mete en el HTML: `__TPL_EXPORT__`, `__TPL_DASH__`, `__DICT__`, `__TOOL_CFE__`, `__TOOL_LIQ__`. Escribe el resultado en `dist/` y en `src/goapp/` (lo embebe `//go:embed`). Ese HTML no está en el repo: en un clon nuevo hay que correr `node build.js .` antes de `go build`.
Antes de compilar, comprobar la sintaxis de los `<script>` sin `type` con `new Function(...)` en Node.

## Probar

- Pruebas automáticas: `cd fuentes && node --test tests/prueba_csv.js` y `cd fuentes/src/goapp && go test ./...`.
- Exe de prueba con `HIO_PORT=8790`, `HIO_NO_BROWSER=1`, `HIO_DATA_DIR=<carpeta temporal>` para no pisar el del usuario (puerto 8765).
- Abrirlo en el navegador integrado (`http://127.0.0.1:8790/`). Al terminar, cerrar el proceso de `fuentes\dist`.

## Publicar y actualizar

Cada merge a `main` que toque `fuentes/` corre `.github/workflows/publicar.yml`: prueba, compila con `-X main.version=1.NN` (número de corrida del workflow, pedido de Rodrigo) y publica el `.exe` como versión en GitHub. Las novedades de cada versión son el mensaje del commit de `main` (título del PR y su lista), sin las firmas: escribirlo pensando en el usuario. En el programa, **★ Novedades** las lista y se abre sola la primera vez después de actualizar. El programa publicado revisa al abrir (y cada 6 h) la última versión; si hay otra y el programa se acaba de abrir (primer minuto), se instala sola; si aparece más tarde, queda el aviso y **↻ Reiniciar y actualizar** la baja. En los dos casos deja la actual como `.old.exe` y reabre. Las compilaciones locales (versión `dev`) no se actualizan solas.
Migrate ("Traer desde Migrate"): el usuario escribe una vez Código de integración (EmpCodigo), Clave de acceso y Clave del partner (EmpPK); quedan cifradas con DPAPI. La Clave del partner no va en el exe (el repo es público). La clave de comunicación (EmpCK) se calcula en cada llamada: MD5 hex de clave de acceso + el XML enviado después del Encabezado (ejemplo oficial: github.com/migrate-company/integracao-InvoiCy-UY-csharp).

## Instalar una versión nueva a mano

1. Si el exe de la carpeta principal está abierto: borrar `Herramientas_HiOffice.old.exe` si existe y renombrar el exe actual a `.old.exe`.
2. Copiar `dist/Herramientas_HiOffice.exe` y `.html` a la carpeta principal.
3. El usuario pulsa **↻ Reiniciar y actualizar**: el programa se reabre con la versión nueva y borra el `.old.exe`.

## Notas técnicas

- PowerShell: `$f` y `$F` son la misma variable. Los `.ps1` con tildes y sin BOM fallan en PS 5.1.
- El log se abre en modo añadir después de tomar el puerto, solo a archivo (con `windowsgui` no hay stderr).
- Endpoints `/__helper/*` exigen el token (incluido `quit`).
- La app arranca siempre en la pestaña **Inicio** (tarjetas); `showTab(nombre)` cambia de pestaña.

## Conocimiento de HiOffice aprendido en uso

- La exportación de tablas devuelve `[{"data":[{"0":..}],"columnTypes":[..],"columnNames":[..],"columnKeys":[..],"index":0}]`. La app lo detecta y ofrece "Pasar a CSV".
- Las importaciones CSV emparejan columnas **por nombre de encabezado**, no por posición.
- Importación de stock por defecto (`751af2fc-5b87-45d1-8bf7-fa44b3308b4a`): busca el artículo **por código de barras**. Funciona si el artículo lo tiene. El almacén va por su **código** (ej. `2`).
- La importación personalizada "1 - Importacion Stocks" (`b127cff8-…`) falla siempre con `Table 'DB…Temp__1_1_-_Impo__…' doesn't exist`. Hipótesis: el guion o el nombre rompe la tabla temporal. Se recomendó duplicar la de por defecto con nombre simple y buscar por Cód. Artículo.
