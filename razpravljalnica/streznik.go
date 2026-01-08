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
	"os"
	"sync"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	controlPlane "api/razpravljalnica/protobufControlPlane"
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
		if err == storage.ErrorAlreadyLiked {
			return nil, status.Error(codes.AlreadyExists, err.Error())
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
func (s *messageBoardServer) GetSubscriptionNode(ctx context.Context, req *razpravljalnica.SubscriptionNodeRequest) (*razpravljalnica.SubscriptionNodeResponse, error) {
	// Generiramo token za naročnino
	token, err := generateToken()
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to generate subscription token")
	}

	// Za enostavno implementacijo vrnemo trenutni strežnik
	// V implementaciji z verižno replikacijo bi tukaj izbrali vozlišče glede na uravnoteženje obremenitve
	hostname, _ := os.Hostname()
	nodeInfo := &razpravljalnica.NodeInfo{
		NodeId:  hostname,
		Address: "localhost:50051", // To bo konfiguracijski parameter
	}

	return &razpravljalnica.SubscriptionNodeResponse{
		SubscribeToken: token,
		Node:           nodeInfo,
	}, nil
}

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
func NewControlPlaneServer(headAddress, tailAddress string) *controlPlaneServer {
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

	nodeID := int64(len(s.nodes))

	if req.Role == "head" && len(s.nodes) > 0 {
		if s.nodes[0].Role == "head" {
			return nil, status.Error(codes.AlreadyExists, "head node already registered")
		} else {
			s.nodes[0], s.nodes[nodeID] = s.nodes[nodeID], s.nodes[0] // swap head and node at position 0
			s.nodes[0].Position = 0
			s.nodes[nodeID].Position = nodeID
			nodeID = 0
		}
	} else if req.Role == "tail" && s.nodes[nodeID-1].Role == "tail" {
		return nil, status.Error(codes.AlreadyExists, "tail node already registered")
	} else if req.Role == "chain" && s.nodes[nodeID-1].Role == "tail" {
		s.nodes[nodeID-1], s.nodes[nodeID] = s.nodes[nodeID], s.nodes[nodeID-1] // swap chain and tail node at position nodeID-1
		s.nodes[nodeID-1].Position = nodeID - 1
		s.nodes[nodeID].Position = nodeID
		nodeID = nodeID - 1
	}

	s.nodes[nodeID] = &NodeRegistration{
		NodeId:         int64(len(s.nodes)),
		Role:           req.Role,
		ClientAddress:  req.ClientAddress,
		ControlAddress: req.ControlAddress,
		DataAddress:    req.DataAddress,
		Position:       nodeID,
	}
	return &controlPlane.RegisterNodeResponse{
		AssignedPosition: nodeID,
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

func HeadServer(clientPort, controlPort, dataPort, serverControlPort string) {
	role := "head"
	clientGrpc := grpc.NewServer()
	controlGrpc := grpc.NewServer()
	dataGrpc := grpc.NewServer()

	messageBoardSrv := NewMessageBoardServer()

}

func TailServer(clientPort, controlPort, dataPort, serverControlPort string) {
	role := "tail"
	clientGrpc := grpc.NewServer()
	controlGrpc := grpc.NewServer()
	dataGrpc := grpc.NewServer()
}

func ChainServer(clientPort, controlPort, dataPort, serverControlPort string) {
	role := "chain"
	clientGrpc := grpc.NewServer()
	controlGrpc := grpc.NewServer()
	dataGrpc := grpc.NewServer()
}

func ControlServer(clientControlPort, serverControlPort string) {
	// Pripravimo strežnik gRPC
	grpcServer := grpc.NewServer()
	// Pripravimo strežnika za ControlPlane
	controlPlaneSrv := NewControlPlaneServer(clientControlPort, clientControlPort) // Za enostavno implementacijo sta head in tail enaka

	// Registriramo storitve
	controlPlane.RegisterControlPlaneServer(grpcServer, controlPlaneSrv)

	// Odpremo vtičnico
	listener, err := net.Listen("tcp", serverControlPort)
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}

	hostname, _ := os.Hostname()
	fmt.Printf("Control gRPC server listening at %s:%s\n", hostname, serverControlPort)

	// Začnemo s streženjem
	if err := grpcServer.Serve(listener); err != nil {
		panic(fmt.Sprintf("failed to serve: %v", err))
	}

}

// StartServer zažene gRPC strežnik
func StartServer(address string) {
	// Pripravimo strežnik gRPC
	grpcServer := grpc.NewServer()

	// Pripravimo strežnika za MessageBoard in ControlPlane
	messageBoardSrv := NewMessageBoardServer()
	controlPlaneSrv := NewControlPlaneServer(address, address) // Za enostavno implementacijo sta head in tail enaka

	// Registriramo storitve
	razpravljalnica.RegisterMessageBoardServer(grpcServer, messageBoardSrv)
	controlPlane.RegisterControlPlaneServer(grpcServer, controlPlaneSrv)

	// Odpremo vtičnico
	listener, err := net.Listen("tcp", address)
	if err != nil {
		panic(fmt.Sprintf("failed to listen: %v", err))
	}

	hostname, _ := os.Hostname()
	fmt.Printf("gRPC server listening at %s:%s\n", hostname, address)

	// Začnemo s streženjem
	if err := grpcServer.Serve(listener); err != nil {
		panic(fmt.Sprintf("failed to serve: %v", err))
	}
}
