// Herramientas HiOffice: servidor local que sirve la herramienta (embebida), reenvía las llamadas
// al Bridge de HiOffice (evita el bloqueo CORS del navegador) y guarda configuración y nombres
// de columnas en archivos junto al .exe.
package main

import (
	"bytes"
	"crypto/rand"
	"crypto/tls"
	_ "embed"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed Herramientas_HiOffice.html
var page []byte

const appID = "herramientas-hioffice"
var basePort = 8765 // HIO_PORT lo cambia (sólo para pruebas)
const maxBody = 60 << 20

var (
	token     string
	dataDir   string
	extraHost = map[string]bool{}
	mu        sync.Mutex
	lastPing  time.Time
	sawPing   bool
	quit      = make(chan struct{}, 1)
	restart   = make(chan struct{}, 1)
	exePath   string      // ruta con la que se abrió el programa
	exeStamp  os.FileInfo // tamaño y fecha del .exe al arrancar
)

// Hay versión nueva si el .exe en disco ya no es el que está corriendo.
func updateAvailable() bool {
	if exeStamp == nil {
		return false
	}
	fi, err := os.Stat(exePath)
	return newRelease() || err == nil && (fi.Size() != exeStamp.Size() || !fi.ModTime().Equal(exeStamp.ModTime()))
}

func handleRestart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "método no permitido"})
		return
	}
	if _, err := os.Stat(exePath); err != nil {
		writeJSON(w, 200, map[string]string{"error": "No encuentro el programa en " + exePath})
		return
	}
	if err := applyRelease(); err != nil {
		writeJSON(w, 200, map[string]string{"error": "No se pudo bajar la versión nueva: " + err.Error()})
		return
	}
	writeJSON(w, 200, map[string]bool{"ok": true})
	select {
	case restart <- struct{}{}:
	default:
	}
}

// Borra la copia vieja que queda al actualizar (Herramientas_HiOffice.old.exe); reintenta mientras la vieja termina de cerrarse.
func cleanOld() {
	if exePath == "" {
		return
	}
	ext := filepath.Ext(exePath)
	old := strings.TrimSuffix(exePath, ext) + ".old" + ext
	for i := 0; i < 20; i++ {
		if err := os.Remove(old); err == nil || os.IsNotExist(err) {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func randToken() string { b := make([]byte, 24); rand.Read(b); return hex.EncodeToString(b) }

func hostAllowed(raw string) bool {
	u, err := url.Parse(raw)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	if extraHost[strings.ToLower(u.Host)] || extraHost[h] {
		return u.Scheme == "http" || u.Scheme == "https"
	}
	if u.Scheme != "https" {
		return false
	}
	return h == "cloudlicense.icg.eu" || strings.HasSuffix(h, ".icg.eu") || strings.HasSuffix(h, ".hiopos.com")
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(v)
}

func localHost(r *http.Request) bool {
	h := r.Host
	if i := strings.LastIndex(h, ":"); i >= 0 {
		h = h[:i]
	}
	return h == "127.0.0.1" || h == "localhost"
}

func guard(next http.HandlerFunc, needToken bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !localHost(r) {
			writeJSON(w, 403, map[string]string{"error": "host no permitido"})
			return
		}
		if needToken && r.Header.Get("X-Helper-Token") != token {
			writeJSON(w, 403, map[string]string{"error": "token del ayudante incorrecto; recarga la página"})
			return
		}
		next(w, r)
	}
}

func handlePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" && r.URL.Path != "/index.html" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Write(page)
}

func handlePing(w http.ResponseWriter, r *http.Request) {
	mu.Lock()
	lastPing, sawPing = time.Now(), true
	mu.Unlock()
	// Sin cabeceras CORS: otras webs no pueden leer el token.
	writeJSON(w, 200, map[string]any{"ok": true, "app": appID, "token": token, "version": 2, "exe": true,
		"dataDir": dataDir, "secrets": secretsSupported(), "update": updateAvailable(), "appVersion": version, "db": db != nil})
}

type relayReq struct {
	Method  string            `json:"method"`
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Body    *string           `json:"body"`
	File    *struct {
		Field, Name, Type, Data string
	} `json:"file"`
}

var client = &http.Client{Timeout: 5 * time.Minute, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12}},
	// Go reenvía x-auth-token en las redirecciones: sólo se siguen si el destino también está permitido.
	CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if !hostAllowed(r.URL.String()) {
			return errors.New("el servidor redirigió a un destino no permitido: " + r.URL.Hostname())
		}
		if len(via) >= 10 {
			return errors.New("demasiadas redirecciones")
		}
		return nil
	}}

func handleRelay(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "método no permitido"})
		return
	}
	var q relayReq
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&q); err != nil {
		writeJSON(w, 200, map[string]string{"error": "petición inválida: " + err.Error()})
		return
	}
	q.Method = strings.ToUpper(q.Method)
	if (q.Method != "GET" && q.Method != "POST") || !hostAllowed(q.URL) {
		u, _ := url.Parse(q.URL)
		h := q.URL
		if u != nil {
			h = u.Hostname()
		}
		writeJSON(w, 200, map[string]string{"error": "Destino no permitido: " + h})
		return
	}
	var body io.Reader
	hdr := http.Header{}
	for k, v := range q.Headers {
		switch strings.ToLower(k) {
		case "x-auth-token", "content-type", "accept":
			hdr.Set(k, v)
		}
	}
	if q.File != nil {
		data, err := base64.StdEncoding.DecodeString(q.File.Data)
		if err != nil {
			writeJSON(w, 200, map[string]string{"error": "archivo inválido"})
			return
		}
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		field := q.File.Field
		if field == "" {
			field = "file"
		}
		ph := textproto.MIMEHeader{}
		name := strings.NewReplacer(`"`, "", "\r", "", "\n", "").Replace(q.File.Name)
		ph.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"; filename="%s"`, field, name))
		ct := q.File.Type
		if ct == "" {
			ct = "application/octet-stream"
		}
		ph.Set("Content-Type", ct)
		part, _ := mw.CreatePart(ph)
		part.Write(data)
		mw.Close()
		body = &buf
		hdr.Set("Content-Type", mw.FormDataContentType())
	} else if q.Body != nil {
		body = strings.NewReader(*q.Body)
	} else if q.Method == "POST" {
		body = strings.NewReader("")
	}
	req, err := http.NewRequest(q.Method, q.URL, body)
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": err.Error()})
		return
	}
	req.Header = hdr
	resp, err := client.Do(req)
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": "No se pudo conectar: " + err.Error()})
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": "Se cortó la respuesta del servidor: " + err.Error()})
		return
	}
	if len(raw) > maxBody {
		writeJSON(w, 200, map[string]string{"error": fmt.Sprintf("La respuesta del servidor pasa de %d MB: es demasiado grande para mostrarla aquí.", maxBody>>20)})
		return
	}
	log.Printf("%s %s%s -> %d", q.Method, req.URL.Hostname(), req.URL.Path, resp.StatusCode)
	writeJSON(w, 200, map[string]any{"status": resp.StatusCode, "contentType": resp.Header.Get("Content-Type"), "body": strings.ToValidUTF8(string(raw), "�")})
}

// ---- Almacenamiento en archivos junto al .exe ----
var storeFiles = map[string]string{"config": "configuracion.json", "names": "nombres_columnas.json", "auto": "nombres_aprendidos.json"}

func handleStore(w http.ResponseWriter, r *http.Request) {
	file, ok := storeFiles[r.URL.Query().Get("key")]
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "clave desconocida"})
		return
	}
	p := filepath.Join(dataDir, file)
	switch r.Method {
	case http.MethodGet:
		b, err := os.ReadFile(p)
		if err != nil {
			writeJSON(w, 200, map[string]any{"value": nil})
			return
		}
		var v any
		if json.Unmarshal(b, &v) != nil {
			writeJSON(w, 200, map[string]any{"value": nil})
			return
		}
		writeJSON(w, 200, map[string]any{"value": v})
	case http.MethodPut:
		var v any
		if err := json.NewDecoder(io.LimitReader(r.Body, 5<<20)).Decode(&v); err != nil {
			writeJSON(w, 400, map[string]string{"error": "JSON inválido"})
			return
		}
		b, _ := json.MarshalIndent(v, "", "  ")
		if err := writeAtomic(p, b); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		writeJSON(w, 405, map[string]string{"error": "método no permitido"})
	}
}

func writeAtomic(p string, b []byte) error {
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}

// ---- Contraseña recordada (cifrada con DPAPI de Windows) ----
var secretFiles = map[string]string{"": "credencial.bin", "migrate": "credencial_migrate.bin"}

func handleSecret(w http.ResponseWriter, r *http.Request) {
	file, ok := secretFiles[r.URL.Query().Get("key")]
	if !ok {
		writeJSON(w, 400, map[string]string{"error": "clave desconocida"})
		return
	}
	p := filepath.Join(dataDir, file)
	switch r.Method {
	case http.MethodGet:
		b, err := os.ReadFile(p)
		if err != nil {
			writeJSON(w, 200, map[string]any{"value": nil})
			return
		}
		plain, err := unprotect(b)
		if err != nil {
			writeJSON(w, 200, map[string]any{"value": nil, "error": "No se pudo descifrar (¿otro usuario u otro equipo?)."})
			return
		}
		writeJSON(w, 200, map[string]any{"value": string(plain)})
	case http.MethodPut:
		var v struct{ Value string }
		if err := json.NewDecoder(io.LimitReader(r.Body, 1<<16)).Decode(&v); err != nil {
			writeJSON(w, 400, map[string]string{"error": "JSON inválido"})
			return
		}
		if v.Value == "" {
			os.Remove(p)
			writeJSON(w, 200, map[string]bool{"ok": true})
			return
		}
		enc, err := protect([]byte(v.Value))
		if err != nil {
			writeJSON(w, 200, map[string]string{"error": err.Error()})
			return
		}
		if err := writeAtomic(p, enc); err != nil {
			writeJSON(w, 500, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]bool{"ok": true})
	default:
		writeJSON(w, 405, map[string]string{"error": "método no permitido"})
	}
}

func handleQuit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]bool{"ok": true})
	select {
	case quit <- struct{}{}:
	default:
	}
}

func openBrowser(u string) {
	if os.Getenv("HIO_NO_BROWSER") == "1" {
		return
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", u)
	case "darwin":
		cmd = exec.Command("open", u)
	default:
		cmd = exec.Command("xdg-open", u)
	}
	cmd.Start()
}

func pickDataDir() string {
	if d := os.Getenv("HIO_DATA_DIR"); d != "" {
		os.MkdirAll(d, 0o700)
		return d
	}
	if exe, err := os.Executable(); err == nil {
		d := filepath.Join(filepath.Dir(exe), "datos")
		if os.MkdirAll(d, 0o700) == nil {
			if f, err := os.CreateTemp(d, ".w"); err == nil {
				f.Close()
				os.Remove(f.Name())
				return d
			}
		}
	}
	base, _ := os.UserConfigDir()
	d := filepath.Join(base, "HerramientasHiOffice")
	os.MkdirAll(d, 0o700)
	return d
}

// Si ya hay una instancia abierta, abre el navegador en ella y termina.
func existingInstance(port int) bool {
	c := http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := c.Get(fmt.Sprintf("http://127.0.0.1:%d/__helper/ping", port))
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	var v struct{ App string }
	json.NewDecoder(resp.Body).Decode(&v)
	return v.App == appID
}

func main() {
	dataDir = pickDataDir()
	if p, err := os.Executable(); err == nil {
		exePath = p
		exeStamp, _ = os.Stat(p)
	}
	if p, err := fmt.Sscan(os.Getenv("HIO_PORT"), &basePort); p != 1 || err != nil {
		basePort = 8765
	}
	restarted := os.Getenv("HIO_RESTARTED") == "1"
	os.Unsetenv("HIO_RESTARTED")
	for _, h := range strings.Split(os.Getenv("HIO_EXTRA_HOSTS"), ",") {
		if h = strings.TrimSpace(strings.ToLower(h)); h != "" {
			extraHost[h] = true
		}
	}
	token = randToken()

	var ln net.Listener
	var port int
	// Al reiniciar, la instancia anterior puede tardar un momento en soltar el puerto: se espera para conservar la misma dirección.
	if restarted {
		for i := 0; i < 25; i++ {
			if l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", basePort)); err == nil {
				l.Close()
				break
			}
			time.Sleep(200 * time.Millisecond)
		}
	}
	for p := basePort; p < basePort+10; p++ {
		l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p))
		if err == nil {
			ln, port = l, p
			break
		}
		if existingInstance(p) {
			openBrowser(fmt.Sprintf("http://127.0.0.1:%d/", p))
			return
		}
	}
	if ln == nil {
		fatal(errors.New("no hay ningún puerto libre entre 8765 y 8774"))
	}
	// El registro se abre recién ahora (antes, abrir el .exe por segunda vez lo vaciaba). Se acumula y se reinicia al pasar 1 MB.
	logPath := filepath.Join(dataDir, "registro.log")
	if fi, err := os.Stat(logPath); err == nil && fi.Size() > 1<<20 {
		os.Remove(logPath)
	}
	if lf, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600); err == nil {
		log.SetOutput(lf) // sólo al archivo: sin consola (windowsgui) escribir en stderr falla y cortaba el registro
	}
	go cleanOld()
	go watchReleases()
	if err := openDB(); err != nil {
		log.Printf("No se pudo abrir la base de datos: %v", err)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", guard(handlePage, false))
	mux.HandleFunc("/__helper/ping", guard(handlePing, false))
	mux.HandleFunc("/__helper/relay", guard(handleRelay, true))
	mux.HandleFunc("/__helper/store", guard(handleStore, true))
	mux.HandleFunc("/__helper/secret", guard(handleSecret, true))
	mux.HandleFunc("/__helper/quit", guard(handleQuit, true))
	mux.HandleFunc("/__helper/restart", guard(handleRestart, true))
	mux.HandleFunc("/__helper/db", guard(handleDB, true))
	mux.HandleFunc("/__helper/migrate", guard(handleMigrate, true))
	srv := &http.Server{Handler: mux, ReadHeaderTimeout: 10 * time.Second}
	addr := fmt.Sprintf("http://127.0.0.1:%d/", port)
	log.Printf("Herramientas HiOffice en %s (datos: %s)", addr, dataDir)
	go srv.Serve(ln)
	if !restarted {
		openBrowser(addr)
	}

	// Se cierra al pulsar "Cerrar herramienta" o si nadie tiene la página abierta durante 3 minutos.
	idle := 3 * time.Minute
	t := time.NewTicker(15 * time.Second)
	start := time.Now()
	for {
		select {
		case <-quit:
			time.Sleep(300 * time.Millisecond)
			return
		case <-restart:
			// Suelta el puerto y abre de nuevo el .exe que está en disco (la versión nueva, si se actualizó).
			time.Sleep(300 * time.Millisecond)
			srv.Close()
			cmd := exec.Command(exePath)
			cmd.Dir = filepath.Dir(exePath)
			cmd.Env = append(os.Environ(), "HIO_RESTARTED=1")
			if err := cmd.Start(); err != nil {
				log.Printf("No se pudo reiniciar: %v", err)
				fatal(fmt.Errorf("no se pudo reiniciar: %v", err))
			}
			log.Printf("Reiniciando…")
			return
		case <-t.C:
			mu.Lock()
			lp, saw := lastPing, sawPing
			mu.Unlock()
			if (saw && time.Since(lp) > idle) || (!saw && time.Since(start) > 5*time.Minute) {
				log.Printf("Sin páginas abiertas: cerrando.")
				return
			}
		}
	}
}
