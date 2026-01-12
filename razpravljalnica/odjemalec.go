// Komunikacija po protokolu gRPC
// odjemalec za Razpravljalnico

package main

import (
	controlPlane "api/razpravljalnica/protobufControlPlane"
	razpravljalnica "api/razpravljalnica/protobufStorage"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/emptypb"
)

// Client demonstrira uporabo vseh operacij Razpravljalnice
func Client(controlURL string) {
	// Vzpostavimo povezavo s strežnikom
	fmt.Printf("gRPC client connecting to %v\n", controlURL)
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

	// Vzpostavimo izvajalno okolje
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*30)
	defer cancel()

	// Demonstracija delovanja Razpravljalnice
	fmt.Println("\n=== Razpravljalnica Demo ===")

	// 2. Ustvarimo uporabnike
	fmt.Println("2. Creating users...")
	user1, err := headClient.CreateUser(ctx, &razpravljalnica.CreateUserRequest{Name: "Alice"})
	if err != nil {
		panic(fmt.Sprintf("Failed to create user1: %v", err))
	}
	fmt.Printf("   Created user: ID=%d, Name=%s, Token=%s\n", user1.Id, user1.Name, user1.Token)

	user2, err := headClient.CreateUser(ctx, &razpravljalnica.CreateUserRequest{Name: "Bob"})
	if err != nil {
		panic(fmt.Sprintf("Failed to create user2: %v", err))
	}
	fmt.Printf("   Created user: ID=%d, Name=%s, Token=%s\n", user2.Id, user2.Name, user2.Token)
	fmt.Println()

	// 3. Ustvarimo teme
	fmt.Println("3. Creating topics...")
	topic1, err := headClient.CreateTopic(ctx, &razpravljalnica.CreateTopicRequest{Name: "Go Programming"})
	if err != nil {
		panic(fmt.Sprintf("Failed to create topic1: %v", err))
	}
	fmt.Printf("   Created topic: ID=%d, Name=%s\n", topic1.Id, topic1.Name)

	topic2, err := headClient.CreateTopic(ctx, &razpravljalnica.CreateTopicRequest{Name: "Distributed Systems"})
	if err != nil {
		panic(fmt.Sprintf("Failed to create topic2: %v", err))
	}
	fmt.Printf("   Created topic: ID=%d, Name=%s\n", topic2.Id, topic2.Name)
	fmt.Println()

	// 4. Pridobimo vse teme
	fmt.Println("4. Listing all topics...")
	topicsResp, err := headClient.ListTopics(ctx, &emptypb.Empty{})
	if err != nil {
		panic(fmt.Sprintf("Failed to list topics: %v", err))
	}
	fmt.Printf("   Found %d topics:\n", len(topicsResp.Topics))
	for _, topic := range topicsResp.Topics {
		fmt.Printf("     - ID=%d, Name=%s\n", topic.Id, topic.Name)
	}
	fmt.Println()

	// 5. Objavimo sporočila
	fmt.Println("5. Posting messages...")
	msg1, err := headClient.PostMessage(ctx, &razpravljalnica.PostMessageRequest{
		TopicId: topic1.Id,
		UserId:  user1.Id,
		Text:    "Go is a great language for concurrent programming!",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to post message1: %v", err))
	}
	fmt.Printf("   Posted message: ID=%d, Text='%s'\n", msg1.Id, msg1.Text)

	msg2, err := headClient.PostMessage(ctx, &razpravljalnica.PostMessageRequest{
		TopicId: topic1.Id,
		UserId:  user2.Id,
		Text:    "I agree! The goroutines make it very powerful.",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to post message2: %v", err))
	}
	fmt.Printf("   Posted message: ID=%d, Text='%s'\n", msg2.Id, msg2.Text)

	msg3, err := headClient.PostMessage(ctx, &razpravljalnica.PostMessageRequest{
		TopicId: topic2.Id,
		UserId:  user1.Id,
		Text:    "Chain replication is an interesting approach to consistency.",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to post message3: %v", err))
	}
	fmt.Printf("   Posted message: ID=%d, Text='%s'\n", msg3.Id, msg3.Text)
	fmt.Println()

	// 6. Pridobimo sporočila iz teme
	fmt.Println("6. Getting messages from topic...")
	messagesResp, err := tailClient.GetMessages(ctx, &razpravljalnica.GetMessagesRequest{
		TopicId:       topic1.Id,
		FromMessageId: 0,
		Limit:         10,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to get messages: %v", err))
	}
	fmt.Printf("   Found %d messages in topic '%s':\n", len(messagesResp.Messages), topic1.Name)
	for _, msg := range messagesResp.Messages {
		fmt.Printf("     - ID=%d, UserID=%d, Likes=%d, Text='%s'\n",
			msg.Id, msg.UserId, msg.Likes, msg.Text)
	}
	fmt.Println()

	// 7. Všečkajmo sporočilo
	fmt.Println("7. Liking messages...")
	likedMsg, err := headClient.LikeMessage(ctx, &razpravljalnica.LikeMessageRequest{
		TopicId:   topic1.Id,
		MessageId: msg1.Id,
		UserId:    user2.Id,
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to like message: %v", err))
	}
	fmt.Printf("   Message ID=%d now has %d likes\n", likedMsg.Id, likedMsg.Likes)
	fmt.Println()

	// 8. Posodobimo sporočilo
	fmt.Println("8. Updating message...")
	updatedMsg, err := headClient.UpdateMessage(ctx, &razpravljalnica.UpdateMessageRequest{
		TopicId:   topic1.Id,
		UserId:    user1.Id,
		MessageId: msg1.Id,
		Text:      "Go is an excellent language for concurrent programming!",
	})
	if err != nil {
		panic(fmt.Sprintf("Failed to update message: %v", err))
	}
	fmt.Printf("   Updated message: ID=%d, New text='%s'\n", updatedMsg.Id, updatedMsg.Text)
	fmt.Println()

	// 9. Naročnina na dogodke (v ločeni gorutini)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		fmt.Println("9. Subscribing to topic events...")

		// Vzpostavimo naročnino (uporabimo isti strežnik za enostavnost)
		subscribeCtx, subscribeCancel := context.WithCancel(context.Background())
		defer subscribeCancel()

		stream, err := subClient.SubscribeTopic(subscribeCtx, &razpravljalnica.SubscribeTopicRequest{
			TopicId:       []int64{topic1.Id, topic2.Id},
			UserId:        user1.Id,
			FromMessageId: 0,
			// SubscribeToken: initialState.Sub.SubscribeToken,
		})
		if err != nil {
			fmt.Printf("   Error subscribing: %v\n", err)
			return
		}

		fmt.Println("   Subscribed! Waiting for events...")
		eventCount := 0
		for {
			event, err := stream.Recv()
			if err == io.EOF {
				fmt.Println("   Stream closed")
				return
			}
			if err != nil {
				fmt.Printf("   Error receiving event: %v\n", err)
				return
			}

			eventCount++
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

			fmt.Printf("   [Event #%d] %s - Message ID=%d, Text='%s', Likes=%d\n",
				event.SequenceNumber, opTypeStr, event.Message.Id, event.Message.Text, event.Message.Likes)

			// Po 5 dogodkih prekinemo
			if eventCount >= 5 {
				subscribeCancel()
				break
			}
		}
	}()

	// Počakamo malo, da se naročnina vzpostavi
	time.Sleep(500 * time.Millisecond)

	// Medtem ko je naročnina aktivna, dodajamo nova sporočila
	fmt.Println("10. Posting more messages while subscribed...")
	time.Sleep(1 * time.Second)

	msg4, err := headClient.PostMessage(ctx, &razpravljalnica.PostMessageRequest{
		TopicId: topic1.Id,
		UserId:  user2.Id,
		Text:    "Let's discuss channels and select statements!",
	})
	if err != nil {
		fmt.Printf("   Error posting message: %v\n", err)
	} else {
		fmt.Printf("   Posted message: ID=%d\n", msg4.Id)
	}

	time.Sleep(1 * time.Second)

	// Všečkajmo še eno sporočilo
	_, err = headClient.LikeMessage(ctx, &razpravljalnica.LikeMessageRequest{
		TopicId:   topic1.Id,
		MessageId: msg2.Id,
		UserId:    user1.Id,
	})
	if err != nil {
		fmt.Printf("   Error liking message: %v\n", err)
	} else {
		fmt.Println("   Liked message")
	}

	time.Sleep(2 * time.Second)

	// 11. Izbrišemo sporočilo
	fmt.Println("\n11. Deleting message...")
	_, err = headClient.DeleteMessage(ctx, &razpravljalnica.DeleteMessageRequest{
		TopicId:   topic1.Id,
		UserId:    user2.Id,
		MessageId: msg4.Id,
	})
	if err != nil {
		fmt.Printf("   Error deleting message: %v\n", err)
	} else {
		fmt.Printf("   Deleted message ID=%d\n", msg4.Id)
	}

	// Počakamo, da se gorutina za naročnino zaključi
	time.Sleep(2 * time.Second)
	wg.Wait()

	fmt.Println("\n=== Demo completed ===")
}
