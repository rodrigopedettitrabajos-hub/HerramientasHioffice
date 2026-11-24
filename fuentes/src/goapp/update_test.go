package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestReleaseYActualizacion(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/latest" {
			fmt.Fprintf(w, `{"tag_name":"v2","assets":[{"name":"otro.zip","browser_download_url":"x"},{"name":"Herramientas_HiOffice.exe","browser_download_url":"%s/exe"}]}`, srv.URL)
			return
		}
		w.Write([]byte("MZnuevo"))
	}))
	defer srv.Close()
	releaseAPI = srv.URL + "/latest"
	tag, url, err := latestRelease()
	if err != nil || tag != "v2" || url != srv.URL+"/exe" {
		t.Fatal(tag, url, err)
	}
	dir := t.TempDir()
	exePath = filepath.Join(dir, "Herramientas_HiOffice.exe")
	os.WriteFile(exePath, []byte("MZviejo"), 0o755)
	nueva.tag, nueva.url = tag, url
	if err := applyRelease(); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(exePath); string(b) != "MZnuevo" {
		t.Fatal(string(b))
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "Herramientas_HiOffice.old.exe")); string(b) != "MZviejo" {
		t.Fatal(string(b))
	}
	// una descarga que no es un .exe no toca el programa
	nueva.url = srv.URL + "/latest"
	if err := applyRelease(); err == nil {
		t.Fatal("esperaba error")
	}
	if b, _ := os.ReadFile(exePath); string(b) != "MZnuevo" {
		t.Fatal(string(b))
	}
}
