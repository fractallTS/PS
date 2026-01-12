package main

import (
	controlPlane "api/razpravljalnica/protobufControlPlane"
	razpravljalnica "api/razpravljalnica/protobufStorage"
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

func ClientManual(controlURL string) {
	// Najprej se povežemo na control plane in preberemo stanje gruče
	fmt.Printf("gRPC control plane connecting to %v\n", controlURL)
	controlConn, err := grpc.NewClient(controlURL, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer controlConn.Close()

	controlPlaneClient := controlPlane.NewControlPlaneClient(controlConn)

	stateCtx, stateCancel := context.WithTimeout(context.Background(), time.Second*10)
	initialState, err := controlPlaneClient.GetClusterState(stateCtx, &emptypb.Empty{})
	stateCancel()
	if err != nil {
		panic(err)
	}

	fmt.Printf("Cluster state: head=%d at %s, tail=%d at %s, sub=%d at %s\n", initialState.Head.NodeId, initialState.Head.Address, initialState.Tail.NodeId, initialState.Tail.Address, initialState.Sub.NodeId, initialState.Sub.Address)

	headConn, err := grpc.NewClient(initialState.Head.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer headConn.Close()

	subConn, err := grpc.NewClient(initialState.Sub.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer subConn.Close()

	tailConn, err := grpc.NewClient(initialState.Tail.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		panic(err)
	}
	defer tailConn.Close()

	headClient := razpravljalnica.NewMessageBoardClient(headConn)
	tailClient := razpravljalnica.NewMessageBoardClient(tailConn)
	subClient := razpravljalnica.NewMessageBoardClient(subConn)

	// Nov bufio Reader instance
	reader := bufio.NewReader(os.Stdin)

	fmt.Println("\n=== Razpravljalnica Manual Client ===")

	fmt.Print("Type 1 to login or 2 to create a new user: ")
	choice, _ := reader.ReadString('\n')
	choice = strings.TrimSpace(choice)
	var user *razpravljalnica.User

	switch choice {
	case "1":
		// Prijava obstoječega uporabnika
		fmt.Print("Enter userID and token: ")
		userInput, _ := reader.ReadString('\n')
		userInput = strings.TrimSpace(userInput)
		parts := strings.SplitN(userInput, " ", 2)
		if len(parts) < 2 {
			fmt.Println("Invalid input. Expected format: <userID> <token>")
			return
		}
		userID := parseInt64(parts[0])
		token := parts[1]

		ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
		defer cancel()
		user, err := headClient.LoginUser(ctx, &razpravljalnica.LoginUserRequest{
			UserId: userID,
			Token:  token,
		})
		if err != nil {
			fmt.Printf("Error logging in: %v\n", err)
			return
		}
		fmt.Printf("Logged in as user: ID=%d, Name=%s\n\n", user.Id, user.Name)
	case "2":
		// Naredimo nov user z imenom prebranim iz CLI
		fmt.Print("Enter username: ")
		username, _ := reader.ReadString('\n')
		username = strings.TrimSpace(username)
		user = createUser(headClient, username)
	}

	fmt.Println("Type 'h' for help")

	// Glavni loop za CLI iz katerega preberemo ukaze in ji izvedemo
	for {
		fmt.Print("\n> ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		parts := strings.Fields(input)
		command := parts[0]

		switch command {
		case "h", "help":
			showHelp()
		case "createtopic":
			if len(parts) < 2 {
				fmt.Println("Usage: createtopic <name>")
				break
			}
			createTopic(headClient, strings.Join(parts[1:], " "))
		case "listtopics":
			listTopics(headClient)
		case "post":
			if len(parts) < 3 {
				fmt.Println("Usage: post <topicID> <message text>")
				break
			}
			postMessage(headClient, user, parts[1], strings.Join(parts[2:], " "))
		case "getmessages":
			if len(parts) < 2 {
				fmt.Println("Usage: getmessages <topicID>")
				break
			}
			getMessages(tailClient, parts[1])
		case "like":
			if len(parts) < 3 {
				fmt.Println("Usage: like <topicID> <messageID>")
				break
			}
			likeMessage(headClient, user, parts[1], parts[2])
		case "update":
			if len(parts) < 4 {
				fmt.Println("Usage: update <topicID> <messageID> <new text>")
				break
			}
			updateMessage(headClient, user, parts[1], parts[2], strings.Join(parts[3:], " "))
		case "delete":
			if len(parts) < 3 {
				fmt.Println("Usage: delete <topicID> <messageID>")
				break
			}
			deleteMessage(headClient, user, parts[1], parts[2])
		case "sub":
			if len(parts) < 2 {
				fmt.Println("Usage: sub <topicID>")
				break
			}
			// Convert all topic IDs in parts[1:] to a slice of int64
			var topicIDs []int64
			for _, t := range parts[1:] {
				id := parseInt64(t)
				if id != -1 {
					topicIDs = append(topicIDs, id)
				}
			}
			if len(topicIDs) == 0 {
				fmt.Println("No valid topic IDs provided.")
				break
			}
			subToTopic(subClient, user, topicIDs, initialState.Sub.NodeId, initialState.Sub.Address, reader)
			fmt.Println("Unsubscribed from all topics")
		case "exit", "quit":
			fmt.Println("Exiting...")
			return
		default:
			fmt.Printf("Unknown command: %s. Type 'h' for help.\n", command)
		}
	}
}

// Spodaj so definirane vse funkcije, ki jih lahko kličemo iz CLI

func showHelp() {
	fmt.Println("\n=== Available Commands ===")
	fmt.Println("  h, help                             - Show this help message")
	fmt.Println("  createtopic <name>                  - Create a new topic")
	fmt.Println("  listtopics                          - List all topics")
	fmt.Println("  post <topicID> <text>               - Post a message")
	fmt.Println("  getmessages <topicID>               - Get messages from a topic")
	fmt.Println("  like <topicID> <messageID>          - Like a message")
	fmt.Println("  update <topicID> <messageID> <text> - Update a message")
	fmt.Println("  delete <topicID> <messageID>        - Delete a message")
	fmt.Println("  sub <topicID>                       - Subscribe to a topic (real-time events)")
	fmt.Println("  exit, quit                          - Exit the client")
	fmt.Println()
}

// V vsaki funkciji vzpostavimo svoj context saj se lahko izvajajo dlje časa in če pride do timeouta naj se prekinejo
// da ne blokirajo celoten CLI (recimo v primeru če strežnik ne odgovarja)

// Ustvarimo uporabnika
func createUser(client razpravljalnica.MessageBoardClient, name string) *razpravljalnica.User {
	// Vzpostavimo izvajalno okolje
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	// Ustvarimo uporabnika
	user, err := client.CreateUser(ctx, &razpravljalnica.CreateUserRequest{Name: name})
	if err != nil {
		fmt.Printf("Error creating user: %v\n", err)
		return nil
	}
	fmt.Printf("Created user: ID=%d, Name=%s, Token=%s\n\n", user.Id, user.Name, user.Token)
	return user
}

// Ustvarimo temo
func createTopic(client razpravljalnica.MessageBoardClient, topicName string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	topic, err := client.CreateTopic(ctx, &razpravljalnica.CreateTopicRequest{Name: topicName})
	if err != nil {
		fmt.Printf("Error creating topic: %v\n", err)
		return
	}
	fmt.Printf("Created topic: ID=%d, Name=%s\n", topic.Id, topic.Name)
}

// Izpišemo vse teme
func listTopics(client razpravljalnica.MessageBoardClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	resp, err := client.ListTopics(ctx, &emptypb.Empty{})
	if err != nil {
		fmt.Printf("Error listing topics: %v\n", err)
		return
	}
	if len(resp.Topics) == 0 {
		fmt.Println("No topics found")
		return
	}
	fmt.Println("\n=== Topics ===")
	for _, topic := range resp.Topics {
		fmt.Printf("  ID=%d, Name=%s\n", topic.Id, topic.Name)
	}
}

// Objavimo sporočilo
func postMessage(client razpravljalnica.MessageBoardClient, user *razpravljalnica.User, topicIDStr, text string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	topicID := parseInt64(topicIDStr)
	if topicID == -1 {
		return
	}
	msg, err := client.PostMessage(ctx, &razpravljalnica.PostMessageRequest{
		TopicId: topicID,
		UserId:  user.Id,
		Text:    text,
	})
	if err != nil {
		fmt.Printf("Error posting message: %v\n", err)
		return
	}
	fmt.Printf("Posted message: ID=%d, Text='%s'\n", msg.Id, msg.Text)
}

// Pridobimo sporočila iz teme
func getMessages(client razpravljalnica.MessageBoardClient, topicIDStr string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	topicID := parseInt64(topicIDStr)
	if topicID == -1 {
		return
	}
	resp, err := client.GetMessages(ctx, &razpravljalnica.GetMessagesRequest{
		TopicId:       topicID,
		FromMessageId: 0,
		Limit:         50,
	})
	if err != nil {
		fmt.Printf("Error getting messages: %v\n", err)
		return
	}
	if len(resp.Messages) == 0 {
		fmt.Println("No messages found")
		return
	}
	fmt.Println("\n=== Messages ===")
	for _, msg := range resp.Messages {
		fmt.Printf("  ID=%d, UserID=%d, Likes=%d, Text='%s'\n", msg.Id, msg.UserId, msg.Likes, msg.Text)
	}
}

// Všečkajmo sporočilo
func likeMessage(client razpravljalnica.MessageBoardClient, user *razpravljalnica.User, topicIDStr, msgIDStr string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	topicID := parseInt64(topicIDStr)
	msgID := parseInt64(msgIDStr)
	if topicID == -1 || msgID == -1 {
		return
	}
	msg, err := client.LikeMessage(ctx, &razpravljalnica.LikeMessageRequest{
		TopicId:   topicID,
		MessageId: msgID,
		UserId:    user.Id,
	})
	if err != nil {
		fmt.Printf("Error liking message: %v\n", err)
		return
	}
	fmt.Printf("Message ID=%d now has %d likes\n", msg.Id, msg.Likes)
}

// Posodobimo sporočilo
func updateMessage(client razpravljalnica.MessageBoardClient, user *razpravljalnica.User, topicIDStr, msgIDStr, newText string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	topicID := parseInt64(topicIDStr)
	msgID := parseInt64(msgIDStr)
	if topicID == -1 || msgID == -1 {
		return
	}
	msg, err := client.UpdateMessage(ctx, &razpravljalnica.UpdateMessageRequest{
		TopicId:   topicID,
		UserId:    user.Id,
		MessageId: msgID,
		Text:      newText,
	})
	if err != nil {
		fmt.Printf("Error updating message: %v\n", err)
		return
	}
	fmt.Printf("Updated message: ID=%d, New text='%s'\n", msg.Id, msg.Text)
}

// Izbrišemo sporočilo
func deleteMessage(client razpravljalnica.MessageBoardClient, user *razpravljalnica.User, topicIDStr, msgIDStr string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	topicID := parseInt64(topicIDStr)
	msgID := parseInt64(msgIDStr)
	if topicID == -1 || msgID == -1 {
		return
	}
	_, err := client.DeleteMessage(ctx, &razpravljalnica.DeleteMessageRequest{
		TopicId:   topicID,
		UserId:    user.Id,
		MessageId: msgID,
	})
	if err != nil {
		fmt.Printf("Error deleting message: %v\n", err)
		return
	}
	fmt.Printf("Deleted message ID=%d\n", msgID)
}

// Naročnina na dogodke iz teme; povežemo se direktno na sub node iz control plane
func subToTopic(client razpravljalnica.MessageBoardClient, user *razpravljalnica.User, topicIDs []int64, subNodeID int64, subNodeAddress string, reader *bufio.Reader) {

	for topicID := range topicIDs {
		fmt.Printf("Subscribing to topic ID=%d via %d at %s...\n", topicID, subNodeID, subNodeAddress)
	}

	fmt.Println("Waiting for events (press Enter to stop)...")

	subCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stream, err := client.SubscribeTopic(subCtx, &razpravljalnica.SubscribeTopicRequest{
		TopicId:       topicIDs,
		UserId:        user.Id,
		FromMessageId: 0,
	})
	if err != nil {
		fmt.Printf("Error subscribing: %v\n", err)
		return
	}

	exitChan := make(chan bool, 1)

	// Gorutina bere vhod uporabnika
	go func() {

		_, err := reader.ReadString('\n')
		if err != nil {

			exitChan <- true
			return
		}
		// Vsak input sproži izhod
		exitChan <- true
	}()

	// Kanali za dogodke
	eventChan := make(chan *razpravljalnica.MessageEvent, 1)
	errChan := make(chan error, 1)

	// Go rutina sprejema dogodke
	go func() {
		for {
			event, err := stream.Recv()
			if err != nil {
				errChan <- err
				return
			}
			select {
			case eventChan <- event:
			case <-subCtx.Done():
				return
			}
		}
	}()

	for {
		select {
		case <-exitChan:
			// uporabnik je pritisnil enter
			cancel()

		case <-subCtx.Done():
			return
		case event := <-eventChan:
			opTypeStr := "UNKNOWN"
			switch event.Op {
			case razpravljalnica.OpType_OP_POST:
				opTypeStr = "POST"
			case razpravljalnica.OpType_OP_UPDATE:
				opTypeStr = "UPDATE"
			case razpravljalnica.OpType_OP_DELETE:
				opTypeStr = "DELETE"
			case razpravljalnica.OpType_OP_LIKE:
				opTypeStr = "LIKE"
			}

			fmt.Printf("[Event #%d] %s - Message Topic=%d ID=%d, UserID=%d, Likes=%d, Text='%s'\n",
				event.SequenceNumber, opTypeStr, event.Message.TopicId, event.Message.Id, event.Message.UserId, event.Message.Likes, event.Message.Text)

		case err := <-errChan:
			if err != nil {
				fmt.Printf("Stream closed or error: %v\n", err)
				return
			}
		}

	}
}

// Pridobimo stanje clustra
/*func clusterState(client controlPlane.ControlPlaneClient) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	state, err := client.GetClusterState(ctx, &emptypb.Empty{})
	if err != nil {
		fmt.Printf("Error getting cluster state: %v\n", err)
		return
	}
	fmt.Println("\n=== Cluster State ===")
	fmt.Printf("   Head: %d at %s\n", state.Head.NodeId, state.Head.Address)
	fmt.Printf("   Tail: %d at %s\n", state.Tail.NodeId, state.Tail.Address)
	fmt.Printf("   Sub: %d at %s\n", state.Sub.NodeId, state.Sub.Address)
	fmt.Println()
}*/

// Pomožna funkcija za pretvorbo stringa v int64 z obravnavo napak
func parseInt64(s string) int64 {
	var val int64
	_, err := fmt.Sscanf(s, "%d", &val)
	if err != nil {
		fmt.Printf("Invalid number: %s\n", s)
		return -1
	}
	return val
}
