package main

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestParseMigrate(t *testing.T) {
	cfe := base64.StdEncoding.EncodeToString([]byte("<CFE><Nombre>Ca\xf1a</Nombre></CFE>")) // ISO-8859-1
	inner := `<?xml version="1.0" encoding="utf-8"?><DescargaEmitidosRetorno><Encabezado><NroLinRetorno>1</NroLinRetorno><NroLinTotalRetorno>3</NroLinTotalRetorno><Pagina>1</Pagina><MsgCod>100</MsgCod><MsgDsc>OK</MsgDsc></Encabezado><ListaCFE><CFE><TipoCFE>101</TipoCFE><CFESerie>A</CFESerie><CFENumero>7</CFENumero><CFEXML>` + cfe + `</CFEXML></CFE></ListaCFE></DescargaEmitidosRetorno>`
	env := `<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/"><SOAP-ENV:Body><WS_DescargaEmitidos.ExecuteResponse xmlns="Gx"><Xmlretorno><![CDATA[` + inner + `]]></Xmlretorno></WS_DescargaEmitidos.ExecuteResponse></SOAP-ENV:Body></SOAP-ENV:Envelope>`
	r, err := parseMigrate([]byte(env))
	if err != nil {
		t.Fatal(err)
	}
	if r.Total != 3 || len(r.CFE) != 1 || r.CFE[0].Numero != "7" || !strings.Contains(r.CFE[0].XML, "Caña") {
		t.Fatalf("%+v", r)
	}
	// también con el XML escapado en vez de CDATA, y un Fault
	env2 := strings.Replace(env, "<![CDATA["+inner+"]]>", xmlEsc(inner), 1)
	if r, err := parseMigrate([]byte(env2)); err != nil || r.Total != 3 {
		t.Fatal(err)
	}
	fault := `<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://schemas.xmlsoap.org/soap/envelope/"><SOAP-ENV:Body><SOAP-ENV:Fault><faultcode>x</faultcode><faultstring>Clave inválida</faultstring></SOAP-ENV:Fault></SOAP-ENV:Body></SOAP-ENV:Envelope>`
	if _, err := parseMigrate([]byte(fault)); err == nil || err.Error() != "Clave inválida" {
		t.Fatal(err)
	}
}
