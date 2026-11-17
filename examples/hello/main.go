package main

import (
	"log"

	"github.com/matbur/stargo/internal/example"
)

func main() {
	app := example.New()
	defer app.Close()

	if err := app.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
