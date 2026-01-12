// paket storage
// 		enostavna shramba nalog, zgrajena kot slovar
//		strukturo Todo pri protokolu REST prenašamo v obliki sporočil JSON, zato pri opisu dodamo potrebne označbe
//		strukturi TodoStorage definiramo metode
//			odtis funkcije (argumenti, vrnjene vrednosti) je tak, da jezik go zna tvoriti oddaljene klice metod

package storage

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// User predstavlja uporabnika razpravljalnice
type User struct {
	ID    int64
	Name  string
	Token string // avtentikacijski token
}

// Topic predstavlja temo razprave
type Topic struct {
	ID   int64
	Name string
}

// Message predstavlja sporočilo v temi
type Message struct {
	ID        int64
	TopicID   int64
	UserID    int64
	Text      string
	CreatedAt time.Time
	Likes     int32
}

// Like predstavlja všeček sporočila
// Uporabljamo za sledenje, kdo je že všečkal sporočilo (da preprečimo dvojno všečkanje)
type Like struct {
	TopicID   int64
	MessageID int64
	UserID    int64 // uporabnik, ki je všečkal
}

// MessageEvent predstavlja dogodek, ki se zgodi z sporočilom (dodajanje, posodobitev, brisanje, všečkanje)
type MessageEvent struct {
	SequenceNumber int64
	OpType         string // "POST", "UPDATE", "DELETE", "LIKE"
	Message        Message
	EventAt        time.Time
}

// DiscussionBoardStorage je glavna struktura za shranjevanje vseh podatkov
type DiscussionBoardStorage struct {
	// Podatki
	users    map[int64]*User    // map[userID] -> User
	topics   map[int64]*Topic   // map[topicID] -> Topic
	messages map[int64]*Message // map[messageID] -> Message
	likes    map[string]*Like   // map["topicID:messageID:userID"] -> Like (za preprečevanje dvojnega všečkanja)

	// Indeksi za hitrejše iskanje
	messagesByTopic map[int64][]int64 // map[topicID] -> []messageID (sporočila v temi, urejena po ID)

	// Števci za ID-je
	nextUserID    int64
	nextTopicID   int64
	nextMessageID int64
	nextEventSeq  int64 // zaporedna številka dogodka za naročnine

	// Sinhronizacija
	mu sync.RWMutex

	// Naročnine na dogodke (za streaming)
	subscribers map[int64][]chan MessageEvent // map[userID] -> []subscription channels
	subMu       sync.RWMutex
}

var (
	ErrorUserNotFound    = errors.New("user not found")
	ErrorTopicNotFound   = errors.New("topic not found")
	ErrorMessageNotFound = errors.New("message not found")
	ErrorUnauthorized    = errors.New("unauthorized: user is not the owner of this message")
	ErrorAlreadyLiked    = errors.New("message already liked by this user")
)

// NewDiscussionBoardStorage ustvari novo shrambo za razpravljalnico
func NewDiscussionBoardStorage() *DiscussionBoardStorage {
	return &DiscussionBoardStorage{
		users:           make(map[int64]*User),
		topics:          make(map[int64]*Topic),
		messages:        make(map[int64]*Message),
		likes:           make(map[string]*Like),
		messagesByTopic: make(map[int64][]int64),
		nextUserID:      1,
		nextTopicID:     1,
		nextMessageID:   1,
		nextEventSeq:    1,
		subscribers:     make(map[int64][]chan MessageEvent),
	}
}

// generateToken generira naključen token za prijavo uporabnika
func generateToken() (string, error) {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateUser ustvari novega uporabnika in mu dodeli ID
// Če je userID > 0, se uporabi ta ID (za replikacijo), sicer se avtomatsko dodeli nov ID
func (s *DiscussionBoardStorage) CreateUser(name string, userID ...int64) (*User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var id int64
	if len(userID) > 0 && userID[0] > 0 {
		// Uporabimo dodeljen ID (za replikacijo)
		id = userID[0]
		// Preverimo, če uporabnik že obstaja (idempotentnost)
		if existing, exists := s.users[id]; exists {
			// Če že obstaja, samo posodobimo ime (za konsistentnost)
			existing.Name = name
			return existing, nil
		}
		// Posodobimo nextUserID če je potrebno
		if id >= s.nextUserID {
			s.nextUserID = id + 1
		}
	} else {
		// Avtomatsko dodelimo nov ID
		id = s.nextUserID
		s.nextUserID++
	}

	token, err := generateToken() // generira naključen token za prijavo uporabnika
	if err != nil {
		return nil, err
	}

	user := &User{
		ID:    id,
		Name:  name,
		Token: token,
	}
	s.users[user.ID] = user
	return user, nil
}

// GetUser vrne uporabnika po ID-ju
func (s *DiscussionBoardStorage) GetUser(userID int64) (*User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	user, ok := s.users[userID]
	if !ok {
		return nil, ErrorUserNotFound
	}
	return user, nil
}

// CreateTopic ustvari novo temo
// Če je topicID > 0, se uporabi ta ID (za replikacijo), sicer se avtomatsko dodeli nov ID
func (s *DiscussionBoardStorage) CreateTopic(name string, topicID ...int64) (*Topic, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var id int64
	if len(topicID) > 0 && topicID[0] > 0 {
		// Uporabimo dodeljen ID (za replikacijo)
		id = topicID[0]
		// Preverimo, če tema že obstaja (idempotentnost)
		if existing, exists := s.topics[id]; exists {
			// Če že obstaja, samo posodobimo ime (za konsistentnost)
			existing.Name = name
			return existing, nil
		}
		// Posodobimo nextTopicID če je potrebno
		if id >= s.nextTopicID {
			s.nextTopicID = id + 1
		}
	} else {
		// Avtomatsko dodelimo nov ID
		id = s.nextTopicID
		s.nextTopicID++
	}

	topic := &Topic{
		ID:   id,
		Name: name,
	}
	s.topics[topic.ID] = topic
	s.messagesByTopic[topic.ID] = make([]int64, 0)
	return topic, nil
}

// GetTopic vrne temo po ID-ju
func (s *DiscussionBoardStorage) GetTopic(topicID int64) (*Topic, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topic, ok := s.topics[topicID]
	if !ok {
		return nil, ErrorTopicNotFound
	}
	return topic, nil
}

// GetAllTopics vrne vse teme
func (s *DiscussionBoardStorage) GetAllTopics() []*Topic {
	s.mu.RLock()
	defer s.mu.RUnlock()

	topics := make([]*Topic, 0, len(s.topics))
	for _, topic := range s.topics {
		topics = append(topics, topic)
	}
	return topics
}

// PostMessage doda novo sporočilo v temo
// Preveri, da uporabnik in tema obstajata
func (s *DiscussionBoardStorage) PostMessage(topicID, userID int64, text string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Preverimo, da uporabnik obstaja
	if _, ok := s.users[userID]; !ok {
		return nil, ErrorUserNotFound
	}

	// Preverimo, da tema obstaja
	if _, ok := s.topics[topicID]; !ok {
		return nil, ErrorTopicNotFound
	}

	message := &Message{
		ID:        s.nextMessageID,
		TopicID:   topicID,
		UserID:    userID,
		Text:      text,
		CreatedAt: time.Now(),
		Likes:     0,
	}
	s.messages[message.ID] = message
	s.messagesByTopic[topicID] = append(s.messagesByTopic[topicID], message.ID)
	s.nextMessageID++

	// Pošljemo dogodek naročnikom
	s.broadcastEvent(MessageEvent{
		SequenceNumber: s.nextEventSeq,
		OpType:         "POST",
		Message:        *message,
		EventAt:        time.Now(),
	})
	s.nextEventSeq++

	return message, nil
}

// GetMessage vrne sporočilo po ID-ju
func (s *DiscussionBoardStorage) GetMessage(messageID int64) (*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	message, ok := s.messages[messageID]
	if !ok {
		return nil, ErrorMessageNotFound
	}
	return message, nil
}

// GetMessages vrne sporočila v temi
// fromMessageID: ID sporočila, od katerega začnemo (0 = od začetka)
// limit: največje število sporočil
func (s *DiscussionBoardStorage) GetMessages(topicID int64, fromMessageID int64, limit int32) ([]*Message, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Preverimo, da tema obstaja
	if _, ok := s.topics[topicID]; !ok {
		return nil, ErrorTopicNotFound
	}

	messageIDs := s.messagesByTopic[topicID]
	if len(messageIDs) == 0 {
		return []*Message{}, nil
	}

	// Najdemo začetni indeks
	startIdx := 0
	if fromMessageID > 0 {
		for i, id := range messageIDs {
			if id >= fromMessageID {
				startIdx = i
				break
			}
		}
	}

	// Vzamemo sporočila do limita
	messages := make([]*Message, 0)
	count := int32(0)
	for i := startIdx; i < len(messageIDs) && count < limit; i++ {
		if msg, ok := s.messages[messageIDs[i]]; ok {
			messages = append(messages, msg)
			count++
		}
	}

	return messages, nil
}

// UpdateMessage posodobi sporočilo
// Dovoljeno samo uporabniku, ki je sporočilo objavil
func (s *DiscussionBoardStorage) UpdateMessage(topicID, userID, messageID int64, newText string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	message, ok := s.messages[messageID]
	if !ok {
		return nil, ErrorMessageNotFound
	}

	// Preverimo, da je sporočilo v pravi temi
	if message.TopicID != topicID {
		return nil, ErrorTopicNotFound
	}

	// Preverimo avtorizacijo
	if message.UserID != userID {
		return nil, ErrorUnauthorized
	}

	// Posodobimo sporočilo
	message.Text = newText

	// Pošljemo dogodek naročnikom
	s.broadcastEvent(MessageEvent{
		SequenceNumber: s.nextEventSeq,
		OpType:         "UPDATE",
		Message:        *message,
		EventAt:        time.Now(),
	})
	s.nextEventSeq++

	return message, nil
}

// DeleteMessage izbriše sporočilo
// Dovoljeno samo uporabniku, ki je sporočilo objavil
func (s *DiscussionBoardStorage) DeleteMessage(topicID, userID, messageID int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	message, ok := s.messages[messageID]
	if !ok {
		return ErrorMessageNotFound
	}

	// Preverimo, da je sporočilo v pravi temi
	if message.TopicID != topicID {
		return ErrorTopicNotFound
	}

	// Preverimo avtorizacijo
	if message.UserID != userID {
		return ErrorUnauthorized
	}

	// Izbrišemo sporočilo iz mape
	delete(s.messages, messageID)

	// Odstranimo iz indeksa teme
	messageIDs := s.messagesByTopic[topicID]
	for i, id := range messageIDs {
		if id == messageID {
			s.messagesByTopic[topicID] = append(messageIDs[:i], messageIDs[i+1:]...)
			break
		}
	}

	// Pošljemo dogodek naročnikom
	s.broadcastEvent(MessageEvent{
		SequenceNumber: s.nextEventSeq,
		OpType:         "DELETE",
		Message:        *message,
		EventAt:        time.Now(),
	})
	s.nextEventSeq++

	return nil
}

// LikeMessage doda všeček sporočilu
// Prepreči dvojno všečkanje istega sporočila z istim uporabnikom
func (s *DiscussionBoardStorage) LikeMessage(topicID, messageID, userID int64) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	message, ok := s.messages[messageID]
	if !ok {
		return nil, ErrorMessageNotFound
	}

	// Preverimo, da je sporočilo v pravi temi
	if message.TopicID != topicID {
		return nil, ErrorTopicNotFound
	}

	// Preverimo, da uporabnik obstaja
	if _, ok := s.users[userID]; !ok {
		return nil, ErrorUserNotFound
	}

	// Če je uporabnik že všečkal sporočilo, všeček odstranimo
	likeKey := likeKey(topicID, messageID, userID)
	if _, ok := s.likes[likeKey]; ok {
		delete(s.likes, likeKey)
		message.Likes--
	} else {

		// Dodamo všeček
		like := &Like{
			TopicID:   topicID,
			MessageID: messageID,
			UserID:    userID,
		}
		s.likes[likeKey] = like
		message.Likes++
	}

	// Pošljemo dogodek naročnikom
	s.broadcastEvent(MessageEvent{
		SequenceNumber: s.nextEventSeq,
		OpType:         "LIKE",
		Message:        *message,
		EventAt:        time.Now(),
	})
	s.nextEventSeq++

	return message, nil
}

// Subscribe ustvari naročnino na dogodke za določene teme
// Vrne kanal, preko katerega se prejemajo dogodki
func (s *DiscussionBoardStorage) Subscribe(userID int64, topicIDs []int64, fromMessageID int64) <-chan MessageEvent {
	ch := make(chan MessageEvent, 100)

	s.subMu.Lock()
	s.subscribers[userID] = append(s.subscribers[userID], ch)
	s.subMu.Unlock()

	// Pošljemo zgodovinske dogodke (sporočila, ki so že objavljena)
	// To je poenostavljena verzija - v pravi implementaciji bi morali filtrirati po topicIDs
	go func() {
		s.mu.RLock()
		// Pošljemo vsa obstoječa sporočila v naročenih temah
		for _, topicID := range topicIDs {
			if messages, ok := s.messagesByTopic[topicID]; ok {
				for _, msgID := range messages {
					if msgID > fromMessageID {
						if msg, ok := s.messages[msgID]; ok {
							select {
							case ch <- MessageEvent{
								SequenceNumber: 0, // zgodovinski dogodki nimajo zaporedne številke
								OpType:         "POST",
								Message:        *msg,
								EventAt:        msg.CreatedAt,
							}:
							default:
							}
						}
					}
				}
			}
		}
		s.mu.RUnlock()
	}()

	return ch
}

// broadcastEvent pošlje dogodek vsem naročnikom
func (s *DiscussionBoardStorage) broadcastEvent(event MessageEvent) {
	s.subMu.RLock()
	defer s.subMu.RUnlock()

	for _, channels := range s.subscribers {
		for _, ch := range channels {
			select {
			case ch <- event:
			default:
				// Kanali so polni, preskočimo
			}
		}
	}
}

// likeKey ustvari ključ za mapo všečkov
func likeKey(topicID, messageID, userID int64) string {
	return fmt.Sprintf("%d:%d:%d", topicID, messageID, userID)
}
