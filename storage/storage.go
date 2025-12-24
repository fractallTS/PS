// paket storage
// 		enostavna shramba nalog, zgrajena kot slovar
//		strukturo Todo pri protokolu REST prenašamo v obliki sporočil JSON, zato pri opisu dodamo potrebne označbe
//		strukturi TodoStorage definiramo metode
//			odtis funkcije (argumenti, vrnjene vrednosti) je tak, da jezik go zna tvoriti oddaljene klice metod

package storage

import (
	"errors"
	"sync"
)

type Todo struct {
	Task      string `json:"task"`
	Completed bool   `json:"completed"`
}

type TodoEvent struct {
	Entry  Todo   `json:"entry"`
	Action string `json:"action"`
}

type TodoStorage struct {
	dict        map[string]Todo
	lock        sync.RWMutex
	subscribers []chan TodoEvent
	subLock     sync.RWMutex
}

var ErrorNotFound = errors.New("not found")

func NewTodoStorage() *TodoStorage {
	dict := make(map[string]Todo)
	return &TodoStorage{
		dict:        dict,
		subscribers: make([]chan TodoEvent, 0),
	}
}

// Subscribe returns a channel that receives all events
func (tds *TodoStorage) Subscribe() <-chan TodoEvent {
	tds.subLock.Lock()
	defer tds.subLock.Unlock()
	ch := make(chan TodoEvent, 100)
	tds.subscribers = append(tds.subscribers, ch)
	return ch
}

// broadcastEvent sends an event to all subscribers
func (tds *TodoStorage) broadcastEvent(event TodoEvent) {
	tds.subLock.RLock()
	defer tds.subLock.RUnlock()
	for _, ch := range tds.subscribers {
		select {
		case ch <- event:
		default:
			// Channel is full, skip this subscriber
		}
	}
}

func (tds *TodoStorage) Create(todo *Todo, ret *struct{}) error {
	tds.lock.Lock()
	tds.dict[todo.Task] = *todo
	tds.lock.Unlock()

	// Emit event
	tds.broadcastEvent(TodoEvent{
		Entry:  *todo,
		Action: "create",
	})
	return nil
}

func (tds *TodoStorage) Read(todo *Todo, dict *map[string]Todo) error {
	tds.lock.RLock()
	defer tds.lock.RUnlock()
	if todo.Task == "" {
		for k, v := range tds.dict {
			(*dict)[k] = v
		}
		return nil
	} else {
		if val, ok := tds.dict[todo.Task]; ok {
			(*dict)[val.Task] = val
			return nil
		}
		return ErrorNotFound
	}
}

func (tds *TodoStorage) Update(todo *Todo, ret *struct{}) error {
	tds.lock.Lock()
	exists := false
	if _, ok := tds.dict[todo.Task]; ok {
		tds.dict[todo.Task] = *todo
		exists = true
	}
	tds.lock.Unlock()

	if !exists {
		return ErrorNotFound
	}

	// Emit event
	tds.broadcastEvent(TodoEvent{
		Entry:  *todo,
		Action: "update",
	})
	return nil
}

func (tds *TodoStorage) Delete(todo *Todo, ret *struct{}) error {
	tds.lock.Lock()
	exists := false
	var deletedTodo Todo
	if val, ok := tds.dict[todo.Task]; ok {
		deletedTodo = val
		delete(tds.dict, todo.Task)
		exists = true
	}
	tds.lock.Unlock()

	if !exists {
		return ErrorNotFound
	}

	// Emit event
	tds.broadcastEvent(TodoEvent{
		Entry:  deletedTodo,
		Action: "delete",
	})
	return nil
}
