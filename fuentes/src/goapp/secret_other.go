//go:build !windows

package main

import (
	"errors"
	"log"
)

// Fuera de Windows no hay DPAPI: no se recuerdan contraseñas.
func secretsSupported() bool { return false }
func protect([]byte) ([]byte, error) {
	return nil, errors.New("recordar contraseña sólo está disponible en Windows")
}
func unprotect([]byte) ([]byte, error) { return nil, errors.New("no disponible") }
func fatal(e error)                    { log.Fatal(e) }
