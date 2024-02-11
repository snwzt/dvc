package main

import (
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/pion/webrtc/v3"
)

func HandleHome(c echo.Context) error {
	return c.Render(http.StatusOK, "home.html", nil)
}

func HandleCreate(c echo.Context) error {
	id := uuid.New().String()
	// sid := uuid.New().String()

	room := &Room{
		Peers: &Peers{
			TrackLocals: make(map[string]*webrtc.TrackLocalStaticRTP),
		},
		Hub: NewHub(),
	}

	Rooms[id] = room

	go room.Hub.Run()

	return c.Redirect(http.StatusFound, fmt.Sprintf("/room/%s", id))
}

func HandleRoom(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusNotFound, "room doesn't exist")
	}

	RoomsLock.RLock()
	_, ok := Rooms[id]
	RoomsLock.RUnlock()

	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "room doesn't exist")
	}

	RoomWSaddr := "wss://" + c.Request().Host + "/room/" + id + "/rws"

	return c.Render(http.StatusOK, "chat.html", map[string]interface{}{
		"TurnUrl":    TurnUrl,
		"TurnUser":   TurnUser,
		"TurnCred":   TurnCred,
		"RoomWSaddr": RoomWSaddr,
	})
}

func HandleRoomWS(c echo.Context) error {
	id := c.Param("id")
	if id == "" {
		return echo.NewHTTPError(http.StatusNotFound, "room doesn't exist")
	}

	ws, err := Upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return err
	}
	defer ws.Close()

	RoomsLock.RLock()
	room, ok := Rooms[id]
	RoomsLock.RUnlock()

	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "room doesn't exist")
	}

	RoomConn(ws, room.Peers)

	return nil
}
