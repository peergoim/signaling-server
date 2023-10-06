package handler

import (
	"github.com/gin-gonic/gin"
	"github.com/peergoim/signaling-server/internal/handler/wslogic"
	"github.com/peergoim/signaling-server/internal/types"
)

func (h *Handler) CallHandler(context *gin.Context) {
	wsOnce.Do(func() {
		wslogic.Init(h.svcCtx)
	})
	request := &types.CallRequest{}
	if err := context.ShouldBindJSON(request); err != nil {
		context.JSON(200, types.RequestUnmarshalErrorResponse)
		return
	}
	response, err := wslogic.Instance.OnCall(context, request)
	if err != nil {
		// 设置500
		context.Writer.WriteHeader(500)
	} else {
		// 设置200
		context.Writer.WriteHeader(200)
	}
	context.Header("Content-Type", "application/json")
	context.Writer.Write(response)
}
