package main

import (
	"log"

	"github.com/matbur/stargo/examples/hello/app"
)

func main() {
	application := app.New()
	defer application.Close()

	if err := application.ListenAndServe(":8080"); err != nil {
		log.Fatal(err)
	}
}
