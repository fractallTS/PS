// Komunikacija po protokolu gRPC
// odjemalec

package main

import (
	"api/grpc/protobufStorage"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func Client(url string) {
	// vzpostavimo povezavo s strežnikom
	fmt.Printf("gRPC client connecting to %v\n", url)
	conn, err := grpc.NewClient(url, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// vzpostavimo vmesnik gRPC
	grpcClient := protobufStorage.NewCRUDClient(conn)

	// vzpostavimo izvajalno okolje za Subscribe (brez timeout, ker je to dolgotrajni tok)
	contextSubscribe, cancelSubscribe := context.WithCancel(context.Background())
	defer cancelSubscribe()

	// vzpostavimo izvajalno okolje za CRUD operacije
	contextCRUD, cancelCRUD := context.WithTimeout(context.Background(), time.Second*10)
	defer cancelCRUD()

	// pripravimo strukture, ki jih uporabljamo kot argumente pri klicu oddaljenih metod
	lecturesCreate := protobufStorage.Todo{Task: "predavanja", Completed: false}
	lecturesUpdate := protobufStorage.Todo{Task: "predavanja", Completed: true}
	practicals := protobufStorage.Todo{Task: "vaje", Completed: false}
	homework := protobufStorage.Todo{Task: "domaca naloga", Completed: false}
	readAll := protobufStorage.Todo{Task: "", Completed: false}

	var wg sync.WaitGroup

	// Gorutina za spremljanje dogodkov
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("Starting to subscribe to events...")
		stream, err := grpcClient.Subscribe(contextSubscribe, &emptypb.Empty{})
		if err != nil {
			fmt.Printf("Error subscribing: %v\n", err)
			return
		}
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("Stream closed")
				return
			}
			if err != nil {
				fmt.Printf("Error receiving event: %v\n", err)
				return
			}
			fmt.Printf("Event received: Action=%s, Todo={Task: %s, Completed: %v}\n",
				event.Action, event.T.Task, event.T.Completed)
		}
	}()

	// Kratka pavza, da se Subscribe vzpostavi
	time.Sleep(500 * time.Millisecond)

	// Gorutina za izvajanje CRUD operacij
	wg.Add(1)
	go func() {
		defer wg.Done()
		// ustvarimo zapis
		fmt.Print("1. Create: ")
		if _, err := grpcClient.Create(contextCRUD, &lecturesCreate); err != nil {
			panic(err)
		}
		fmt.Println("done")
		time.Sleep(1 * time.Second)

		// preberemo en zapis
		fmt.Print("2. Read 1: ")
		if response, err := grpcClient.Read(contextCRUD, &lecturesCreate); err == nil {
			fmt.Println(response.Todos, ": done")
		} else {
			panic(err)
		}
		time.Sleep(1 * time.Second)

		// ustvarimo zapis
		fmt.Print("3. Create: ")
		if _, err := grpcClient.Create(contextCRUD, &practicals); err != nil {
			panic(err)
		}
		fmt.Println("done")
		time.Sleep(1 * time.Second)

		// preberemo vse zapise
		fmt.Print("4. Read *: ")
		if response, err := grpcClient.Read(contextCRUD, &readAll); err == nil {
			fmt.Println(response.Todos, ": done")
		} else {
			panic(err)
		}
		time.Sleep(1 * time.Second)

		// posodobimo zapis
		fmt.Print("5. Update: ")
		if _, err := grpcClient.Update(contextCRUD, &lecturesUpdate); err != nil {
			panic(err)
		}
		fmt.Println("done")
		time.Sleep(1 * time.Second)

		// ustvarimo še en zapis
		fmt.Print("6. Create: ")
		if _, err := grpcClient.Create(contextCRUD, &homework); err != nil {
			panic(err)
		}
		fmt.Println("done")
		time.Sleep(1 * time.Second)

		// izbrišemo zapis
		fmt.Print("7. Delete: ")
		if _, err := grpcClient.Delete(contextCRUD, &practicals); err != nil {
			panic(err)
		}
		fmt.Println("done")
		time.Sleep(1 * time.Second)

		// preberemo vse zapise
		fmt.Print("8. Read *: ")
		if response, err := grpcClient.Read(contextCRUD, &readAll); err == nil {
			fmt.Println(response.Todos, ": done")
		} else {
			panic(err)
		}
		time.Sleep(1 * time.Second)

		// zapremo Subscribe
		cancelSubscribe()
	}()

	// Počakamo, da se obe gorutini zaključita
	wg.Wait()
	fmt.Println("Client finished")
}
