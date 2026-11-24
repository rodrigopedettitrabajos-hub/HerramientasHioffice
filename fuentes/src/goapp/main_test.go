package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// El reenvío sólo puede llamar a servidores de HiOffice por https.
func TestHostAllowed(t *testing.T) {
	casos := map[string]bool{
		"https://cloudlicense.icg.eu/services/cloud/x": true,
		"https://erp12.icg.eu/ErpCloud/x":              true,
		"https://algo.hiopos.com/bridge-back/x":        true,
		"http://erp12.icg.eu/ErpCloud/x":               false, // sin https
		"https://icg.eu.evil.com/x":                    false,
		"https://evil.com/?h=.icg.eu":                  false,
		"https://evil.com/.icg.eu":                     false,
		"https://evilicg.eu/x":                         false,
		"ftp://erp12.icg.eu/x":                         false,
		"no es una url":                                false,
	}
	for u, quiero := range casos {
		if got := hostAllowed(u); got != quiero {
			t.Errorf("hostAllowed(%q) = %v, quiero %v", u, got, quiero)
		}
	}
}

// Una redirección a un destino no permitido se corta (no viaja el token).
func TestRelayNoSigueRedireccionesFuera(t *testing.T) {
	llegó := false
	fuera := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { llegó = true }))
	defer fuera.Close()
	bridge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, fuera.URL+"/robar", http.StatusFound)
	}))
	defer bridge.Close()
	// Sólo se permite el "Bridge" de prueba, no el destino de la redirección.
	extraHost = map[string]bool{strings.TrimPrefix(bridge.URL, "http://"): true}
	defer func() { extraHost = map[string]bool{} }()

	req := httptest.NewRequest("POST", "http://127.0.0.1/__helper/relay", strings.NewReader(`{"method":"GET","url":"`+bridge.URL+`/x","headers":{"x-auth-token":"secreto"}}`))
	rec := httptest.NewRecorder()
	handleRelay(rec, req)
	if llegó {
		t.Fatal("la redirección llegó al destino no permitido")
	}
	if !strings.Contains(rec.Body.String(), "no permitido") {
		t.Fatalf("esperaba un error de destino no permitido, llegó: %s", rec.Body.String())
	}
}
