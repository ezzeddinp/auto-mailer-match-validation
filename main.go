// main.go
package main

import (
	"fmt"
	"log"
	"mode-serius/config"
	"mode-serius/handlers"
	"net/http"
	"os"

	"github.com/gorilla/mux"
)

func main() {
	if err := os.MkdirAll("./temp", os.ModePerm); err != nil {
		log.Fatalf("Gagal membuat folder temp: %v", err)
	}

	config.LoadConfig()
	r := mux.NewRouter()

	r.HandleFunc("/login", handlers.HandleLogin)
	r.HandleFunc("/callback", handlers.HandleCallback)
	r.HandleFunc("/validate", handlers.HandleValidateFile).Methods("POST")

	fmt.Println("Server berjalan di http://localhost:8080")
	log.Fatalf("Server gagal: %v", http.ListenAndServe(":8080", r))
}
