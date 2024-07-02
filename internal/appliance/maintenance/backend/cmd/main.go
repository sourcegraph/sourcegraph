package main

import (
	"fmt"

	"github.com/sourcegraph/sourcegraph/internal/appliance/maintenance/backend/api"
)

func main() {
	server := api.New()
	fmt.Println("Starting mock API server")
	server.Run()
}
