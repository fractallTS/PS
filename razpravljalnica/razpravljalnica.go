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
	clientMode := flag.String("clientMode", "test", "Client mode: 'test' or 'manual'")
	address := flag.String("address", ":50051", "Address (for server: listen address, for client: server address)")
	flag.Parse()

	switch *mode {
	case "server":
		fmt.Println("Starting Razpravljalnica server...")
		fmt.Printf("Listening on %s\n", *address)
		StartServer(*address)
	case "client":
		//dodal case manual za manual uporabljanje clienta -> CLI
		fmt.Println("Starting Razpravljalnica client...")
		switch *clientMode {
		case "test":
			Client(*address)
		case "manual":
			ClientManual(*address)
		default:
			fmt.Fprintf(os.Stderr, "Unknown clientMode: %s. Use 'test' or 'manual'\n", *clientMode)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use 'server' or 'client'\n", *mode)
		os.Exit(1)
	}
}
