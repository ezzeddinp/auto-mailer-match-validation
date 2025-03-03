// handlers/auth.go
package handlers

import (
	"fmt"
	"mode-serius/config"
	"net/http"
)

func HandleLogin(w http.ResponseWriter, r *http.Request) {
	// Simulasi redirect ke halaman login, langsung kasih URL dummy
	url := "http://localhost:8080/callback?code=dummycode"
	http.Redirect(w, r, url, http.StatusFound)
}

func HandleCallback(w http.ResponseWriter, r *http.Request) {
	// Cek kode dummy (ga perlu validasi rumit)
	code := r.URL.Query().Get("code")
	if code != "dummycode" {
		http.Error(w, "Kode tidak valid", http.StatusBadRequest)
		return
	}

	// Kasih token dummy
	fmt.Fprintf(w, "Login berhasil! Access Token: %s", config.DummyToken)
}
