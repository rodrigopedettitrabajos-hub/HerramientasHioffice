// Descarga de CFE emitidos desde Migrate/Invoicy (webservice SOAP WS_DescargaEmitidos).
package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"errors"
	"io"
	"log"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var migrateURL = "https://appuy.migrate.info/InvoiCy/aws_descargaemitidos.aspx" // variable para las pruebas

func xmlEsc(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}

type migrateCFE struct {
	Tipo   string `xml:"TipoCFE" json:"tipo"`
	Serie  string `xml:"CFESerie" json:"serie"`
	Numero string `xml:"CFENumero" json:"numero"`
	XML    string `xml:"CFEXML" json:"xml"`
}

type migrateRet struct {
	N      int          `xml:"Encabezado>NroLinRetorno" json:"n"`
	Total  int          `xml:"Encabezado>NroLinTotalRetorno" json:"total"`
	Pagina int          `xml:"Encabezado>Pagina" json:"pagina"`
	MsgCod string       `xml:"Encabezado>MsgCod" json:"msgCod"`
	MsgDsc string       `xml:"Encabezado>MsgDsc" json:"msgDsc"`
	CFE    []migrateCFE `xml:"ListaCFE>CFE" json:"cfe"`
}

var reCodigo = regexp.MustCompile(`^\d{1,10}$`)

// Pide una página de CFE emitidos entre dos fechas (inclusión en Invoicy) y devuelve los XML ya decodificados.
func handleMigrate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "método no permitido"})
		return
	}
	var q struct {
		Codigo, Clave, Socio, Desde, Hasta string
		Pagina                             int
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&q); err != nil {
		writeJSON(w, 200, map[string]string{"error": "petición inválida"})
		return
	}
	q.Codigo, q.Clave, q.Socio = strings.TrimSpace(q.Codigo), strings.TrimSpace(q.Clave), strings.TrimSpace(q.Socio)
	_, e1 := time.Parse("2006-01-02", q.Desde)
	_, e2 := time.Parse("2006-01-02", q.Hasta)
	switch {
	case !reCodigo.MatchString(q.Codigo):
		writeJSON(w, 200, map[string]string{"error": "El código de empresa tiene que ser un número."})
		return
	case q.Clave == "" || len(q.Clave) > 64 || q.Socio == "" || len(q.Socio) > 64:
		writeJSON(w, 200, map[string]string{"error": "Faltan la clave de comunicación o la clave del socio."})
		return
	case e1 != nil || e2 != nil || q.Hasta < q.Desde:
		writeJSON(w, 200, map[string]string{"error": "Rango de fechas inválido."})
		return
	}
	if q.Pagina < 1 {
		q.Pagina = 1
	}
	// TipoRetornoXML 1 = XML de envío del CFE; CFERepImpresa 4 = sin PDF (pesa mucho y no se usa).
	inner := "<DescargaEmitidos><Encabezado><EmpCodigo>" + q.Codigo + "</EmpCodigo><EmpPK>" + xmlEsc(q.Socio) +
		"</EmpPK><EmpCK>" + xmlEsc(q.Clave) + "</EmpCK><Pagina>" + strconv.Itoa(q.Pagina) + "</Pagina></Encabezado><Filtros>" +
		"<FechaHoraInclusionIni>" + q.Desde + "T00:00:00</FechaHoraInclusionIni><FechaHoraInclusionFin>" + q.Hasta +
		"T23:59:59</FechaHoraInclusionFin><TipoRetornoXML>1</TipoRetornoXML><CFERepImpresa>4</CFERepImpresa></Filtros></DescargaEmitidos>"
	env := `<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:gx="Gx"><soapenv:Header/><soapenv:Body>` +
		`<gx:WS_DescargaEmitidos.Execute><gx:Xmldescarga>` + xmlEsc(inner) + `</gx:Xmldescarga></gx:WS_DescargaEmitidos.Execute></soapenv:Body></soapenv:Envelope>`
	req, _ := http.NewRequest("POST", migrateURL, strings.NewReader(env))
	req.Header.Set("Content-Type", "text/xml; charset=utf-8")
	req.Header.Set("SOAPAction", `"Gxaction/AWS_DESCARGAEMITIDOS.Execute"`)
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": "No se pudo conectar con Migrate: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, maxBody))
	log.Printf("migrate empresa %s %s..%s pág %d -> %d", q.Codigo, q.Desde, q.Hasta, q.Pagina, resp.StatusCode)
	ret, err := parseMigrate(raw)
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": "Respuesta de Migrate no reconocida (HTTP " + strconv.Itoa(resp.StatusCode) + "): " + err.Error()})
		return
	}
	writeJSON(w, 200, ret)
}

// Saca el texto de <Xmlretorno> (o el faultstring) del sobre SOAP y lo lee.
func parseMigrate(raw []byte) (*migrateRet, error) {
	var env struct {
		Ret   string `xml:"Body>ExecuteResponse>Xmlretorno"`
		Fault string `xml:"Body>Fault>faultstring"`
	}
	// El elemento se llama "WS_DescargaEmitidos.ExecuteResponse": se acorta para que encaje en la ruta.
	raw = bytes.ReplaceAll(raw, []byte("WS_DescargaEmitidos.ExecuteResponse"), []byte("ExecuteResponse"))
	if err := xml.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if env.Fault != "" {
		return nil, errors.New(env.Fault)
	}
	inner := strings.TrimSpace(env.Ret)
	if i := strings.Index(inner, "?>"); strings.HasPrefix(inner, "<?xml") && i > 0 {
		inner = inner[i+2:] // la declaración puede decir otra codificación; el texto ya es UTF-8
	}
	var ret migrateRet
	if err := xml.Unmarshal([]byte(inner), &ret); err != nil {
		return nil, err
	}
	for i, c := range ret.CFE {
		b, err := base64.StdEncoding.DecodeString(strings.Join(strings.Fields(c.XML), ""))
		if err != nil {
			ret.CFE[i].XML = ""
			continue
		}
		if !utf8.Valid(b) { // XML en ISO-8859-1: se pasa a UTF-8
			r := make([]rune, len(b))
			for j, c := range b {
				r[j] = rune(c)
			}
			b = []byte(string(r))
		}
		ret.CFE[i].XML = string(b)
	}
	return &ret, nil
}
