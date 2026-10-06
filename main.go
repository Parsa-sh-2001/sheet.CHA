package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strings"

	_ "modernc.org/sqlite"
)

type Sheet struct {
	Name   string `json:"name"`
	Author string `json:"author"`
	Path   string `json:"url"`
	Pdf    string `json:"pdf"`
}

type PageData struct {
	Sheets []Sheet
	Query  string
}

var sheets []Sheet
var db *sql.DB

func LoadSheets() error {
	rows, err := db.Query("SELECT name, author, path, pdf FROM sheets")
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sheet Sheet

		err := rows.Scan(
			&sheet.Name,
			&sheet.Author,
			&sheet.Path,
			&sheet.Pdf,
		)
		if err != nil {
			fmt.Println(err)
			return err
		}

		sheets = append(sheets, sheet)
	}
	return nil
}

func initDB() error {
	var err error

	db, err = sql.Open("sqlite", "sheets.db")
	if err != nil {
		return err
	}

	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS sheets (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			author TEXT NOT NULL,
			path TEXT NOT NULL UNIQUE,
			pdf TEXT NOT NULL
		)
	`)
	if err != nil {
		return err
	}
	return nil
}

func MainPage(w http.ResponseWriter, r *http.Request) {
	var results []Sheet
	query := r.URL.Query().Get("q")

	rows, err := db.Query(`
	    SELECT name, author, path, pdf
	    FROM sheets
	    WHERE name LIKE ? OR author LIKE ?
	`, "%"+query+"%", "%"+query+"%")
	if err != nil {
		http.Error(w, "DB Problem.", http.StatusInternalServerError)
	}
	defer rows.Close()
	for rows.Next() {
		var sheet Sheet
		err := rows.Scan(
			&sheet.Name,
			&sheet.Author,
			&sheet.Path,
			&sheet.Pdf,
		)
		if err != nil {
			fmt.Println(err)
			return
		}
		results = append(results, sheet)
	}

	if r.URL.Path == "/" {
		tmpl, err := template.ParseFiles("./Assets/index.html")
		if err != nil {
			http.Error(w, "Template error.", http.StatusInternalServerError)
		}

		data := PageData{
			Sheets: results,
			Query:  query,
		}

		w.Header().Set("Content-Type", "text/html")
		tmpl.Execute(w, data)
		return
	}

	ViewSheet(w, r)
}

func ServePDF(w http.ResponseWriter, r *http.Request) {
	file := strings.TrimPrefix(r.URL.Path, "/pdf/")
	fmt.Println("Serving File:", file)

	http.ServeFile(w, r, "./Assets/PDF/"+file)
}

func ViewSheet(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ViewSheet called:", r.URL.Path)
	url := r.URL.Path
	url = strings.Trim(url, "/")

	fmt.Println("URL:", url)
	fmt.Println("SHEETS:", sheets)

	for _, sheet := range sheets {
		if sheet.Path == url {
			// w.Header().Set("Content-Type", "application/json")

			// json.NewEncoder(w).Encode(sheet)
			// return

			tmpl, err := template.ParseFiles("./Assets/form.html")
			if err != nil {
				http.Error(w, "Template Error", http.StatusInternalServerError)
				return
			}

			w.Header().Set("Content-Type", "text/html")
			tmpl.Execute(w, sheet)
			return
		}
	}
	http.Error(w, "Doesn't exist.", http.StatusNotFound)
}

func AddSheet(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./Assets/add_sheet.html")
}

func ServeSheet(w http.ResponseWriter, r *http.Request) {
	fmt.Println("ServeSheet called:", r.Method, r.URL.Path)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed!", http.StatusMethodNotAllowed)
	}

	pdf, header, err := r.FormFile("pdf")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer pdf.Close()
	name := r.FormValue("name")
	author := r.FormValue("author")
	path := r.FormValue("path")

	file_path := "./Assets/PDF/" + header.Filename

	filer, err := os.Create(file_path)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer filer.Close()

	io.Copy(filer, pdf)

	appendant := Sheet{
		Name:   name,
		Author: author,
		Path:   path,
		Pdf:    header.Filename,
	}

	_, err = db.Exec(
		"INSERT INTO sheets (name, author, path, pdf) VALUES (?, ?, ?, ?)",
		name, author, path, header.Filename,
	)
	if err != nil {
		fmt.Println("DB ERROR :", err)
		return
	}
	sheets = append(sheets, appendant)

	fmt.Printf("Name: %v \n File: %v \n Author: %v \n Path: %v \n", name, header.Filename, author, path)
}

func main() {
	if err := initDB(); err != nil {
		panic(err)
	}
	if err := LoadSheets(); err != nil {
		panic(err)
	}
	http.HandleFunc("/", MainPage)
	http.HandleFunc("/add_sheet", AddSheet)
	http.HandleFunc("/api/sheets", ServeSheet)
	http.HandleFunc("/pdf/", ServePDF)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe("0.0.0.0:"+port, nil)
}
