package handler

import (
	"context"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/peergoim/signaling-server/internal/handler/wslogic"
	"github.com/peergoim/signaling-server/internal/types"
	"github.com/peergoim/signaling-server/internal/utils"
	"github.com/zeromicro/go-zero/core/logx"
	"github.com/zeromicro/go-zero/core/trace"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	semconv "go.opentelemetry.io/otel/semconv/v1.4.0"
	oteltrace "go.opentelemetry.io/otel/trace"
	"io"
	"nhooyr.io/websocket"
	"strings"
	"sync"
	"time"
)

var wsOnce = sync.Once{}

// WsHandler peer端调用此接口，升级为websocket连接，接收消息
func (h *Handler) WsHandler(ginContext *gin.Context) {
	wsOnce.Do(func() {
		wslogic.Init(h.svcCtx)
	})
	var (
		w = ginContext.Writer
		r = ginContext.Request
	)
	logger := logx.WithContext(r.Context())
	headers := make(map[string]string)
	for k, v := range r.Header {
		if len(v) > 0 {
			headers[k] = strings.Join(v, ",")
		}
	}
	peerId := ginContext.Query("peerId")
	clientIp := utils.GetClientIp(r)
	// 连接前的检测
	{
		if peerId == "" {
			logger.Errorf("peerId is empty")
			ginContext.Redirect(302, "https://www.google.com")
			return
		}
		if !h.svcCtx.Config.WebSocket.IpWhitelist.InIpWhitelist(clientIp) {
			logger.Errorf("ip %s not in whitelist", clientIp)
			// 直接重定向到google.com，防止其他人恶意访问
			ginContext.Redirect(302, "https://www.google.com")
			return
		}
	}
	compressionMode := websocket.CompressionNoContextTakeover
	// https://github.com/nhooyr/websocket/issues/218
	// 如果是Safari浏览器，不压缩
	if strings.Contains(r.UserAgent(), "Safari") {
		compressionMode = websocket.CompressionDisabled
	}
	c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		Subprotocols:         nil,
		InsecureSkipVerify:   true,
		OriginPatterns:       nil,
		CompressionMode:      compressionMode,
		CompressionThreshold: 0,
	})
	if err != nil {
		return
	}
	c.SetReadLimit(32768) // 32k 防止恶意攻击
	defer c.Close(websocket.StatusInternalError, "")
	ctx, cancelFunc := context.WithCancel(r.Context())
	peerConn := &types.PeerConnection{
		WenSocketConnection: c,
		Headers:             headers,
		Ctx:                 ctx,
		ConnectedAt:         time.Now(),
		RemoteIp:            r.RemoteAddr,
		ClientIp:            clientIp,
		PeerId:              peerId,
	}
	wsReturn := func(ctx context.Context, conn *types.PeerConnection, resp *types.CallResponse) {
		_ = conn.WenSocketConnection.Write(ctx, websocket.MessageBinary, resp.ToBytes())
	}
	loopRead := func(ctx context.Context, cancelFunc context.CancelFunc, conn *types.PeerConnection) {
		defer cancelFunc()
		for {
			logx.WithContext(ctx).Debugf("start read")
			typ, msg, err := conn.WenSocketConnection.Read(ctx)
			if err != nil {
				if errors.Is(err, io.EOF) {
					// 正常关闭
				} else if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
					websocket.CloseStatus(err) == websocket.StatusGoingAway {
					// 正常关闭
					logx.Infof("websocket closed: %v", err)
				} else if strings.Contains(err.Error(), "connection reset by peer") {
					// 网络断开
					logx.Infof("websocket closed: %v", err)
				} else if strings.Contains(err.Error(), "corrupt input") {
					// 输入数据错误
					logx.Infof("websocket closed: %v", err)
				} else {
					logx.Errorf("failed to read message: %v", err)
				}
				return
			}
			if typ == websocket.MessageText {
				// 响应
				logx.WithContext(ctx).Debugf("read message.length: %d", len(msg))
				if len(msg) == 0 {
					logx.WithContext(ctx).Errorf("invalid message length: %d", len(msg))
					return
				}
				response := &types.CallResponse{}
				err = response.FromBytes(msg)
				if err != nil {
					logx.WithContext(ctx).Errorf("failed to unmarshal response: %v", err)
					return
				}
				wslogic.Instance.OnReply(ctx, response)
			} else if typ == websocket.MessageBinary {
				// 请求
				logx.WithContext(ctx).Debugf("read message.length: %d", len(msg))
				if len(msg) == 0 {
					logx.WithContext(ctx).Errorf("invalid message length: %d", len(msg))
					return
				}
				request := &types.CallRequest{}
				err = request.FromBytes(msg)
				if err != nil {
					resp := types.RequestUnmarshalErrorResponse
					// 返回错误
					wsReturn(ctx, peerConn, resp)
					continue
				}
				spanName := "WsHandler/" + request.Method
				tracer := otel.Tracer(trace.TraceName)
				propagator := otel.GetTextMapPropagator()
				ctx := propagator.Extract(r.Context(), propagation.MapCarrier{
					"data":      string(request.Data),
					"callId":    request.CallId,
					"peerId":    request.PeerId,
					"clientIp":  clientIp,
					"connectAt": peerConn.ConnectedAt.Format("2006-01-02 15:04:05.000"),
				})
				spanCtx, span := tracer.Start(
					ctx,
					spanName,
					oteltrace.WithSpanKind(oteltrace.SpanKindServer),
					oteltrace.WithAttributes(semconv.HTTPServerAttributesFromHTTPRequest(
						"signaling-server", spanName, r)...),
				)
				var data []byte
				data, err = wslogic.Instance.OnCall(spanCtx, request)
				if err != nil {
					span.SetStatus(codes.Error, err.Error())
				} else {
					span.SetStatus(codes.Ok, "")
				}
				// 写入数据
				if len(data) > 0 {
					err = conn.WenSocketConnection.Write(ctx, websocket.MessageText, data)
					if err != nil {
						logger.Errorf("failed to write message: %v", err)
					}
				}
				span.End()
			}
		}
	}
	subscribe := func(ctx context.Context, peerConn *types.PeerConnection) error {
		wslogic.Instance.AddSubscriber(peerConn)
		defer wslogic.Instance.DeleteSubscriber(peerConn)
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
	go loopRead(ctx, cancelFunc, peerConn)
	err = subscribe(ctx, peerConn)
	if errors.Is(err, context.Canceled) {
		return
	}
	if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
		websocket.CloseStatus(err) == websocket.StatusGoingAway {
		return
	}
	if err != nil {
		logger.Errorf("failed to subscribe: %v", err)
		return
	}
}
