// Komunikacija po protokolu gRPC
// strežnik za Razpravljalnico

package main

import (
	"api/storage"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	controlPlane "api/razpravljalnica/protobufControlPlane"
	dataPlane "api/razpravljalnica/protobufDataPlane"
	razpravljalnica "api/razpravljalnica/protobufStorage"
)

// messageBoardServer implementira MessageBoard service
type messageBoardServer struct {
	razpravljalnica.UnimplementedMessageBoardServer
	store *storage.DiscussionBoardStorage
}

// NewMessageBoardServer ustvari nov strežnik za MessageBoard
func NewMessageBoardServer() *messageBoardServer {
	return &messageBoardServer{
		store: storage.NewDiscussionBoardStorage(),
	}
}

// CreateUser ustvari novega uporabnika
func (s *messageBoardServer) CreateUser(ctx context.Context, req *razpravljalnica.CreateUserRequest) (*razpravljalnica.User, error) {
	user, err := s.store.CreateUser(req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &razpravljalnica.User{
		Id:   user.ID,
		Name: user.Name,
	}, nil
}

// CreateTopic ustvari novo temo
func (s *messageBoardServer) CreateTopic(ctx context.Context, req *razpravljalnica.CreateTopicRequest) (*razpravljalnica.Topic, error) {
	topic, err := s.store.CreateTopic(req.Name)
	if err != nil {
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &razpravljalnica.Topic{
		Id:   topic.ID,
		Name: topic.Name,
	}, nil
}

// PostMessage doda novo sporočilo v temo
func (s *messageBoardServer) PostMessage(ctx context.Context, req *razpravljalnica.PostMessageRequest) (*razpravljalnica.Message, error) {
	msg, err := s.store.PostMessage(req.TopicId, req.UserId, req.Text)
	if err != nil {
		if err == storage.ErrorUserNotFound || err == storage.ErrorTopicNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return storageMessageToProto(msg), nil
}

// UpdateMessage posodobi obstoječe sporočilo
func (s *messageBoardServer) UpdateMessage(ctx context.Context, req *razpravljalnica.UpdateMessageRequest) (*razpravljalnica.Message, error) {
	msg, err := s.store.UpdateMessage(req.TopicId, req.UserId, req.MessageId, req.Text)
	if err != nil {
		if err == storage.ErrorMessageNotFound || err == storage.ErrorTopicNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err == storage.ErrorUnauthorized {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return storageMessageToProto(msg), nil
}

// DeleteMessage izbriše sporočilo
func (s *messageBoardServer) DeleteMessage(ctx context.Context, req *razpravljalnica.DeleteMessageRequest) (*emptypb.Empty, error) {
	err := s.store.DeleteMessage(req.TopicId, req.UserId, req.MessageId)
	if err != nil {
		if err == storage.ErrorMessageNotFound || err == storage.ErrorTopicNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		if err == storage.ErrorUnauthorized {
			return nil, status.Error(codes.PermissionDenied, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	return &emptypb.Empty{}, nil
}

// LikeMessage doda všeček sporočilu
func (s *messageBoardServer) LikeMessage(ctx context.Context, req *razpravljalnica.LikeMessageRequest) (*razpravljalnica.Message, error) {
	msg, err := s.store.LikeMessage(req.TopicId, req.MessageId, req.UserId)
	if err != nil {
		if err == storage.ErrorMessageNotFound || err == storage.ErrorTopicNotFound || err == storage.ErrorUserNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}

		return nil, status.Error(codes.Internal, err.Error())
	}

	return storageMessageToProto(msg), nil
}

// ListTopics vrne vse teme
func (s *messageBoardServer) ListTopics(ctx context.Context, req *emptypb.Empty) (*razpravljalnica.ListTopicsResponse, error) {
	topics := s.store.GetAllTopics()

	pbTopics := make([]*razpravljalnica.Topic, 0, len(topics))
	for _, topic := range topics {
		pbTopics = append(pbTopics, &razpravljalnica.Topic{
			Id:   topic.ID,
			Name: topic.Name,
		})
	}

	return &razpravljalnica.ListTopicsResponse{
		Topics: pbTopics,
	}, nil
}

// GetMessages vrne sporočila v temi
func (s *messageBoardServer) GetMessages(ctx context.Context, req *razpravljalnica.GetMessagesRequest) (*razpravljalnica.GetMessagesResponse, error) {
	messages, err := s.store.GetMessages(req.TopicId, req.FromMessageId, req.Limit)
	if err != nil {
		if err == storage.ErrorTopicNotFound {
			return nil, status.Error(codes.NotFound, err.Error())
		}
		return nil, status.Error(codes.Internal, err.Error())
	}

	pbMessages := make([]*razpravljalnica.Message, 0, len(messages))
	for _, msg := range messages {
		pbMessages = append(pbMessages, storageMessageToProto(msg))
	}

	return &razpravljalnica.GetMessagesResponse{
		Messages: pbMessages,
	}, nil
}

// GetSubscriptionNode vrne vozlišče, na katerega se lahko odpre naročnina
// Za enostavno implementacijo (ocena 6-7) vedno vrnemo trenutni strežnik
// client dobi subnode pri klicu getClusterState iz contol plane
/*
func (s *messageBoardServer) GetSubscriptionNode(ctx context.Context, req *razpravljalnica.SubscriptionNodeRequest) (*razpravljalnica.SubscriptionNodeResponse, error) {
	// Generiramo token za naročnino
	token, err := generateToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate subscription token")
	}

	// Za enostavno implementacijo vrnemo trenutni strežnik
	// V implementaciji z verižno replikacijo bi tukaj izbrali vozlišče glede na uravnoteženje obremenitve
	hostname, _ := os.Hostname()
	nodeInfo := &controlPlane.NodeInfo{
		NodeId:  hostname,
		Address: "localhost:50051", // To bo konfiguracijski parameter
	}

	return &razpravljalnica.SubscriptionNodeResponse{
		SubscribeToken: token,
		Node: &razpravljalnica.NodeInfo{
			NodeId:  nodeInfo.NodeId,
			Address: nodeInfo.Address,
		},
	}, nil
}
*/

// SubscribeTopic omogoča naročnino na dogodke v temah
func (s *messageBoardServer) SubscribeTopic(req *razpravljalnica.SubscribeTopicRequest, stream razpravljalnica.MessageBoard_SubscribeTopicServer) error {
	// Preverimo token (v pravi implementaciji bi to shranili in preverili)
	// Za enostavno implementacijo to preskočimo

	// Ustvarimo naročnino
	eventChan := s.store.Subscribe(req.UserId, req.TopicId, req.FromMessageId)

	// Pošiljamo dogodke odjemalcu
	for {
		select {
		case event, ok := <-eventChan:
			if !ok {
				// Kanal je zaprt
				return nil
			}

			// Preverimo, ali je dogodek v eni od naročenih tem
			// (v naši implementaciji Subscribe že filtrira, vendar preverimo še enkrat)
			isSubscribed := false
			for _, topicID := range req.TopicId {
				if event.Message.TopicID == topicID {
					isSubscribed = true
					break
				}
			}

			if !isSubscribed {
				continue
			}

			// Pretvorimo v protobuf MessageEvent
			pbEvent := storageEventToProto(&event)

			// Pošljemo odjemalcu
			if err := stream.Send(pbEvent); err != nil {
				return err
			}

		case <-stream.Context().Done():
			// Odjemalec se je odklopil
			return nil
		}
	}
}

// storageMessageToProto pretvori storage.Message v protobuf.Message
func storageMessageToProto(msg *storage.Message) *razpravljalnica.Message {
	return &razpravljalnica.Message{
		Id:        msg.ID,
		TopicId:   msg.TopicID,
		UserId:    msg.UserID,
		Text:      msg.Text,
		CreatedAt: timestamppb.New(msg.CreatedAt),
		Likes:     msg.Likes,
	}
}

// storageEventToProto pretvori storage.MessageEvent v protobuf.MessageEvent
func storageEventToProto(event *storage.MessageEvent) *razpravljalnica.MessageEvent {
	// Pretvorimo OpType iz stringa v enum
	var opType razpravljalnica.OpType
	switch event.OpType {
	case "POST":
		opType = razpravljalnica.OpType_OP_POST
	case "UPDATE":
		opType = razpravljalnica.OpType_OP_UPDATE
	case "DELETE":
		opType = razpravljalnica.OpType_OP_DELETE
	case "LIKE":
		opType = razpravljalnica.OpType_OP_LIKE
	default:
		opType = razpravljalnica.OpType_OP_POST
	}

	return &razpravljalnica.MessageEvent{
		SequenceNumber: event.SequenceNumber,
		Op:             opType,
		Message:        storageMessageToProto(&event.Message),
		EventAt:        timestamppb.New(event.EventAt),
	}
}

// generateToken generira naključen token za naročnino
func generateToken() (string, error) {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// controlPlaneServer implementira ControlPlane service
type controlPlaneServer struct {
	controlPlane.UnimplementedControlPlaneServer
	nodes               map[int64]*NodeRegistration
	mu                  sync.RWMutex
	subscriptionCounter int64
}

type NodeRegistration struct {
	NodeId         int64
	Role           string
	ClientAddress  string
	ControlAddress string
	DataAddress    string
	Position       int64
}

// NewControlPlaneServer ustvari nov strežnik za ControlPlane
func NewControlPlaneServer() *controlPlaneServer {
	return &controlPlaneServer{
		nodes: make(map[int64]*NodeRegistration),
	}
}

// GetClusterState vrne stanje klastra (head, tail in sub vozlišča)
func (s *controlPlaneServer) GetClusterState(ctx context.Context, req *emptypb.Empty) (*controlPlane.GetClusterStateResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	numNodes := int64(len(s.nodes))
	if numNodes == 0 {
		return nil, status.Error(codes.NotFound, "no nodes registered")
	}
	// Izberemo sub vozlišče na podlagi števila naročnin (round-robin)
	subIndex := s.subscriptionCounter % numNodes
	s.subscriptionCounter++
	// Najdemo head, tail in sub vozlišče
	var head, tail, sub *NodeRegistration
	for _, node := range s.nodes {
		if node.Position == 0 {
			head = node
		}
		if node.Position == numNodes-1 {
			tail = node
		}
		if node.Position == subIndex {
			sub = node
		}
	}
	Headresponse := &controlPlane.NodeInfo{
		NodeId:  head.NodeId,
		Address: head.ClientAddress,
	}
	Tailresponse := &controlPlane.NodeInfo{
		NodeId:  tail.NodeId,
		Address: tail.ClientAddress,
	}
	Subresponse := &controlPlane.NodeInfo{
		NodeId:  sub.NodeId,
		Address: sub.ClientAddress,
		// Generiramo token za naročnino
		// SubscribeToken: generateToken(),
	}

	return &controlPlane.GetClusterStateResponse{
		Head: Headresponse,
		Tail: Tailresponse,
		Sub:  Subresponse,
	}, nil
}

func (s *controlPlaneServer) RegisterNode(ctx context.Context, req *controlPlane.RegisterNodeRequest) (*controlPlane.RegisterNodeResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	numNodes := int64(len(s.nodes))
	position := numNodes // Dodamo na koncu

	// Samo en head, zato preverimo če že obstaja
	if req.Role == "head" {
		for _, n := range s.nodes {
			if n.Role == "head" {
				return nil, status.Error(codes.AlreadyExists, "head node already registered")
			}
		}
		// če še ne obstaja, dobi pozicijo 0
		position = 0
		// ostale node premaknemo za eno pozicijo naprej
		for _, n := range s.nodes {
			n.Position++
		}
	}

	// Samo en tail, zato preverimo če že obstaja
	if req.Role == "tail" {
		for _, n := range s.nodes {
			if n.Role == "tail" {
				return nil, status.Error(codes.AlreadyExists, "tail node already registered")
			}
		}
		// Tail dobi končno pozicijo
		position = numNodes
	}

	// Če dodajamo chain node, ga damo pred tail in premaknemo tail naprej
	if req.Role == "chain" && numNodes > 0 {
		lastPos := numNodes - 1
		for _, n := range s.nodes {
			if n.Position == lastPos && n.Role == "tail" {
				// tail premaknemo naprej
				n.Position++
				position = lastPos
				break
			}
		}
	}

	s.nodes[req.NodeId] = &NodeRegistration{
		NodeId:         req.NodeId,
		Role:           req.Role,
		ClientAddress:  req.ClientAddress,
		ControlAddress: req.ControlAddress,
		DataAddress:    req.DataAddress,
		Position:       position,
	}
	return &controlPlane.RegisterNodeResponse{
		AssignedPosition: position,
		Success:          true,
	}, nil
}

func (s *controlPlaneServer) GetNextNode(ctx context.Context, req *controlPlane.GetNextNodeRequest) (*controlPlane.NodeInfo, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	// Poiščemo vozlišče z danim NodeId
	node := s.nodes[req.NodeId]
	if node == nil {
		return nil, status.Error(codes.NotFound, "node not found")
	}
	// Poiščemo naslednje vozlišče v verigi in ga tudi vrnemo
	nextPosition := node.Position + 1
	for _, n := range s.nodes {
		if n.Position == nextPosition {
			return &controlPlane.NodeInfo{
				NodeId:  n.NodeId,
				Address: n.DataAddress,
			}, nil
		}
	}
	return nil, status.Error(codes.NotFound, "next node not found")
}

// forwardUpdate posreduje posodobitveno operacijo naslednjemu vozlišču v verigi
// Kliče se samo iz head vozlišč -> aka start the propagation
func (s *Server) forwardUpdate(op razpravljalnica.OpType, msg *razpravljalnica.Message, user *razpravljalnica.User, topic *razpravljalnica.Topic) error {
	s.nextNodeMutex.RLock()
	client := s.nextNodeClient
	s.nextNodeMutex.RUnlock()

	if client == nil {
		if s.role == "tail" {
			// Če smo tail, ni naslednjega vozlišča
			return nil
		}
		return fmt.Errorf("no next node client available")
	}

	// Pridobimo naslednjo zaporedno številko
	s.sequenceMutex.Lock()
	seqNum := s.sequenceNumber
	s.sequenceNumber++
	s.sequenceMutex.Unlock()

	// Ustvarimo zahtevo za posodobitev
	req := &dataPlane.UpdateRequest{
		SequenceNumber: seqNum,
		Op:             op,
		Message:        msg,
		User:           user,
		Topic:          topic,
	}

	// Pošljemo zahtevo naslednjemu vozlišču
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ack, err := client.ForwardUpdate(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to forward update to next node: %v", err)
	}
	if !ack.Success {
		return fmt.Errorf("next node reported failure in processing update")
	}

	// Log message based on operation type
	if msg != nil {
		fmt.Printf("[%s-%d] Forwarded %s operation for message %d to next node\n", s.role, s.NodeId, op.String(), msg.Id)
	} else if user != nil {
		fmt.Printf("[%s-%d] Forwarded %s operation for user %d to next node\n", s.role, s.NodeId, op.String(), user.Id)
	} else if topic != nil {
		fmt.Printf("[%s-%d] Forwarded %s operation for topic %d to next node\n", s.role, s.NodeId, op.String(), topic.Id)
	}
	return nil
}

// ForwardUpdate prejme posodobitveno operacijo od prejšnjega vozlišča in posodobi lokalno shrambo
// Potem posreduje naprej, če ni tail, in čaka na ack od naslednjega vozlišča (back propagation)
func (s *Server) ForwardUpdate(ctx context.Context, req *dataPlane.UpdateRequest) (*dataPlane.AcknowledgeResponse, error) {
	// Log message based on operation type
	if req.Message != nil {
		fmt.Printf("[%s-%d] Received forwarded %s operation for message %d\n", s.role, s.NodeId, req.Op.String(), req.Message.Id)
	} else if req.User != nil {
		fmt.Printf("[%s-%d] Received forwarded %s operation for user %d (%s)\n", s.role, s.NodeId, req.Op.String(), req.User.Id, req.User.Name)
	} else if req.Topic != nil {
		fmt.Printf("[%s-%d] Received forwarded %s operation for topic %d (%s)\n", s.role, s.NodeId, req.Op.String(), req.Topic.Id, req.Topic.Name)
	}

	// Najprej updatamo lokalno shrambo
	err := s.applyUpdate(req)
	if err != nil {
		fmt.Printf("[%s-%d] Failed to apply update: %v\n", s.role, s.NodeId, err)
		return &dataPlane.AcknowledgeResponse{
			SequenceNumber: req.SequenceNumber,
			Success:        false,
		}, err
	}

	// Če nismo tail posredujemo naprej in čakamo na ack
	if s.role != "tail" {
		s.nextNodeMutex.RLock()
		client := s.nextNodeClient
		s.nextNodeMutex.RUnlock()

		if client != nil {
			// pošljemo update msg naprej in čakamo na ack
			ackFromNext, err := client.ForwardUpdate(ctx, req)
			if err != nil {
				fmt.Printf("[%s-%d] Failed to forward update to next node: %v\n", s.role, s.NodeId, err)
				return &dataPlane.AcknowledgeResponse{
					SequenceNumber: req.SequenceNumber,
					Success:        false,
				}, err
			}
			// Če je naslednje vozlišče vrnilo napako, vrnemo napako
			if !ackFromNext.Success {
				fmt.Printf("[%s-%d] Next node failed to process update\n", s.role, s.NodeId)
				return &dataPlane.AcknowledgeResponse{
					SequenceNumber: req.SequenceNumber,
					Success:        false,
				}, nil
			}
			// Log based on operation type
			if req.Message != nil {
				fmt.Printf("[%s-%d] Forwarded %s operation for message %d to next node (ack received)\n", s.role, s.NodeId, req.Op.String(), req.Message.Id)
			} else if req.User != nil {
				fmt.Printf("[%s-%d] Forwarded %s operation for user %d to next node (ack received)\n", s.role, s.NodeId, req.Op.String(), req.User.Id)
			} else if req.Topic != nil {
				fmt.Printf("[%s-%d] Forwarded %s operation for topic %d to next node (ack received)\n", s.role, s.NodeId, req.Op.String(), req.Topic.Id)
			}
		}
	} else {
		// Tail je konec -> izpiše info na terminal in vrne ack
		if req.Message != nil {
			fmt.Printf("[%s-%d] Applied %s operation for message %d (tail - no forward)\n", s.role, s.NodeId, req.Op.String(), req.Message.Id)
		} else if req.User != nil {
			fmt.Printf("[%s-%d] Applied %s operation for user %d (tail - no forward)\n", s.role, s.NodeId, req.Op.String(), req.User.Id)
		} else if req.Topic != nil {
			fmt.Printf("[%s-%d] Applied %s operation for topic %d (tail - no forward)\n", s.role, s.NodeId, req.Op.String(), req.Topic.Id)
		}
	}

	return &dataPlane.AcknowledgeResponse{
		SequenceNumber: req.SequenceNumber,
		Success:        true,
	}, nil

}

// applyUpdate posodobi lokalno shrambo
func (s *Server) applyUpdate(req *dataPlane.UpdateRequest) error {
	msg := req.Message
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	switch req.Op {
	case razpravljalnica.OpType_OP_CREATE_USER:
		if req.User == nil {
			return fmt.Errorf("OP_CREATE_USER requires User field")
		}
		fmt.Printf("[%s-%d] Applying CREATE_USER for user %d (%s)\n", s.role, s.NodeId, req.User.Id, req.User.Name)
		// Ustvarimo uporabnika v lokalni shrambi z dodeljenim ID (za replikacijo)
		_, err := s.messageBoardServer.store.CreateUser(req.User.Name, req.User.Id)
		if err != nil {
			return fmt.Errorf("failed to create user: %v", err)
		}
		return nil

	case razpravljalnica.OpType_OP_CREATE_TOPIC:
		if req.Topic == nil {
			return fmt.Errorf("OP_CREATE_TOPIC requires Topic field")
		}
		fmt.Printf("[%s-%d] Applying CREATE_TOPIC for topic %d (%s)\n", s.role, s.NodeId, req.Topic.Id, req.Topic.Name)
		// Ustvarimo temo v lokalni shrambi z dodeljenim ID (za replikacijo)
		_, err := s.messageBoardServer.store.CreateTopic(req.Topic.Name, req.Topic.Id)
		if err != nil {
			return fmt.Errorf("failed to create topic: %v", err)
		}
		return nil

	case razpravljalnica.OpType_OP_POST:
		fmt.Printf("[%s-%d] Applying POST for message %d\n", s.role, s.NodeId, msg.Id)
		// Zapišemo spremembo v shrambo
		_, err := s.messageBoardServer.PostMessage(ctx, &razpravljalnica.PostMessageRequest{
			TopicId: msg.TopicId,
			UserId:  msg.UserId,
			Text:    msg.Text,
		})
		if err != nil {
			return fmt.Errorf("failed to post message: %v", err)
		}
		return nil

	case razpravljalnica.OpType_OP_UPDATE:
		fmt.Printf("[%s-%d] Applying UPDATE for message %d\n", s.role, s.NodeId, msg.Id)
		// Zapišemo spremembo v shrambo
		_, err := s.messageBoardServer.UpdateMessage(ctx, &razpravljalnica.UpdateMessageRequest{
			TopicId:   msg.TopicId,
			UserId:    msg.UserId,
			MessageId: msg.Id,
			Text:      msg.Text,
		})
		if err != nil {
			return fmt.Errorf("failed to update message: %v", err)
		}
		return nil

	case razpravljalnica.OpType_OP_DELETE:
		fmt.Printf("[%s-%d] Applying DELETE for message %d\n", s.role, s.NodeId, msg.Id)
		// Zapišemo spremembo v shrambo
		_, err := s.messageBoardServer.DeleteMessage(ctx, &razpravljalnica.DeleteMessageRequest{
			TopicId:   msg.TopicId,
			UserId:    msg.UserId,
			MessageId: msg.Id,
		})
		if err != nil {
			return fmt.Errorf("failed to delete message: %v", err)
		}
		return nil

	case razpravljalnica.OpType_OP_LIKE:
		fmt.Printf("[%s-%d] Applying LIKE for message %d\n", s.role, s.NodeId, msg.Id)
		// Zapišemo spremembo v shrambo
		_, err := s.messageBoardServer.LikeMessage(ctx, &razpravljalnica.LikeMessageRequest{
			TopicId:   msg.TopicId,
			MessageId: msg.Id,
			UserId:    msg.UserId,
		})
		if err != nil {
			return fmt.Errorf("failed to like message: %v", err)
		}
		return nil

	default:
		return fmt.Errorf("unknown operation type: %v", req.Op)
	}
}

// Storage functions that are overridden by streznik
// Te funkcije kliče samo head volišče, ki potem posreduje update naprej
// Client zahteva post na head, head updatira lokalno shrambo in potem posreduje naprej

func (s *Server) CreateUser(ctx context.Context, req *razpravljalnica.CreateUserRequest) (*razpravljalnica.User, error) {
	// Updatamo lokalno shrambo
	user, err := s.messageBoardServer.CreateUser(ctx, req)
	if err != nil {
		return nil, err
	}

	// Head posreduje update naprej
	if s.role == "head" {
		err = s.forwardUpdate(razpravljalnica.OpType_OP_CREATE_USER, nil, user, nil)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to forward create user update: %v", err))
		}
	}

	return user, nil
}

func (s *Server) CreateTopic(ctx context.Context, req *razpravljalnica.CreateTopicRequest) (*razpravljalnica.Topic, error) {
	// Updatamo lokalno shrambo
	topic, err := s.messageBoardServer.CreateTopic(ctx, req)
	if err != nil {
		return nil, err
	}

	// Head posreduje update naprej
	if s.role == "head" {
		err = s.forwardUpdate(razpravljalnica.OpType_OP_CREATE_TOPIC, nil, nil, topic)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to forward create topic update: %v", err))
		}
	}

	return topic, nil
}

func (s *Server) PostMessage(ctx context.Context, req *razpravljalnica.PostMessageRequest) (*razpravljalnica.Message, error) {
	// Updatamo lokalno shrambo
	msg, err := s.messageBoardServer.PostMessage(ctx, req)
	if err != nil {
		return nil, err
	}

	// Head posreduje update naprej
	if s.role == "head" {
		err = s.forwardUpdate(razpravljalnica.OpType_OP_POST, msg, nil, nil)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to forward post update: %v", err))
		}
	}

	return msg, nil
}

func (s *Server) UpdateMessage(ctx context.Context, req *razpravljalnica.UpdateMessageRequest) (*razpravljalnica.Message, error) {
	// Updatamo lokalno shrambo
	msg, err := s.messageBoardServer.UpdateMessage(ctx, req)
	if err != nil {
		return nil, err
	}

	// Head posreduje update naprej
	if s.role == "head" {
		err = s.forwardUpdate(razpravljalnica.OpType_OP_UPDATE, msg, nil, nil)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to forward update update: %v", err))
		}
	}

	return msg, nil
}

func (s *Server) DeleteMessage(ctx context.Context, req *razpravljalnica.DeleteMessageRequest) (*emptypb.Empty, error) {
	// Updatamo lokalno shrambo
	resp, err := s.messageBoardServer.DeleteMessage(ctx, req)
	if err != nil {
		return nil, err
	}

	// Head posreduje update naprej
	if s.role == "head" {
		// Ustvarimo message objekt za posredovanje
		msg := &razpravljalnica.Message{
			Id:      req.MessageId,
			TopicId: req.TopicId,
			UserId:  req.UserId,
		}
		err = s.forwardUpdate(razpravljalnica.OpType_OP_DELETE, msg, nil, nil)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to forward delete update: %v", err))
		}
	}

	return resp, nil
}

func (s *Server) LikeMessage(ctx context.Context, req *razpravljalnica.LikeMessageRequest) (*razpravljalnica.Message, error) {
	// Updatamo lokalno shrambo
	msg, err := s.messageBoardServer.LikeMessage(ctx, req)
	if err != nil {
		return nil, err
	}

	// Head posreduje update naprej
	if s.role == "head" {
		// Pri LIKE posredujemo ID uporabnika, ki je všečkal
		replicaMsg := &razpravljalnica.Message{
			Id:        msg.Id,
			TopicId:   msg.TopicId,
			UserId:    req.UserId,
			Text:      msg.Text,
			CreatedAt: msg.CreatedAt,
			Likes:     msg.Likes,
		}
		err = s.forwardUpdate(razpravljalnica.OpType_OP_LIKE, replicaMsg, nil, nil)
		if err != nil {
			return nil, status.Error(codes.Internal, fmt.Sprintf("failed to forward like update: %v", err))
		}
	}

	return msg, nil
}

// Server struct za podatkovno ravnino iz role izvemo ali je head, tail ali chain
type Server struct {
	*messageBoardServer
	dataPlane.UnimplementedDataPlaneServer

	role          string
	NodeId        int64
	clientAddress string
	dataAddress   string

	// Povezava na control plane
	controlPlaneAddress string
	controlPlaneClient  controlPlane.ControlPlaneClient
	controlConnection   *grpc.ClientConn
	// Povezava na naslednje vozlišče v verigi
	nextNodeClient  dataPlane.DataPlaneClient
	nextNodeAddress string
	nextNodeMutex   sync.RWMutex
	nextNodeConn    *grpc.ClientConn

	// Zaporedna številka sporočil za verižno replikacijo
	sequenceNumber int64
	sequenceMutex  sync.Mutex
}

// NewServer ustvari nov strežnik za podatkovno ravnino
func NewServer(role string, nodeId int64, clientAddress, dataAddress, controlPlaneAddress string) *Server {
	return &Server{
		messageBoardServer:  NewMessageBoardServer(),
		role:                role,
		NodeId:              nodeId,
		clientAddress:       clientAddress,
		dataAddress:         dataAddress,
		controlPlaneAddress: controlPlaneAddress,
		sequenceNumber:      1,
	}
}

// connectToControlPlane vzpostavi povezavo s control plane
func (s *Server) connectToControlPlane() error {
	conn, err := grpc.NewClient(s.controlPlaneAddress, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to control plane: %v", err)
	}
	s.controlConnection = conn
	s.controlPlaneClient = controlPlane.NewControlPlaneClient(conn)
	fmt.Printf("Connected to control plane at %s\n", s.controlPlaneAddress)
	return nil
}

// registerWithControlPlane registrira vozlišče pri control plane
func (s *Server) registerWithControlPlane() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := s.controlPlaneClient.RegisterNode(ctx, &controlPlane.RegisterNodeRequest{
		NodeId:         s.NodeId,
		Role:           s.role,
		ClientAddress:  s.clientAddress,
		ControlAddress: "",
		DataAddress:    s.dataAddress,
	})
	if err != nil {
		return fmt.Errorf("failed to register with control plane: %v", err)
	}
	fmt.Printf("[%s-%d] Registered at position %d\n", s.role, s.NodeId, resp.AssignedPosition)
	return nil
}

func (s *Server) discoverNextNode() error {
	// Ce je tail, ni naslednjega vozlišča
	if s.role == "tail" {
		fmt.Printf("[%s-%d] No next node (tail)\n", s.role, s.NodeId)
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Pridobimo informacije o naslednjem vozlišču iz control plane
	resp, err := s.controlPlaneClient.GetNextNode(ctx, &controlPlane.GetNextNodeRequest{
		NodeId: s.NodeId,
	})
	if err != nil {
		return fmt.Errorf("failed to get next node from control plane: %v", err)
	}
	// Vzpostavimo povezavo z naslednjim vozliščem
	conn, err := grpc.NewClient(resp.Address, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to next node at %s: %v", resp.Address, err)
	}

	s.nextNodeMutex.Lock()
	s.nextNodeAddress = resp.Address
	s.nextNodeConn = conn
	s.nextNodeClient = dataPlane.NewDataPlaneClient(conn)
	s.nextNodeMutex.Unlock()
	fmt.Printf("[%s-%d] Discovered next node at %s\n", s.role, s.NodeId, s.nextNodeAddress)
	return nil
}

func HeadServer(clientPort, controlPort, dataPort, serverControlPort string) {
	StartServer("head", clientPort, dataPort, serverControlPort)
}

func TailServer(clientPort, controlPort, dataPort, serverControlPort string) {
	StartServer("tail", clientPort, dataPort, serverControlPort)
}
func ChainServer(clientPort, controlPort, dataPort, serverControlPort string) {
	StartServer("chain", clientPort, dataPort, serverControlPort)
}

func ControlServer(clientControlPort, serverControlPort string) {
	// Pripravimo strežnik gRPC
	clientGrpc := grpc.NewServer()
	serverGrpc := grpc.NewServer()
	// Pripravimo strežnika za ControlPlane
	controlPlaneSrv := NewControlPlaneServer()

	// Registriramo storitve
	controlPlane.RegisterControlPlaneServer(clientGrpc, controlPlaneSrv)
	controlPlane.RegisterControlPlaneServer(serverGrpc, controlPlaneSrv)

	// Odpremo vtičnice
	go func() {
		listener, err := net.Listen("tcp", clientControlPort)
		if err != nil {
			panic(fmt.Sprintf("failed to listen on client port: %v", err))
		}
		fmt.Printf("Client control gRPC server listening at clientPort%s\n", clientControlPort)
		if err := clientGrpc.Serve(listener); err != nil {
			panic(fmt.Sprintf("failed to serve client gRPC: %v", err))
		}
	}()

	// Za strežnike
	go func() {
		listener, err := net.Listen("tcp", serverControlPort)
		if err != nil {
			panic(fmt.Sprintf("failed to listen on client port: %v", err))
		}
		fmt.Printf("Server control gRPC server listening at serverPort%s\n", serverControlPort)
		if err := clientGrpc.Serve(listener); err != nil {
			panic(fmt.Sprintf("failed to serve client gRPC: %v", err))
		}
	}()

	fmt.Println("Server startup successfull")
	// Blokirmo glavno nit da se ne konča
	select {}
}

// StartServer zažene strežnik z določeno vlogo in naslovi
func StartServer(role, clientPort, dataPort, controlPlaneAddress string) {
	// ID
	nodeID := time.Now().UnixNano()

	// Instanca serverja
	server := NewServer(role, nodeID, "localhost:"+clientPort, "localhost:"+dataPort, controlPlaneAddress)

	// Povežemo in registriramo se s control plane
	err := server.connectToControlPlane()
	if err != nil {
		panic(fmt.Sprintf("failed to connect to control plane: %v", err))
	}
	defer server.controlConnection.Close()

	err = server.registerWithControlPlane()
	if err != nil {
		panic(fmt.Sprintf("failed to register with control plane: %v", err))
	}

	// Odkrijemo naslednje vozlišče v verigi
	time.Sleep(5 * time.Second) // Počakamo malo, da se ostali nodi tudi registrirajo
	err = server.discoverNextNode()
	if err != nil {
		panic(fmt.Sprintf("failed to discover next node: %v", err))
	}

	// naredimo gRPC strežnike za clienta in dataPlane
	clientGrpc := grpc.NewServer()
	dataGrpc := grpc.NewServer()

	// Registriramo storitve
	razpravljalnica.RegisterMessageBoardServer(clientGrpc, server)
	dataPlane.RegisterDataPlaneServer(dataGrpc, server)

	// Odpremo vtičnice
	go func() {
		listener, err := net.Listen("tcp", ":"+clientPort)
		if err != nil {
			panic(fmt.Sprintf("failed to listen on client port: %v", err))
		}
		fmt.Printf("[%s-%d] Client gRPC server listening at clientPort:%s\n", role, nodeID, clientPort)
		if err := clientGrpc.Serve(listener); err != nil {
			panic(fmt.Sprintf("failed to serve client gRPC: %v", err))
		}
	}()

	// Data plane strežnik
	go func() {
		listener, err := net.Listen("tcp", ":"+dataPort)
		if err != nil {
			panic(fmt.Sprintf("failed to listen on data port: %v", err))
		}
		fmt.Printf("[%s-%d] Data gRPC server listening at dataPort:%s\n", role, nodeID, dataPort)
		if err := dataGrpc.Serve(listener); err != nil {
			panic(fmt.Sprintf("failed to serve data gRPC: %v", err))
		}
	}()

	fmt.Println("Server startup successfull")
	// Blokirmo glavno nit da se ne konča
	select {}

}
