package main

import (
	"sync"

	"github.com/gorilla/websocket"
	"github.com/pion/webrtc/v3"
)

var (
	RoomsLock sync.RWMutex
	Rooms     map[string]*Room = make(map[string]*Room)

	Upgrader = websocket.Upgrader{}
)

type websocketMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

// WEBRTC stuff
type PeerConnectionState struct {
	PeerConnection *webrtc.PeerConnection // a webrtc peer connection
	Websocket      *ThreadSafeWriter
}

// write safely over ws
type ThreadSafeWriter struct {
	Conn  *websocket.Conn // a websocket connection
	Mutex sync.Mutex
}

func (t *ThreadSafeWriter) WriteJSON(v interface{}) error {
	t.Mutex.Lock()
	defer t.Mutex.Unlock()
	return t.Conn.WriteJSON(v)
}
