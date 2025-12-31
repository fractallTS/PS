// Glavna datoteka za Razpravljalnico
// Podpira dva načina: strežnik (server) in odjemalec (client)
package main

import (
	"flag"
	"fmt"
	"os"
)

func main() {
	// Definiramo komandno vrstico argumente
	mode := flag.String("mode", "server", "Mode: 'server' or 'client'")
	address := flag.String("address", ":50051", "Address (for server: listen address, for client: server address)")
	flag.Parse()

	switch *mode {
	case "server":
		fmt.Println("Starting Razpravljalnica server...")
		fmt.Printf("Listening on %s\n", *address)
		StartServer(*address)
	case "client":
		fmt.Println("Starting Razpravljalnica client...")
		Client(*address)
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use 'server' or 'client'\n", *mode)
		os.Exit(1)
	}
}

