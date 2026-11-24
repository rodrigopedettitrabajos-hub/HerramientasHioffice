// Actualización desde las versiones publicadas en GitHub (las arma .github/workflows/publicar.yml).
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var version = "dev" // el workflow pone la etiqueta de la versión con -ldflags "-X main.version=..."

// Cliente propio: las descargas de GitHub redirigen a otro dominio, y aquí no viajan credenciales
// (el cliente del relay sólo sigue redirecciones a hosts permitidos).
var ghClient = &http.Client{Timeout: 5 * time.Minute, Transport: client.Transport}

var releaseAPI = "https://api.github.com/repos/rodrigopedettitrabajos-hub/HerramientasHioffice/releases/latest"

var nueva struct{ tag, url string } // versión publicada distinta de la que corre (protegida por mu)

// Revisa al abrir y cada 6 horas. Las compilaciones locales ("dev") no se actualizan solas.
func watchReleases() {
	if version == "dev" {
		return
	}
	for {
		if tag, url, err := latestRelease(); err != nil {
			log.Printf("No se pudo consultar la última versión: %v", err)
		} else if tag != version && url != "" {
			mu.Lock()
			nueva.tag, nueva.url = tag, url
			mu.Unlock()
		}
		time.Sleep(6 * time.Hour)
	}
}

func latestRelease() (tag, url string, err error) {
	resp, err := ghClient.Get(releaseAPI)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", "", errors.New(resp.Status)
	}
	var r struct {
		Tag    string `json:"tag_name"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&r); err != nil {
		return "", "", err
	}
	for _, a := range r.Assets {
		if a.Name == "Herramientas_HiOffice.exe" {
			url = a.URL
		}
	}
	return r.Tag, url, nil
}

func newRelease() bool { return nuevaTag() != "" }

func nuevaTag() string { mu.Lock(); defer mu.Unlock(); return nueva.tag }

// Baja la versión publicada y la deja en lugar del .exe actual (que pasa a .old.exe, como al instalar a mano).
func applyRelease() error {
	mu.Lock()
	tag, url := nueva.tag, nueva.url
	mu.Unlock()
	if url == "" || exePath == "" {
		return nil
	}
	resp, err := ghClient.Get(url)
	if err != nil {
		log.Printf("Descarga de %s: %v", tag, err)
		var ue *neturl.Error
		if errors.As(err, &ue) {
			err = ue.Err // sin el enlace firmado de GitHub, que ocupa todo el aviso
		}
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 200<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 || !bytes.HasPrefix(b, []byte("MZ")) {
		return errors.New("la descarga no es un programa válido (" + resp.Status + ")")
	}
	nuevo := exePath + ".new"
	if err := os.WriteFile(nuevo, b, 0o755); err != nil {
		return err
	}
	old := strings.TrimSuffix(exePath, ".exe") + ".old.exe"
	// Si el .old.exe anterior sigue bloqueado (abierto, OneDrive, antivirus), usa otro nombre; cleanOld borra todos.
	if err := os.Remove(old); err != nil && !os.IsNotExist(err) {
		old = strings.TrimSuffix(exePath, ".exe") + ".old" + strconv.FormatInt(time.Now().Unix(), 10) + ".exe"
	}
	var err2 error
	for i := 0; i < 10; i++ {
		if err2 = os.Rename(exePath, old); err2 == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err2 != nil {
		os.Remove(nuevo)
		return err2
	}
	if err := os.Rename(nuevo, exePath); err != nil {
		os.Rename(old, exePath)
		return err
	}
	log.Printf("Actualizado de %s a %s", version, tag)
	return nil
}

var releasesAPI = "https://api.github.com/repos/rodrigopedettitrabajos-hub/HerramientasHioffice/releases?per_page=20"

// Quita del texto de la versión las líneas de firma de los commits.
var reFirma = regexp.MustCompile(`(?m)^(Co-[Aa]uthored-[Bb]y:|Claude-Session:|-{3,}\s*$).*\n?`)

// Lista de versiones publicadas con sus novedades (para el botón "Novedades").
func handleNovedades(w http.ResponseWriter, r *http.Request) {
	resp, err := ghClient.Get(releasesAPI)
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": "No se pudo consultar GitHub: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	var rs []struct {
		Tag   string `json:"tag_name"`
		Fecha string `json:"published_at"`
		Notas string `json:"body"`
	}
	if resp.StatusCode != 200 || json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&rs) != nil {
		writeJSON(w, 200, map[string]string{"error": "GitHub respondió " + resp.Status})
		return
	}
	// La más nueva primero (GitHub no las devuelve por fecha cuando hay etiquetas con otro formato, como v2026.11.24-1).
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Fecha > rs[j].Fecha })
	out := []map[string]string{}
	for _, x := range rs {
		out = append(out, map[string]string{"tag": x.Tag, "fecha": x.Fecha, "notas": strings.TrimSpace(reFirma.ReplaceAllString(x.Notas, ""))})
	}
	writeJSON(w, 200, map[string]any{"actual": version, "versiones": out})
}
