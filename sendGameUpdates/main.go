package main

import (
	"log"

	"github.com/GoogleCloudPlatform/functions-framework-go/funcframework"
)

func init() {
	funcframework.RegisterHTTPFunction("/", PollHandler)
}

func main() {
	if err := funcframework.Start("8080"); err != nil {
		log.Fatalf("Failed to start function: %v", err)
	}
}
