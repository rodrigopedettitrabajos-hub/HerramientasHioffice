// Base de datos SQLite (datos\pruebas.db) con los clientes de HiOffice y las pruebas de
// exportación e importación guardadas para cada uno.
package main

import (
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // SQLite en Go puro: el .exe sigue sin dependencias
)

var db *sql.DB

const schema = `
CREATE TABLE IF NOT EXISTS clientes (
  customer_id  TEXT PRIMARY KEY,
  nombre       TEXT NOT NULL DEFAULT '',
  email        TEXT NOT NULL DEFAULT '',
  servidor     TEXT NOT NULL DEFAULT '',
  actualizado  TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS pruebas (
  id                INTEGER PRIMARY KEY AUTOINCREMENT,
  customer_id       TEXT NOT NULL REFERENCES clientes(customer_id) ON DELETE CASCADE,
  tipo              TEXT NOT NULL CHECK (tipo IN ('exportacion','importacion')),
  nombre            TEXT NOT NULL,
  config            TEXT NOT NULL,          -- JSON con los campos de la prueba
  archivo_nombre    TEXT,
  archivo_tipo      TEXT,
  archivo           BLOB,                   -- archivo a importar (si se guardó)
  creado            TEXT NOT NULL,
  actualizado       TEXT NOT NULL,
  ultima_ejecucion  TEXT,
  ultimo_resultado  TEXT,
  UNIQUE (customer_id, tipo, nombre)
);
CREATE INDEX IF NOT EXISTS pruebas_cliente ON pruebas(customer_id, tipo);
`

func openDB() error {
	p := filepath.Join(dataDir, "pruebas.db")
	d, err := sql.Open("sqlite", "file:"+filepath.ToSlash(p)+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	d.SetMaxOpenConns(1)
	if _, err := d.Exec(schema); err != nil {
		d.Close()
		return err
	}
	db = d
	return nil
}

func now() string { return time.Now().Format("2006-01-02 15:04:05") }

type dbFile struct {
	Name string `json:"name"`
	Type string `json:"type"`
	Data string `json:"data"` // base64
}

type dbReq struct {
	Op         string          `json:"op"`
	ID         int64           `json:"id"`
	CustomerID string          `json:"customerId"`
	Nombre     string          `json:"nombre"`
	Email      string          `json:"email"`
	Servidor   string          `json:"servidor"`
	Tipo       string          `json:"tipo"`
	Config     json.RawMessage `json:"config"`
	Archivo    *dbFile         `json:"archivo"`
	Resultado  string          `json:"resultado"`
}

func handleDB(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, 405, map[string]string{"error": "método no permitido"})
		return
	}
	if db == nil {
		writeJSON(w, 200, map[string]string{"error": "La base de datos no está disponible."})
		return
	}
	var q dbReq
	if err := json.NewDecoder(io.LimitReader(r.Body, maxBody)).Decode(&q); err != nil {
		writeJSON(w, 400, map[string]string{"error": "petición inválida: " + err.Error()})
		return
	}
	q.CustomerID = strings.TrimSpace(q.CustomerID)
	q.Nombre = strings.TrimSpace(q.Nombre)
	res, err := dbOp(q)
	if err != nil {
		writeJSON(w, 200, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

type userErr string

func (e userErr) Error() string { return string(e) }

func dbOp(q dbReq) (any, error) {
	switch q.Op {
	case "clients":
		rows, err := db.Query(`SELECT c.customer_id, c.nombre, c.email, c.servidor, c.actualizado,
			(SELECT COUNT(*) FROM pruebas p WHERE p.customer_id=c.customer_id AND p.tipo='exportacion'),
			(SELECT COUNT(*) FROM pruebas p WHERE p.customer_id=c.customer_id AND p.tipo='importacion')
			FROM clientes c ORDER BY c.nombre, c.customer_id`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id, nom, em, srv, act string
			var ne, ni int
			if err := rows.Scan(&id, &nom, &em, &srv, &act, &ne, &ni); err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"customerId": id, "nombre": nom, "email": em, "servidor": srv, "actualizado": act, "exportaciones": ne, "importaciones": ni})
		}
		return map[string]any{"items": out}, rows.Err()

	case "saveClient":
		if q.CustomerID == "" {
			return nil, userErr("Falta el cliente.")
		}
		_, err := db.Exec(`INSERT INTO clientes(customer_id, nombre, email, servidor, actualizado) VALUES(?,?,?,?,?)
			ON CONFLICT(customer_id) DO UPDATE SET
			  nombre = CASE WHEN excluded.nombre <> '' THEN excluded.nombre ELSE clientes.nombre END,
			  email = CASE WHEN excluded.email <> '' THEN excluded.email ELSE clientes.email END,
			  servidor = CASE WHEN excluded.servidor <> '' THEN excluded.servidor ELSE clientes.servidor END,
			  actualizado = excluded.actualizado`, q.CustomerID, q.Nombre, q.Email, q.Servidor, now())
		if err != nil {
			return nil, err
		}
		var nom string
		db.QueryRow(`SELECT nombre FROM clientes WHERE customer_id=?`, q.CustomerID).Scan(&nom)
		return map[string]any{"ok": true, "nombre": nom}, nil

	case "list":
		rows, err := db.Query(`SELECT id, nombre, config, COALESCE(archivo_nombre,''), COALESCE(length(archivo),0), actualizado,
			COALESCE(ultima_ejecucion,''), COALESCE(ultimo_resultado,'')
			FROM pruebas WHERE customer_id=? AND tipo=? ORDER BY nombre COLLATE NOCASE`, q.CustomerID, q.Tipo)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		out := []map[string]any{}
		for rows.Next() {
			var id int64
			var nom, cfg, an, act, ue, ur string
			var size int64
			if err := rows.Scan(&id, &nom, &cfg, &an, &size, &act, &ue, &ur); err != nil {
				return nil, err
			}
			out = append(out, map[string]any{"id": id, "nombre": nom, "config": json.RawMessage(cfg), "archivoNombre": an, "archivoBytes": size,
				"actualizado": act, "ultimaEjecucion": ue, "ultimoResultado": ur})
		}
		return map[string]any{"items": out}, rows.Err()

	case "get":
		var nom, cfg string
		var an, at sql.NullString
		var data []byte
		err := db.QueryRow(`SELECT nombre, config, archivo_nombre, archivo_tipo, archivo FROM pruebas WHERE id=?`, q.ID).Scan(&nom, &cfg, &an, &at, &data)
		if err == sql.ErrNoRows {
			return nil, userErr("La prueba ya no existe.")
		} else if err != nil {
			return nil, err
		}
		res := map[string]any{"id": q.ID, "nombre": nom, "config": json.RawMessage(cfg)}
		if data != nil {
			res["archivo"] = dbFile{Name: an.String, Type: at.String, Data: base64.StdEncoding.EncodeToString(data)}
		}
		return res, nil

	case "save":
		if q.CustomerID == "" {
			return nil, userErr("Primero conéctate: la prueba se guarda en el cliente de la sesión.")
		}
		if q.Nombre == "" {
			return nil, userErr("Escribe un nombre para la prueba.")
		}
		if q.Tipo != "exportacion" && q.Tipo != "importacion" {
			return nil, userErr("Tipo de prueba desconocido.")
		}
		if len(q.Config) == 0 || !json.Valid(q.Config) {
			return nil, userErr("Configuración inválida.")
		}
		var an, at any
		var data any
		if q.Archivo != nil {
			b, err := base64.StdEncoding.DecodeString(q.Archivo.Data)
			if err != nil {
				return nil, userErr("Archivo inválido.")
			}
			an, at, data = q.Archivo.Name, q.Archivo.Type, b
		}
		// El cliente tiene que existir (por la clave foránea)
		if _, err := db.Exec(`INSERT OR IGNORE INTO clientes(customer_id, actualizado) VALUES(?,?)`, q.CustomerID, now()); err != nil {
			return nil, err
		}
		t := now()
		_, err := db.Exec(`INSERT INTO pruebas(customer_id, tipo, nombre, config, archivo_nombre, archivo_tipo, archivo, creado, actualizado)
			VALUES(?,?,?,?,?,?,?,?,?)
			ON CONFLICT(customer_id, tipo, nombre) DO UPDATE SET config=excluded.config, archivo_nombre=excluded.archivo_nombre,
			  archivo_tipo=excluded.archivo_tipo, archivo=excluded.archivo, actualizado=excluded.actualizado`,
			q.CustomerID, q.Tipo, q.Nombre, string(q.Config), an, at, data, t, t)
		if err != nil {
			return nil, err
		}
		var id int64
		db.QueryRow(`SELECT id FROM pruebas WHERE customer_id=? AND tipo=? AND nombre=?`, q.CustomerID, q.Tipo, q.Nombre).Scan(&id)
		return map[string]any{"ok": true, "id": id}, nil

	case "delete":
		if _, err := db.Exec(`DELETE FROM pruebas WHERE id=?`, q.ID); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil

	case "result":
		if _, err := db.Exec(`UPDATE pruebas SET ultima_ejecucion=?, ultimo_resultado=? WHERE id=?`, now(), q.Resultado, q.ID); err != nil {
			return nil, err
		}
		return map[string]bool{"ok": true}, nil
	}
	return nil, userErr("Operación desconocida: " + q.Op)
}
