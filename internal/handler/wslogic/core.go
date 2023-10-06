package wslogic

import (
	"context"
	"github.com/peergoim/signaling-server/internal/svc"
	"github.com/peergoim/signaling-server/internal/types"
	"github.com/peergoim/signaling-server/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"nhooyr.io/websocket"
	"sync"
	"time"
)

type wsLogic struct {
	svcCtx              *svc.ServiceContext
	peerConnections     map[string][]*types.PeerConnection
	peerConnectionsLock sync.RWMutex
	callResponseChannel sync.Map
}

var Instance *wsLogic

func Init(svcCtx *svc.ServiceContext) {
	Instance = &wsLogic{
		svcCtx:          svcCtx,
		peerConnections: make(map[string][]*types.PeerConnection),
	}
}

func (l *wsLogic) OnCall(ctx context.Context, request *types.CallRequest) ([]byte, error) {
	var (
		callId = request.CallId
		peerId = request.PeerId

		peerConnections []*types.PeerConnection

		peerConnection *types.PeerConnection
	)
	// 1. 从peerConnections中获取peerId对应的所有连接
	{
		l.peerConnectionsLock.RLock()
		if v, ok := l.peerConnections[peerId]; !ok {
			l.peerConnectionsLock.RUnlock()
			return types.PeerOfflineResponse(request.CallId, request.Method).ToBytes(), types.PeerOfflineResponseError
		} else {
			peerConnections = v
			l.peerConnectionsLock.RUnlock()
		}
		if len(peerConnections) == 0 {
			return types.PeerOfflineResponse(request.CallId, request.Method).ToBytes(), types.PeerOfflineResponseError
		}
		// 真随机取一个peerConnection
		min := 0
		max := len(peerConnections)
		index := utils.RealRandInt(min, max)
		peerConnection = peerConnections[index]
	}
	// 创建一个响应channel，注册
	{
		// 1. 创建一个响应channel
		ch := make(chan *types.CallResponse, 1)
		// 2. 注册到peerConnection
		l.registerCallResponseChannel(callId, ch)
		// 3. 发送请求
		err := peerConnection.WenSocketConnection.Write(ctx, websocket.MessageBinary, request.ToBytes())
		if err != nil {
			return types.PeerOfflineResponse(request.CallId, request.Method).ToBytes(), types.PeerOfflineResponseError
		}
		// 4. 等待响应
		select {
		case <-time.After(time.Second * time.Duration(l.svcCtx.Config.WebSocket.CallTimeout)):
			// 超时
			l.unregisterCallResponseChannel(callId)
			return types.CallTimeoutResponse(request.CallId, request.Method).ToBytes(), types.CallTimeoutResponseError
		case resp := <-ch:
			// 收到响应
			l.unregisterCallResponseChannel(callId)
			return resp.ToBytes(), nil
		}
	}
}

func (l *wsLogic) AddSubscriber(conn *types.PeerConnection) {
	// peer端上线，加入到peerConnections
	l.peerConnectionsLock.Lock()
	peerId := conn.PeerId
	if _, ok := l.peerConnections[peerId]; !ok {
		l.peerConnections[peerId] = make([]*types.PeerConnection, 0)
	}
	l.peerConnections[peerId] = append(l.peerConnections[peerId], conn)
	l.peerConnectionsLock.Unlock()
}

func (l *wsLogic) DeleteSubscriber(conn *types.PeerConnection) {
	// peer端下线，从peerConnections删除
	l.peerConnectionsLock.Lock()
	if _, ok := l.peerConnections[conn.PeerId]; !ok {
		return
	}
	tmp := make([]*types.PeerConnection, 0)
	for _, c := range l.peerConnections[conn.PeerId] {
		if c != conn {
			tmp = append(tmp, c)
		} else {
			// 关闭websocket连接
			_ = c.WenSocketConnection.Close(websocket.StatusNormalClosure, "peer offline")
		}
	}
	if len(tmp) == 0 {
		delete(l.peerConnections, conn.PeerId)
	} else {
		l.peerConnections[conn.PeerId] = tmp
	}
	l.peerConnectionsLock.Unlock()
}

func (l *wsLogic) OnReply(ctx context.Context, response *types.CallResponse) {
	defer func() {
		if err := recover(); err != nil {
			logx.WithContext(ctx).Errorf("OnReply panic: %v", err)
		}
	}()
	// 1. 从callResponseChannel中获取响应channel
	{
		ch, ok := l.callResponseChannel.Load(response.CallId)
		if !ok {
			return
		}
		// 2. 发送响应
		ch.(chan *types.CallResponse) <- response
	}
}

func (l *wsLogic) registerCallResponseChannel(id string, ch chan *types.CallResponse) {
	l.callResponseChannel.Store(id, ch)
}

func (l *wsLogic) unregisterCallResponseChannel(id string) {
	l.callResponseChannel.Delete(id)
}
