// handlers/validation.go
package handlers

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"log"
	"mode-serius/config"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/xuri/excelize/v2"
)

func readExcelFile(path string) (map[string]string, error) {
	file, err := excelize.OpenFile(path)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file Excel: %v", err)
	}

	rows, err := file.GetRows("Sheet1")
	if err != nil {
		return nil, fmt.Errorf("gagal membaca Sheet1: %v", err)
	}

	data := make(map[string]string)
	seenEmails := make(map[string]bool)

	for _, row := range rows {
		// skip baris yg kosong
		if len(row) == 0 {
			continue
		}

		// cari pasangan nama-email di baris mana saja
		for i := 0; i < len(row)-1; i++ {
			name := strings.TrimSpace(row[i])
			email := strings.TrimSpace(row[i+1])

			// memastikan nama n email tidak kosong
			if name != "" && email != "" {
				email = strings.ToLower(email)
				if _, ok := seenEmails[email]; ok {
					log.Printf("Email duplikat diabaikan: %s", email)
					continue
				}
				seenEmails[email] = true
				data[email] = name
				log.Printf("Found pair: %s -> %s", email, name)
			}
		}
	}

	// klo gaada data valid sama sekali maka
	if len(data) == 0 {
		return nil, errors.New("tidak ada pasangan nama-email yang valid di file Excel")
	}

	return data, nil
}

func readZipFile(path string) ([]string, error) {
	zipReader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("gagal membuka file ZIP: %v", err)
	}
	defer zipReader.Close()

	var pdfNames []string
	for _, file := range zipReader.File {
		if strings.HasSuffix(file.Name, ".pdf") {
			pdfNames = append(pdfNames, filepath.Base(file.Name))
		}
	}
	return pdfNames, nil
}

func parsePDFName(name string) (string, string, error) {
	if !strings.HasSuffix(name, ".pdf") {
		return "", "", errors.New("file PDF tidak punya ekstensi .pdf")
	}
	baseName := strings.TrimSuffix(name, ".pdf")

	lastHyphenIndex := strings.LastIndex(baseName, " - ")
	if lastHyphenIndex == -1 {
		return "", "", errors.New("format nama PDF salah: tidak ada ' - '")
	}

	namePart := strings.TrimSpace(baseName[0:lastHyphenIndex])
	emailPart := strings.TrimSpace(baseName[lastHyphenIndex+3:])
	emailPart = strings.ToLower(emailPart)

	if namePart == "" || emailPart == "" {
		return "", "", errors.New("nama atau email di nama PDF kosong")
	}

	return namePart, emailPart, nil
}

func parsePDFNames(pdfNames []string) (map[string]string, error) {
	data := make(map[string]string)
	seenEmails := make(map[string]bool)

	for _, name := range pdfNames {
		pdfName, email, err := parsePDFName(name)
		if err != nil {
			return nil, fmt.Errorf("gagal parse nama PDF %s: %v", name, err)
		}

		if _, ok := seenEmails[email]; ok {
			return nil, errors.New("email duplikat di file PDF: " + email)
		}
		seenEmails[email] = true
		data[email] = pdfName
	}
	return data, nil
}

func validateData(excelData, pdfData map[string]string) (bool, []string) {
	var mismatches []string

	for email, name := range excelData {
		pdfName, ok := pdfData[email]
		if !ok {
			mismatches = append(mismatches, "email tidak ditemukan di file PDF: "+email)
		} else if name != pdfName {
			mismatches = append(mismatches, fmt.Sprintf("nama tidak cocok untuk email %s: nama Excel %s, nama PDF %s", email, name, pdfName))
		}
	}

	for email := range pdfData {
		if _, ok := excelData[email]; !ok {
			mismatches = append(mismatches, "email ada di PDF tapi tidak di Excel: "+email)
		}
	}

	return len(mismatches) == 0, mismatches
}

func HandleValidateFile(w http.ResponseWriter, r *http.Request) {
	token := r.Header.Get("Authorization")
	if token != "Bearer "+config.DummyToken {
		http.Error(w, "Unauthorized: Token tidak valid", http.StatusUnauthorized)
		fmt.Println("Bearer " + config.DummyToken)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		http.Error(w, "Gagal parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	excelFile, excelHeader, err := r.FormFile("excel")
	if err != nil {
		http.Error(w, "Gagal mendapatkan file Excel: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer excelFile.Close()

	zipFile, zipHeader, err := r.FormFile("zip")
	if err != nil {
		http.Error(w, "Gagal mendapatkan file ZIP: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer zipFile.Close()

	excelPath := "./temp/" + excelHeader.Filename
	zipPath := "./temp/" + zipHeader.Filename

	excelOut, err := os.Create(excelPath)
	if err != nil {
		http.Error(w, "Gagal membuat file Excel sementara: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer excelOut.Close()
	defer os.Remove(excelPath)

	_, err = io.Copy(excelOut, excelFile)
	if err != nil {
		http.Error(w, "Gagal menulis file Excel sementara: "+err.Error(), http.StatusInternalServerError)
		return
	}

	zipOut, err := os.Create(zipPath)
	if err != nil {
		http.Error(w, "Gagal membuat file ZIP sementara: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer zipOut.Close()
	defer os.Remove(zipPath)

	_, err = io.Copy(zipOut, zipFile)
	if err != nil {
		http.Error(w, "Gagal menulis file ZIP sementara: "+err.Error(), http.StatusInternalServerError)
		return
	}

	excelData, err := readExcelFile(excelPath)
	if err != nil {
		http.Error(w, "Error membaca file Excel: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pdfNames, err := readZipFile(zipPath)
	if err != nil {
		http.Error(w, "Error membaca file ZIP: "+err.Error(), http.StatusInternalServerError)
		return
	}

	pdfData, err := parsePDFNames(pdfNames)
	if err != nil {
		http.Error(w, "Error parse nama PDF: "+err.Error(), http.StatusInternalServerError)
		return
	}

	ok, mismatches := validateData(excelData, pdfData)
	if ok {
		fmt.Fprintf(w, "Validation successful. User can submit.")
	} else {
		fmt.Fprintf(w, "Validation failed with the following issues:\n")
		for _, msg := range mismatches {
			fmt.Fprintf(w, "%s\n", msg)
		}
	}
}
