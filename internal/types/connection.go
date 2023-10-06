package types

import (
	"context"
	"nhooyr.io/websocket"
	"time"
)

type PeerConnection struct {
	PeerId              string
	WenSocketConnection *websocket.Conn
	Headers             map[string]string
	Ctx                 context.Context
	ConnectedAt         time.Time
	RemoteIp            string
	ClientIp            string
}
