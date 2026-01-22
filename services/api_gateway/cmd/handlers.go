package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/grpc/metadata"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
)

func (g *Gateway) registerRoutes() {
	// 健康检查
	g.router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })

	// Swagger UI 文档 - 使用 ./docs（容器中工作目录为 /app）
	g.router.Static("/docs", "./docs")
	g.router.GET("/swagger", func(c *gin.Context) {
		c.Redirect(http.StatusMovedPermanently, "/docs/index.html")
	})

	// 公开接口（不需要认证）
	g.router.POST("/api/auth/register", g.handleRegister)
	g.router.POST("/api/auth/login", g.handleLogin)
	g.router.POST("/api/auth/refresh", g.handleRefresh)

	// 需要认证的接口
	authorized := g.router.Group("/api")
	authorized.Use(g.authMiddleware())
	{
		// 用户相关
		authorized.GET("/users/me", g.handleGetProfile)
		authorized.PUT("/users/me", g.handleUpdateProfile)

		// 联系人相关
		authorized.GET("/contacts", g.handleGetContacts)
		authorized.POST("/contacts/apply", g.handleApplyContact)
		authorized.POST("/contacts/handle", g.handleContactApply)
		authorized.DELETE("/contacts/:id", g.handleDeleteContact)

		// 会话相关
		authorized.GET("/conversations", g.handleGetConversations)
		authorized.POST("/conversations", g.handleCreateConversation)
		authorized.GET("/conversations/:id", g.handleGetConversation)
		authorized.PUT("/conversations/:id", g.handleUpdateConversation)

		// 消息相关
		authorized.POST("/messages", g.handleSendMessage)
		authorized.GET("/messages/history", g.handleGetHistory)
		authorized.POST("/messages/read", g.handleMarkRead)
		authorized.POST("/messages/:id/revoke", g.handleRevokeMessage)

		// 在线状态
		authorized.GET("/presence", g.handleGetPresence)

		// 文件相关
		authorized.POST("/files/upload", g.handleCreateUpload)
		authorized.POST("/files/complete", g.handleCompleteUpload)
	}
}

// JWT认证中间件
func (g *Gateway) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization header"})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
			return
		}

		tokenString := parts[1]
		claims := jwt.MapClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(g.cfg.JWT.Secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}

		// 从claims中提取user_id（存在 jti 字段中）
		if sub, ok := claims["sub"].(string); ok {
			userID, err := strconv.ParseUint(sub, 10, 64)
			if err == nil {
				c.Set("user_id", userID)
			}
		} else if jti, ok := claims["jti"].(string); ok {
			// 兼容旧 token: jti 里存 user_id
			var userID uint64
			fmt.Sscanf(jti, "%d", &userID)
			c.Set("user_id", userID)
		}

		c.Next()
	}
}

// ctxWithUserID 创建带有 user_id metadata 的 gRPC 上下文
func (g *Gateway) ctxWithUserID(c *gin.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	userID, exists := c.Get("user_id")
	if exists {
		ctx = metadata.AppendToOutgoingContext(ctx, "user_id", strconv.FormatUint(userID.(uint64), 10))
	}
	return ctx, cancel
}

// ==================== 请求/响应结构体 ====================

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type registerRequest struct {
	Username    string `json:"username" binding:"required"`
	Password    string `json:"password" binding:"required"`
	DisplayName string `json:"display_name"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type authResponse struct {
	AccessToken  string      `json:"access_token"`
	RefreshToken string      `json:"refresh_token"`
	ExpiresIn    int64       `json:"expires_in"`
	Profile      interface{} `json:"profile,omitempty"`
}

// ==================== 认证相关 Handler ====================

func (g *Gateway) handleRegister(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()

	resp, err := g.identityClient.Register(ctx, &imv1.RegisterRequest{
		Username:    req.Username,
		Password:    req.Password,
		DisplayName: req.DisplayName,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": authResponse{
			AccessToken:  resp.AccessToken,
			RefreshToken: resp.RefreshToken,
			ExpiresIn:    resp.ExpiresIn,
			Profile:      resp.Profile,
		},
	})
}

func (g *Gateway) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()
	resp, err := g.identityClient.Login(ctx, &imv1.LoginRequest{Username: req.Username, Password: req.Password})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
		Profile:      resp.Profile,
	})
}

func (g *Gateway) handleRefresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()
	resp, err := g.identityClient.Refresh(ctx, &imv1.RefreshRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
		Profile:      resp.Profile,
	})
}

// ==================== 用户相关 Handler ====================

func (g *Gateway) handleGetProfile(c *gin.Context) {
	userID := c.GetUint64("user_id")
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()

	resp, err := g.identityClient.GetProfile(ctx, &imv1.GetProfileRequest{UserId: int64(userID)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func (g *Gateway) handleUpdateProfile(c *gin.Context) {
	var req struct {
		DisplayName string `json:"display_name"`
		AvatarURL   string `json:"avatar_url"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	// 从上下文获取 user_id
	userID, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user_id not found"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()

	// 通过 gRPC metadata 传递 user_id
	ctx = metadata.AppendToOutgoingContext(ctx, "user_id", strconv.FormatUint(userID.(uint64), 10))

	resp, err := g.identityClient.UpdateProfile(ctx, &imv1.UpdateProfileRequest{
		DisplayName: req.DisplayName,
		AvatarUrl:   req.AvatarURL,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": resp})
}

// ==================== 联系人相关 Handler ====================

func (g *Gateway) handleGetContacts(c *gin.Context) {
	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.identityClient.ListContacts(ctx, &imv1.ListContactsRequest{Page: 1, PageSize: 100})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp.Contacts, "total": resp.Total})
}

func (g *Gateway) handleApplyContact(c *gin.Context) {
	var req struct {
		TargetUserID uint64 `json:"target_user_id" binding:"required"`
		Remark       string `json:"remark"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	_, err := g.identityClient.ApplyContact(ctx, &imv1.ApplyContactRequest{
		TargetUserId: int64(req.TargetUserID),
		Remark:       req.Remark,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (g *Gateway) handleContactApply(c *gin.Context) {
	var req struct {
		TargetUserID uint64 `json:"target_user_id" binding:"required"`
		Accept       bool   `json:"accept"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	_, err := g.identityClient.RespondContact(ctx, &imv1.RespondContactRequest{
		TargetUserId: int64(req.TargetUserID),
		Accept:       req.Accept,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (g *Gateway) handleDeleteContact(c *gin.Context) {
	targetID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid contact id"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	_, err = g.identityClient.RemoveContact(ctx, &imv1.RemoveContactRequest{TargetUserId: int64(targetID)})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

// ==================== 会话相关 Handler ====================

func (g *Gateway) handleGetConversations(c *gin.Context) {
	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.conversationClient.ListMyConversations(ctx, &imv1.ListMyConversationsRequest{Page: 1, PageSize: 50})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp.Items, "total": resp.Total})
}

func (g *Gateway) handleCreateConversation(c *gin.Context) {
	var req struct {
		Type      int8     `json:"type" binding:"required"` // 1: 单聊, 2: 群聊
		Title     string   `json:"title"`
		MemberIDs []uint64 `json:"member_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	memberIDs := make([]int64, len(req.MemberIDs))
	for i, id := range req.MemberIDs {
		memberIDs[i] = int64(id)
	}

	resp, err := g.conversationClient.CreateConversation(ctx, &imv1.CreateConversationRequest{
		Type:      imv1.ConversationType(req.Type),
		Title:     req.Title,
		MemberIds: memberIDs,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

func (g *Gateway) handleGetConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || convID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.conversationClient.ListMyConversations(ctx, &imv1.ListMyConversationsRequest{Page: 1, PageSize: 100})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, item := range resp.Items {
		if item.Id == convID {
			c.JSON(http.StatusOK, gin.H{"code": 0, "data": item})
			return
		}
	}
	c.JSON(http.StatusNotFound, gin.H{"error": "conversation not found"})
}

func (g *Gateway) handleUpdateConversation(c *gin.Context) {
	convID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || convID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid conversation id"})
		return
	}

	var req struct {
		Title string `json:"title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.conversationClient.UpdateConversation(ctx, &imv1.UpdateConversationRequest{
		ConversationId: convID,
		Title:          req.Title,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp})
}

// ==================== 消息相关 Handler ====================

func (g *Gateway) handleSendMessage(c *gin.Context) {
	var req struct {
		ConversationID int64  `json:"conversation_id" binding:"required"`
		ClientMsgID    string `json:"client_msg_id" binding:"required"`
		ContentType    int32  `json:"content_type" binding:"required"`
		Text           string `json:"text"` // 文本消息内容
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	// 构建消息体
	body := &imv1.MessageBody{
		Body: &imv1.MessageBody_Text{
			Text: &imv1.TextBody{Text: req.Text},
		},
	}

	resp, err := g.messageClient.SendMessage(ctx, &imv1.SendMessageRequest{
		ConversationId: req.ConversationID,
		ClientMsgId:    req.ClientMsgID,
		ContentType:    imv1.MessageContentType(req.ContentType),
		Body:           body,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp.Message})
}

func (g *Gateway) handleGetHistory(c *gin.Context) {
	var req struct {
		ConversationID int64 `form:"conversation_id" binding:"required"`
		AfterSeq       int64 `form:"after_seq"`
		Limit          int32 `form:"limit"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	if req.Limit == 0 {
		req.Limit = 50
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.messageClient.GetHistory(ctx, &imv1.GetHistoryRequest{
		ConversationId: req.ConversationID,
		AfterSeq:       req.AfterSeq,
		Limit:          req.Limit,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp.Items})
}

func (g *Gateway) handleMarkRead(c *gin.Context) {
	var req struct {
		ConversationID int64 `json:"conversation_id" binding:"required"`
		ReadSeq        int64 `json:"read_seq" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	_, err := g.messageClient.UpdateRead(ctx, &imv1.UpdateReadRequest{
		ConversationId: req.ConversationID,
		ReadSeq:        req.ReadSeq,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success"})
}

func (g *Gateway) handleRevokeMessage(c *gin.Context) {
	msgID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || msgID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid message id"})
		return
	}
	if g.cfg.Server.HttpAddrMessage == "" {
		c.JSON(http.StatusFailedDependency, gin.H{"error": "message http addr not configured"})
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()

	url := fmt.Sprintf("%s/api/v1/messages/%d/revoke", g.cfg.Server.HttpAddrMessage, msgID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	req.Header.Set("X-User-ID", strconv.FormatUint(c.GetUint64("user_id"), 10))

	client := &http.Client{Timeout: g.timeout}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}
	defer resp.Body.Close()

	payload, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		c.JSON(resp.StatusCode, gin.H{"error": string(payload)})
		return
	}
	c.Data(resp.StatusCode, "application/json", payload)
}

// ==================== 在线状态 Handler ====================

func (g *Gateway) handleGetPresence(c *gin.Context) {
	var req struct {
		UserIDs []int64 `form:"user_ids" binding:"required"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.presenceClient.GetOnline(ctx, &imv1.GetOnlineRequest{UserIds: req.UserIDs})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp.Items})
}

// ==================== 文件相关 Handler ====================

func (g *Gateway) handleCreateUpload(c *gin.Context) {
	var req struct {
		Filename    string `json:"filename" binding:"required"`
		ContentType string `json:"content_type" binding:"required"`
		SizeBytes   int64  `json:"size_bytes" binding:"required"`
		Kind        string `json:"kind"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.fileClient.CreateUpload(ctx, &imv1.CreateUploadRequest{
		Filename:    req.Filename,
		ContentType: req.ContentType,
		SizeBytes:   req.SizeBytes,
		Kind:        req.Kind,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"object_key":   resp.ObjectKey,
			"upload_url":   resp.UploadUrl,
			"callback_url": resp.CallbackUrl,
		},
	})
}

func (g *Gateway) handleCompleteUpload(c *gin.Context) {
	var req struct {
		ConversationID int64  `json:"conversation_id" binding:"required"`
		ClientMsgID    string `json:"client_msg_id" binding:"required"`
		ObjectKey      string `json:"object_key" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}

	ctx, cancel := g.ctxWithUserID(c)
	defer cancel()

	resp, err := g.fileClient.CompleteUpload(ctx, &imv1.CompleteUploadRequest{
		ConversationId: req.ConversationID,
		ClientMsgId:    req.ClientMsgID,
		Media: &imv1.MediaRef{
			ObjectKey: req.ObjectKey,
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": resp.Message})
}
