# Herramientas HiOffice — contexto del proyecto

Documento de traspaso para continuar este trabajo en un ambiente nuevo. Resume todo lo que se habló, decidió y construyó.

---

## 1. Reglas del proyecto (preferencias del usuario)

- **Idioma:** todo en español (interfaz, mensajes, documentación).
- **Sin repositorio:** este proyecto **no va en ningún repo de GitHub**. Es aparte de los demás proyectos (no tiene relación con `conciliaciones-Magma-Rodelu`).
- **Carpeta de trabajo:** todo se guarda en local, en
  `C:\Users\Rodri\OneDrive\Aplicaciones Claude\Herramientas HiOffice`
- **Equipo:** Windows. PC principal probablemente `DESKTOP-M6VNNKE` (también existe `M0nia-PC`).
- **Vista de tablas:** basta con ver la **estructura** (columnas, orden, totales, filtros), sin datos.
- **Entrega preferida:** un `.exe` que se abre con doble clic, sin `.bat` ni Python, con la configuración pedida al entrar.

### Pendiente de entorno
- La sesión en la nube **no pudo escribir en la carpeta del usuario**: la carpeta no estaba en Remote Control y los PCs no ofrecían carpetas a sesiones en la nube (probablemente hay que actualizar la app de Claude en Windows). Para que Claude escriba directo hay que: actualizar la app → *Configuración → Claude Code → Remote Control* → agregar la carpeta → reiniciar la app. Lo más simple es trabajar desde **Claude Desktop / Claude Code local** abierto en esa carpeta.

---

## 2. Qué es y para qué sirve

Herramienta para trabajar con **dashboards y exportaciones de HiOffice** (ERP cloud de HIOPOS / ICG) y probar el **Bridge** (módulo de importación/exportación por webservice).

Tres funciones (pestañas):

1. **Transformar**: convierte un dashboard/informe (tabla) en exportación CSV y al revés. Incluye vista previa de la estructura de la tabla y un configurador de nombres de columnas.
2. **Validar JSON**: revisa sin conexión que un JSON de dashboard/exportación esté bien armado antes de subirlo a HiOffice.
3. **Probar en HiOffice (Bridge)**: login con CloudLicense, lanzar exportaciones (ver/descargar el archivo) y lanzar importaciones (seguir estado y errores).

---

## 3. Entregables actuales

En `Herramientas_HiOffice.zip` (lo que usa el usuario):

| Archivo | Qué es |
|---|---|
| `Herramientas_HiOffice.exe` | Programa Windows (Go, ~6,7 MB). Sirve la página embebida en `http://127.0.0.1:8765`, abre el navegador, hace de intermediario con el Bridge (evita CORS) y guarda datos en `datos\`. Sin consola (`-H windowsgui`). |
| `Herramientas_HiOffice.html` | La misma herramienta sola (doble clic). Transformar y Validar funcionan sin el exe; el Bridge desde el HTML suelto probablemente lo bloquee CORS. |
| `LEEME.txt` | Instrucciones para el usuario. |

Carpeta `datos\` (la crea el exe junto a sí mismo; si no puede escribir, usa `%APPDATA%\HerramientasHiOffice`):
- `configuracion.json` — email, idioma, servidor de login, `hasPassword`, `autoConnect`
- `nombres_columnas.json` — nombres puestos por el usuario (`{"attr:1660": "Serie", "met:1352": "Uds.V"}`)
- `nombres_aprendidos.json` — nombres deducidos automáticamente de los JSON
- `credencial.bin` — contraseña cifrada con **DPAPI** de Windows (sólo si el usuario elige recordarla)
- `registro.log` — llamadas (método, host, ruta, estado; sin contraseñas ni tokens)

Versiones anteriores ya reemplazadas (el usuario debe borrarlas si quedaron): `Iniciar_herramienta.bat`, `ayudante_hioffice.py`, `Conversor_Dashboard_Exportacion.html`.

---

## 4. Código fuente (en `HiOffice_fuentes.zip`)

```
build.js                         genera dist/Herramientas_HiOffice.html y lo copia a src/goapp/ (node build.js .)
src/page.template.html           la página completa con marcadores __TPL_EXPORT__, __TPL_DASH__, __DICT__
src/plantilla_exportacion.json   valores por defecto de una exportación (sacados de "Exportación de Ventas - Facturas")
src/plantilla_informe.json       valores por defecto de un informe (sacados de "Extracto Tesoreria 3.0")
src/diccionario_nombres.json     nombres de columnas incluidos en el programa
src/goapp/main.go                servidor local, relay, almacenamiento, cierre automático
src/goapp/secret_windows.go      DPAPI (CryptProtectData/CryptUnprotectData) y MessageBox de error
src/goapp/secret_other.go        stub fuera de Windows (no recuerda contraseñas)
src/goapp/main_test.go           pruebas de la lista de servidores permitidos y de las redirecciones
tests/prueba_csv.js              prueba del paso exportación -> CSV (node --test)
```

Compilar: `node build.js .` y luego `go build` (ver CLAUDE.md). El HTML se incrusta en el exe con `//go:embed`, por eso `build.js` copia la página a `src/goapp/` antes de compilar.

Compilación cruzada desde Linux: `GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -H windowsgui"`.

### Arquitectura de la página (un solo HTML, sin dependencias externas)
- `<script id="tplExport">`, `<script id="tplDash">`, `<script id="dict">`: JSON embebidos.
- `<script id="core">`: lógica pura (sin DOM): `isExport`, `analyze`, `convert` (informe→exportación), `convertToReport` (exportación→informe), `previewModel`, `validate`, plantillas. Exporta con `module.exports` si existe (para probar en Node).
- Script de interfaz: pestañas, transformador, vista previa, configurador de nombres, Bridge, modal de configuración.
- `PRISTINE` = HTML original capturado al arrancar, usado por "Descargar el programa con los nombres incluidos" (reemplaza el contenido de `<script id="dict">`).
- Tokens de color en `:root` con modo oscuro (`prefers-color-scheme`).

### Endpoints del exe (sólo `127.0.0.1`, se rechaza otro `Host`)
| Ruta | Uso |
|---|---|
| `GET /` | la página embebida |
| `GET /__helper/ping` | `{ok, app:"herramientas-hioffice", token, version:2, exe:true, dataDir, secrets}`. Sin cabeceras CORS, así otras webs no leen el token. También sirve de latido. |
| `POST /__helper/relay` | requiere `X-Helper-Token`. Cuerpo `{method, url, headers, body \| file:{field,name,type,data(base64)}}` → `{status, contentType, body}`. Sólo GET/POST, sólo `https` a `cloudlicense.icg.eu`, `*.icg.eu`, `*.hiopos.com`. Sólo pasa las cabeceras `x-auth-token`, `content-type`, `accept`. Con archivo arma `multipart/form-data` (campo `file`). |
| `GET/PUT /__helper/store?key=config\|names\|auto` | lee/escribe los JSON de `datos\` (requiere token) |
| `GET/PUT /__helper/secret` | contraseña cifrada con DPAPI (requiere token) |
| `POST /__helper/quit` | cierra el programa |

Otros comportamientos del exe: puerto 8765 (prueba hasta 8774). Si ya hay una instancia, abre el navegador en ella y termina. Se cierra solo si nadie hace ping durante 3 minutos (la página hace ping cada 20 s). Variables para pruebas: `HIO_EXTRA_HOSTS=127.0.0.1:9900` (permite el mock por http), `HIO_NO_BROWSER=1`, `HIO_DATA_DIR`.

---

## 5. Conocimiento sobre los JSON de HiOffice

Archivos de ejemplo que pasó el usuario (descargados desde HiOffice):
- `Exportación_96911_2026-11-02.json` → exportación **"Exportación de Ventas - Facturas"** (dashboard 16033, tipo 6)
- `Dashboard_96911_2026-11-02.json` → informe **"Extracto Tesoreria 3.0"** (dashboard 1615661, tipo 1)
- `Informe_Ventas_x_Art_Exportacion.json` → informe **"Ventas x Art Exportacion"** (dashboard 1638957, tipo 1)

### Estructura general
```
{ "masterData": {...}, "dashboardsData": {...}, ["exportationProfiles": [...], "exportations": [], ...] }
```
- `masterData`: global de la cuenta (métricas personalizadas `metricList`/`metricNameList`, algunos atributos personalizados, `dimensionActionList`). En los dos primeros ejemplos es **idéntico**.
- `dashboardsData`: todo lo del dashboard. Listas relacionadas por ID:
  - `dashboardList` (1 elemento): `idDashboard`, `type` (**1 = dashboard/informe**, **6 = exportación**), `isAnalytics`, `visibleFilter`
  - `dashboardNameList` (nombres por idioma), `dashboardBlockList` (posición/tamaño de cada bloque, `useGlobalFilter`)
  - `blockList`: `idBlock`, `type` (**1 = tabla**, **4 = filtro global**), `guid`, `ignoreDates`, `hideHeader`
  - por bloque: `blockTableList` (config de tabla: `limit`, `showTotals`, `showFilters`, `showRowNumbers`, `isMultiselection`, `bridgeExportationVisibilityType`…), `blockColumnList`, `blockColumnTitleList` (subtítulos por posición), `blockColumnFormulaList`, `blockColumnStyleList`, `blockDataSourceList`, `blockNameList`, `blockStyleConfigurationList`, `blockFilterList`, `blockGlobalFilterList`
  - filtros: `filterList` (fechas `dateValue`/`dateValue2`, escalas) → `filterBlockList` → `filterGroupList` → `filterRowList` → `filterConditionList` (`idAttribute`, `logicOperator`, `value`, `value2`, `askBefore` = se pregunta al ejecutar)
  - exportación: `blockActionList` (`blockActionTypeId` **12002** = acción de exportación; su `guid` es el del bloque), `blockActionNameList`, `blockActionParamValueList` (el parámetro **31** tiene en `valueInteger` el **ID del perfil de exportación**; los parámetros 32–36 se copian tal cual, su significado es desconocido)
- `exportationProfiles` (sólo en exportaciones): `exportationProfileId`, `name`, `doCsv`, `doPdf`, `fieldSeparator` (`;`), `decimalSeparator` (`,`), `dateSeparator`, `fileName`, `showTotals`, `toEmail`/`email`, `toFtp`/`ip`/`port`/`user`/`password`, `toWebservice`…

### Columnas
- Cada columna tiene `idAttribute` **o** `idMetric`, `position`, `idInternal`, `isShown`, `totalType` (1 = con total).
- Las fórmulas de métricas (en `masterData.metricList[].formula`, un JSON en texto) referencian columnas por **`idInternal`** (`{"type":"ColumnFormula","value":"24","displayValue":"Saldo"}`). Ejemplos: `Saldo = Debe-Haber`, `negativo = Saldo<0` (usado en un estilo rojo), `Tipo Documento = IF(Cód. Tipo Doc.=9, "NC Factura Compra", Tipo Documento)`.
- **El JSON NO trae el nombre de los atributos estándar ni de la mayoría de las métricas, sólo su ID.** Fuentes de nombres, por prioridad en la herramienta:
  1. subtítulo del bloque (`blockColumnTitleList`, es lo que se ve en pantalla)
  2. nombre puesto por el usuario (`datos\nombres_columnas.json`)
  3. diccionario incluido en el programa
  4. nombre en el JSON (`attributeNameList`/`metricNameList`), referencias `"type":"Attribute"`/`"type":"Metric"` dentro de cualquier fórmula, o el `displayValue` de una `ColumnFormula`
  5. aprendido de otros JSON
- Las referencias a atributos/métricas dentro de las fórmulas son un **diccionario global** útil, por ejemplo `{"type":"Metric","value":"1357","displayValue":"VENTAS - LÍNEAS:Neto"}`.

### Diccionario de IDs conocido
| ID | Nombre | Origen |
|---|---|---|
| attr 109 | Cód. Artículo | captura de pantalla de "Ventas por artículo" |
| attr 1061 | Establecimiento | captura |
| met 19170 | Artículo | captura (métrica que devuelve texto) |
| met 1352 | Uds.V | captura |
| met 1357 | VENTAS - LÍNEAS: Neto (subtítulo "Total" en Ventas x Art) | fórmula + captura |
| met 1645855 | Familia Descuentos | metricNameList |
| attr 1653 | Cód. Tipo Doc. | fórmula |
| attr 1590 | Tipo Documento | fórmula |
| attr 19164 / 19165 | Debe / Haber | fórmula |
| attr 1865 | Documento: Cód. Almacén | fórmula (es lo que pregunta el filtro global del Extracto) |
| attr 1703 | Documento - Línea: Precio | fórmula |
| attr 1465 | Familia: Familia | fórmula |
| attr 1669 / 1674 | Documento: Tasa Cambio / Total Neto | fórmula |
| attr 1754 / 1756 | Formas pago: Importe Neto / Tasa Cambio | fórmula |
| attr 12249 | Documento - Línea: %Impuestos | fórmula |
| attr 20189 / 20190 | Calendario Laboral: Cód. Motivo Ausencia / Día Ausencia | fórmula |
| attr 21786, 1780, 15223, 15193 | Proveedor, Nombre Fantasia, S/N Pago, Fecha Pago | **subtítulos** del Extracto (no necesariamente el nombre del atributo) |

Sin nombre todavía: en Facturas 1660, 1661, 1944, 1670, 1694, 1697, 2, 3209, 1091, 14032, 1709, 32, 1778; en el Extracto 32, 1781, 12070, 12069, 124, 15204, 19169…; en Ventas x Art, el **atributo 2**, que filtra descuentos ("SCOTIA 25%", "DESCUENTO PERSONAL"…).

**Operadores de filtro** (`logicOperator`): sólo vistos 1, 2, 52, 65, 66, sin significado documentado. 66 con varios valores de descuento parece "excluir".

### Reglas de conversión
**Informe → exportación** (`convert`):
- dashboard `type 6`, `isAnalytics:false`, `visibleFilter:false` (de la plantilla); un solo bloque tabla con `useGlobalFilter:true`
- se copia el bloque tabla elegido: columnas, títulos, fórmulas, estilos, fuente de datos, nombres y config de estilo; la config de tabla se toma de la plantilla (`limit 5`, `limitAdaptative`, `hideButtonsBar`, `bridgeExportationVisibilityType 1`, `isMultiselection`)
- filtro: el del bloque tabla; opcionalmente se agregan las condiciones del filtro global (por defecto no); todo queda en un único grupo
- acción 12002 + parámetros de la plantilla, con el param 31 apuntando al nuevo perfil; perfil CSV nuevo con nombre, archivo, separadores y totales elegidos
- se descarta el bloque de filtro global (tipo 4)
- `ignoreDates` se hereda del bloque (el Extracto lo tiene en `true` porque necesita todo el historial para el saldo)

**Exportación → informe** (`convertToReport`):
- `type 1`, `isAnalytics:true`, `visibleFilter:true`
- opcional (por defecto sí) un bloque de filtro global tipo 4 arriba (`posY 0`, alto 1) con `blockGlobalFilterList` y un filtro sólo de fechas; la tabla debajo (`posY 1`, alto 20) con `useGlobalFilter:true`
- config de tabla de la plantilla del Extracto (`limit 20`, totales, filtros, nº de fila)
- se quitan la acción 12002 y el perfil, avisándolo
- ida y vuelta probada: Extracto → exportación → informe devuelve la misma tabla (columnas, títulos, fórmulas, estilos, datasource, condiciones)

**IDs nuevos:** propone IDs libres dentro del JSON (no puede saber cuáles existen en la cuenta de HiOffice). Si coincide con el ID original avisa que podría sobrescribirlo. Se generan GUIDs nuevos.

### Validación (`validate`)
Errores y avisos que revisa:
- un dashboard
- bloques colocados en `dashboardBlockList`
- referencias por `idBlock` en todas las listas
- GUIDs duplicados
- tablas con columnas visibles, datasource, posiciones e `idInternal` únicos
- fórmulas existentes en `masterData` y sus referencias `ColumnFormula`
- estilos apuntando a columnas y métricas válidas
- cadena de filtros completa y filtros sin usar
- fechas invertidas
- filtro global con su configuración
- en exportaciones: acción 12002 en una tabla, parámetro 31 → perfil existente; el perfil genera CSV/PDF, con separadores presentes y distintos, nombre de archivo, email/FTP completos; contraseña FTP guardada en el perfil (aviso al compartir); tabla sin filtro global

Los 3 JSON del usuario pasan sin errores.

---

## 6. Bridge de HiOffice (según la documentación del usuario)

Documentos: **"BRIDGE HIOFFICE" v1.03** (10/01/2024) y **"Máster Advanced Engineering – UF8 Bridge – Mappings import/export"** (HIOPOS). En la UF8, las estructuras de archivo del anexo son imágenes (no hay texto).

### Login (exportación e importación)
`GET https://cloudlicense.icg.eu/services/cloud/getCustomerWithAuthToken?email=…&password=…&isoLanguage=ES|EN|CA|DE|EL|PT` (EN por defecto)
Respuesta XML: `<response><customerWithAuthTokenResponse><address>cloudclientbeta05.hiopos.com</address><authToken>…</authToken><customerId>…</customerId><port>443</port><secure>true</secure></customerWithAuthTokenResponse></response>`
→ `base = https://{address}` (con `:port` si no es el puerto por defecto). Todas las llamadas siguientes llevan la cabecera `x-auth-token`.

### Exportación
- `POST {base}/ErpCloud/exportation/launch` con cuerpo `{"exportationId": 132, "startDate":"yyyy-mm-dd", "endDate":"yyyy-mm-dd", "filters":[]}`. `exportationId` es **Integer para exportación por tabla** y **String (GUID) para exportación por dashboard**.
  Respuesta: `"exportedDocs": [{"name","data"(base64),"type"}]`, con tipo 0 = sin tipo, 1 = csv, 2 = pdf, 3 = txt, 4 = json, 5 = xml.
- `POST {base}/ErpCloud/exportation/getExportationDashboardFilters/{guid}` → `[{"attributeId":1658,"arithmeticOperator":"EQUAL","type":"Integer","value":"","value2":""}]` (filtros del filtro global, para mandarlos en `filters`).
- `GET {base}/ErpCloud/session/logout`

### Importación
- `POST {base}/bridge-back/api/import/{id_importation}/launch` (sin archivo: FTP o configurado)
- `POST …/launchWithFile` (multipart, campo `file`)
- `POST …/launchWithBody` (`application/json` o `application/xml`)
- Todas devuelven `[{"configurationName","configurationUUID","importUUID"}]`: puede haber más de una si hay importaciones encadenadas.
- `GET {base}/bridge-back/api/import/{importUUID}/status` → `{statusId: 1 en ejecución | 2 con errores | 3 correcta, status, recordsInserted, recordsUpdated, filesToProcess, filesProcessed, errors, filesRecordsProcessed, startTimestamp, endTimestamp}`
- `GET …/{importUUID}/errors` → `{"errors":[{"errorCode","errorText","errorTimestamp"}]}`

UUID de importaciones por defecto (anexo UF8):

| Importación | UUID |
|---|---|
| Artículos | e185589c-54db-4563-b839-cbfc30606e43 |
| Artículos con formatos | fc468adc-e190-49cf-8c3c-855a235c7ced |
| Coste | 2fe91503-e80b-4039-a8aa-1010066baab1 |
| Stock | 751af2fc-5b87-45d1-8bf7-fa44b3308b4a |
| Tallas y colores / Precios compra (el PDF repite el mismo UUID para las dos) | 4df8945d-3376-43db-8d8c-2d2dbb252e18 |
| Clientes | 3d65832c-bb35-44eb-b621-6b4bb11e79de |
| Vendedores | 72cf79c1-6af2-4e31-9f8a-bbc92dfb9c8a |
| Proveedores | 32bfd2e4-b1fe-4881-a441-d71b4b52aba1 |
| Ref. proveedor | d2f84b0e-4365-4cee-88d1-ec94706d1124 |
| Tarjetas | 5a598351-9875-441a-a89a-e0b10eee15e6 |
| Pedidos venta | 16a6e70e-87ee-48a6-b91a-9752b2740018 |
| Albaranes venta | 16a6e70e-87ee-48a6-b91a-9752b2740019 |
| Fact. simplificadas venta | 16a6e70e-87ee-48a6-b91a-9752b2740017 |
| Facturas venta | 16a6e70e-87ee-48a6-b91a-9752b2740015 |
| Pedidos compra | fc9ffb72-36f1-4f96-ab14-6a62dd155cad |
| Albaranes compra | fc9ffb72-36f1-4f96-ab14-6a62dd155caf |
| Facturas compra | fc9ffb72-36f1-4f96-ab14-6a62dd155cae |

### Cómo lo usa la herramienta
- **Exportación:** valida las fechas y el ID numérico si es por tabla, lanza la exportación, decodifica cada documento y da un veredicto ("funciona", vacío o con problemas). CSV: detecta el separador, muestra hasta 100 filas y avisa de filas con distinto número de columnas. JSON/XML: verifica que sea válido. Permite descargar el archivo.
- **Importación:** exige marcar la confirmación (escribe datos reales). Consulta el estado cada 2 s hasta que termina (por defecto como máximo 10 minutos), y si hay errores trae la lista.
- **Registro de llamadas** sin contraseña ni token. La contraseña se borra del campo después del login.

---

## 7. Cómo se probó

- Transformaciones y validaciones con los 3 JSON reales: OK, incluida la ida y vuelta.
- Bridge contra un Bridge simulado (`mock_bridge.py`, ya borrado del repo), que implementaba los endpoints según la documentación (login correcto/incorrecto, export CSV+JSON, export con error 500, filtros de dashboard, importación con archivo/cuerpo terminando bien o con error): todo OK.
- El ayudante rechaza las peticiones sin token, otros hosts, `http` hacia icg.eu y una cabecera `Host` ajena.
- El exe se probó compilado para Linux: configuración del primer uso, guardado en archivos, nombres leídos desde archivo, login, exportación, botón de cierre, apagado del proceso.
- **No probado:** el `.exe` en Windows real (Wine falló en el contenedor), el cifrado DPAPI ("recordar contraseña") y **cualquier llamada al HiOffice real** (sin acceso de red ni credenciales desde la nube).

---

## 8. Pendientes y dudas abiertas

1. Probar el `.exe` en Windows: aviso de SmartScreen, "recordar contraseña" y conexión automática.
2. Verificar contra HiOffice real:
   - que la respuesta del login tenga el XML documentado;
   - si CORS bloquea el HTML suelto (se espera que sí; por eso existe el exe);
   - qué número es el `exportationId` de una exportación por tabla (¿el `exportationProfileId`, p. ej. 20, u otro?);
   - los nombres reales de `arithmeticOperator` (la lista de sugerencias `NOT_EQUAL`, `GREATER`… es una suposición; sólo `EQUAL` está documentado);
   - si `launch` devuelve un objeto con `exportedDocs` o directamente la lista (se aceptan ambos).
3. Completar el diccionario de nombres de columnas (desde capturas de pantalla de HiOffice) y meterlo en `src/diccionario_nombres.json`.
4. Significado de los `logicOperator` de los filtros (1, 2, 52, 65, 66) y de los parámetros 32–36 de la acción 12002.
5. Alineación de métricas de texto (p. ej. "Artículo"), que el JSON no indica.
6. Ideas posibles:
   - firmar el exe (evita SmartScreen);
   - ícono propio;
   - convertir varios bloques a la vez;
   - exportación por dashboard (JSON/XML) con mapeo;
   - comparar el resultado de una exportación con la vista previa esperada.

## 9. Restos de la sesión anterior

- En GitHub quedó la rama `claude/gallant-hawking-5haxcl` en `conciliaciones-Magma-Rodelu` con un commit del primer JSON convertido; la sesión no pudo borrarla (403). Si sigue ahí, borrarla a mano desde *Branches*. Este proyecto no debe tocar ese repo.
