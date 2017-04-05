package main

import (
	"fmt"
	"net/http"
	"os"
)


func main() {
	http.HandleFunc("/update", update)
	http.HandleFunc("/", homepage)
	http.ListenAndServe(":"+determinePort(), nil)
}

func determinePort() (string) {
	if os.Getenv("FLIPBIT_PORT") == "" {
		return "8080"
	}
	return os.Getenv("FLIPBIT_PORT")

}

func update(w http.ResponseWriter, r *http.Request) {
	var updater Update
	updater.Update(w,r)
}


func homepage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<html><title>Welcome to flipbit-nginx</title><body><p>Welcome to flipbit!</p></body></html>")
}