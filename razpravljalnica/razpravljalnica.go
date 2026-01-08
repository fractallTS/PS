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
	role := flag.String("role", "head", "Role: 'head' or 'tail' or 'chain' or 'control")
	clientPort := flag.String("clientPort", ":50002", "Client-facing port")
	controlPort := flag.String("controlPort", ":50003", "Control plane port")
	dataPort := flag.String("dataPort", ":50004", "Data plane port")
	clientControlPort := flag.String("clientControlPort", ":50000", "Control plane node address for the client to connect or bind to")
	serverControlPort := flag.String("serverControlPort", ":50001", "Control plane node address for the servers to connect or bind to")
	flag.Parse()

	switch *mode {
	case "server":
		fmt.Println("Starting Razpravljalnica server...")
		switch *role {
		case "head":
			HeadServer(*clientPort, *controlPort, *dataPort, *serverControlPort)
		case "tail":
			TailServer(*clientPort, *controlPort, *dataPort, *serverControlPort)
		case "chain":
			ChainServer(*clientPort, *controlPort, *dataPort, *serverControlPort)
		case "control":
			ControlServer(*clientControlPort, *serverControlPort)
		default:
			fmt.Fprintf(os.Stderr, "Unknown role: %s. Use 'head', 'tail', 'chain', or 'control'\n", *role)
			os.Exit(1)
		}
	case "client":
		//dodal case manual za manual uporabljanje clienta -> CLI
		fmt.Println("Starting Razpravljalnica client...")
		switch *clientMode {
		case "test":
			Client(*clientControlPort)
		case "manual":
			ClientManual(*clientControlPort)
		default:
			fmt.Fprintf(os.Stderr, "Unknown clientMode: %s. Use 'test' or 'manual'\n", *clientMode)
			os.Exit(1)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown mode: %s. Use 'server' or 'client'\n", *mode)
		os.Exit(1)
	}
}
