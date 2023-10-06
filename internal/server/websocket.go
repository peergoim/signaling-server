package server

import (
	"github.com/gin-gonic/gin"
	"github.com/peergoim/signaling-server/internal/handler"
	"github.com/peergoim/signaling-server/internal/middleware"
	"github.com/peergoim/signaling-server/internal/svc"
	"log"
)

type WebSocketServer struct {
	svcCtx *svc.ServiceContext
	engine *gin.Engine
}

func NewWebSocketServer(svcCtx *svc.ServiceContext) *WebSocketServer {
	w := &WebSocketServer{svcCtx: svcCtx}
	w.initGin()
	return w
}

func (w *WebSocketServer) initGin() {
	if w.svcCtx.Config.Mode == "pro" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}
	engine := gin.New()
	engine.Use(middleware.Logger(), middleware.Recovery(), middleware.Cors(w.svcCtx.Config.Cors), middleware.Tracer())
	// routes
	w.initRoutes(engine.Group(""))
	w.engine = engine
}

func (w *WebSocketServer) Start() {
	listenOn := w.svcCtx.Config.WebSocket.ListenOn
	log.Printf("websocket server start at %s\n", listenOn)
	err := w.engine.Run(listenOn)
	if err != nil {
		log.Fatalf("failed to start websocket server: %v", err)
	}
}

func (w *WebSocketServer) initRoutes(group *gin.RouterGroup) {
	h := handler.NewHandler(w.svcCtx)
	// "已注册的peer-A" 向 "已注册的peer-B" 发送request, 可以使用ws连接，也可以使用http接口
	group.GET("/ws", h.WsHandler)      // 需要被动接收消息的peer端，需要调用此接口，注册peer
	group.POST("/call", h.CallHandler) // 匿名peer，向"已注册的peer"发送request, "已注册的peer"返回response
}
