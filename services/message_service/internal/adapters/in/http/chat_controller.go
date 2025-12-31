package http

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/EthanQC/IM/services/message_service/internal/domain/entity"
	"github.com/EthanQC/IM/services/message_service/internal/ports/in"
)

// ChatController HTTP消息控制器
type ChatController struct {
	messageUseCase in.MessageUseCase
}

// NewChatController 创建消息控制器
func NewChatController(messageUseCase in.MessageUseCase) *ChatController {
	return &ChatController{messageUseCase: messageUseCase}
}

// RegisterRoutes 注册路由
func (c *ChatController) RegisterRoutes(r *gin.RouterGroup) {
	messages := r.Group("/messages")
	{
		messages.POST("", c.SendMessage)
		messages.GET("/history", c.GetHistory)
		messages.POST("/read", c.UpdateRead)
		messages.POST("/:id/revoke", c.RevokeMessage)
		messages.DELETE("/:id", c.DeleteMessage)
		messages.GET("/unread", c.GetUnreadCount)
	}
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	ConversationID uint64                `json:"conversation_id" binding:"required"`
	ClientMsgID    string                `json:"client_msg_id" binding:"required"`
	ContentType    int8                  `json:"content_type" binding:"required"`
	Content        entity.MessageContent `json:"content" binding:"required"`
	ReplyToMsgID   *uint64               `json:"reply_to_msg_id"`
}

// SendMessage 发送消息
// @Summary 发送消息
// @Tags Messages
// @Accept json
// @Produce json
// @Param request body SendMessageRequest true "消息内容"
// @Success 200 {object} map[string]interface{}
// @Router /messages [post]
func (c *ChatController) SendMessage(ctx *gin.Context) {
	userID := ctx.GetUint64("user_id") // 从JWT中间件获取
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req SendMessageRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	msg, err := c.messageUseCase.SendMessage(ctx.Request.Context(), &in.SendMessageRequest{
		ConversationID: req.ConversationID,
		SenderID:       userID,
		ClientMsgID:    req.ClientMsgID,
		ContentType:    entity.MessageContentType(req.ContentType),
		Content:        req.Content,
		ReplyToMsgID:   req.ReplyToMsgID,
	})
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"message_id": msg.ID,
			"seq":        msg.Seq,
			"created_at": msg.CreatedAt,
		},
	})
}

// GetHistoryRequest 获取历史消息请求
type GetHistoryRequest struct {
	ConversationID uint64 `form:"conversation_id" binding:"required"`
	AfterSeq       uint64 `form:"after_seq"`
	BeforeSeq      uint64 `form:"before_seq"`
	Limit          int    `form:"limit,default=50"`
}

// GetHistory 获取历史消息
// @Summary 获取历史消息
// @Tags Messages
// @Accept json
// @Produce json
// @Param conversation_id query uint64 true "会话ID"
// @Param after_seq query uint64 false "获取此序号之后的消息"
// @Param before_seq query uint64 false "获取此序号之前的消息"
// @Param limit query int false "限制数量,默认50"
// @Success 200 {object} map[string]interface{}
// @Router /messages/history [get]
func (c *ChatController) GetHistory(ctx *gin.Context) {
	userID := ctx.GetUint64("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req GetHistoryRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit <= 0 || req.Limit > 100 {
		req.Limit = 50
	}

	messages, err := c.messageUseCase.GetHistory(ctx.Request.Context(), req.ConversationID, req.AfterSeq, req.Limit)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"messages": messages,
		},
	})
}

// UpdateReadRequest 更新已读请求
type UpdateReadRequest struct {
	ConversationID uint64 `json:"conversation_id" binding:"required"`
	ReadSeq        uint64 `json:"read_seq" binding:"required"`
}

// UpdateRead 更新已读状态
// @Summary 更新已读状态
// @Tags Messages
// @Accept json
// @Produce json
// @Param request body UpdateReadRequest true "已读信息"
// @Success 200 {object} map[string]interface{}
// @Router /messages/read [post]
func (c *ChatController) UpdateRead(ctx *gin.Context) {
	userID := ctx.GetUint64("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req UpdateReadRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err := c.messageUseCase.UpdateRead(ctx.Request.Context(), userID, req.ConversationID, req.ReadSeq)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
	})
}

// RevokeMessage 撤回消息
// @Summary 撤回消息
// @Tags Messages
// @Accept json
// @Produce json
// @Param id path uint64 true "消息ID"
// @Success 200 {object} map[string]interface{}
// @Router /messages/{id}/revoke [post]
func (c *ChatController) RevokeMessage(ctx *gin.Context) {
	userID := ctx.GetUint64("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	msgID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	err = c.messageUseCase.RevokeMessage(ctx.Request.Context(), userID, msgID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
	})
}

// DeleteMessage 删除消息
// @Summary 删除消息(仅对自己不可见)
// @Tags Messages
// @Accept json
// @Produce json
// @Param id path uint64 true "消息ID"
// @Success 200 {object} map[string]interface{}
// @Router /messages/{id} [delete]
func (c *ChatController) DeleteMessage(ctx *gin.Context) {
	userID := ctx.GetUint64("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	msgID, err := strconv.ParseUint(ctx.Param("id"), 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}

	err = c.messageUseCase.DeleteMessage(ctx.Request.Context(), userID, msgID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"msg":  "success",
	})
}

// GetUnreadCountRequest 获取未读数请求
type GetUnreadCountRequest struct {
	ConversationID uint64 `form:"conversation_id" binding:"required"`
}

// GetUnreadCount 获取未读消息数
// @Summary 获取未读消息数
// @Tags Messages
// @Accept json
// @Produce json
// @Param conversation_id query uint64 true "会话ID"
// @Success 200 {object} map[string]interface{}
// @Router /messages/unread [get]
func (c *ChatController) GetUnreadCount(ctx *gin.Context) {
	userID := ctx.GetUint64("user_id")
	if userID == 0 {
		ctx.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}

	var req GetUnreadCountRequest
	if err := ctx.ShouldBindQuery(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	count, err := c.messageUseCase.GetUnreadCount(ctx.Request.Context(), userID, req.ConversationID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	ctx.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"unread_count": count,
		},
	})
}
