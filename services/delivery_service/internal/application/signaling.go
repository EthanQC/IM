package application

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/EthanQC/IM/services/delivery_service/internal/ports/in"
	"github.com/EthanQC/IM/services/delivery_service/internal/ports/out"
)

const (
	// 呼叫超时时间
	callTimeout = 60 * time.Second
	// 通话最大时长
	maxCallDuration = 4 * time.Hour
)

// SignalingConfig 信令服务配置
type SignalingConfig struct {
	STUNServers []string
	TURNServers []in.TURNServer
}

// CallSession 通话会话
type CallSession struct {
	CallID         string
	CallerID       uint64
	CallerDeviceID string
	CalleeID       uint64
	CalleeDeviceID string
	ConversationID uint64
	CallType       in.CallType
	Status         in.CallStatus
	StartedAt      int64
	RingingAt      int64
	ConnectedAt    int64
	EndedAt        int64

	// SDP信息
	CallerOffer  string
	CalleeAnswer string

	// ICE候选
	CallerCandidates []string
	CalleeCandidates []string

	mu sync.RWMutex
}

func (s *CallSession) GetState() *in.CallState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var duration int64
	if s.ConnectedAt > 0 {
		if s.EndedAt > 0 {
			duration = s.EndedAt - s.ConnectedAt
		} else {
			duration = time.Now().Unix() - s.ConnectedAt
		}
	}

	return &in.CallState{
		CallID:         s.CallID,
		CallerID:       s.CallerID,
		CalleeID:       s.CalleeID,
		ConversationID: s.ConversationID,
		CallType:       s.CallType,
		Status:         s.Status,
		StartedAt:      s.StartedAt,
		ConnectedAt:    s.ConnectedAt,
		EndedAt:        s.EndedAt,
		Duration:       duration,
	}
}

// SignalingUseCaseImpl 信令用例实现
type SignalingUseCaseImpl struct {
	config       SignalingConfig
	connManager  out.ConnectionManager
	callSessions map[string]*CallSession // callID -> session
	userCalls    map[uint64]string       // userID -> callID (当前通话)
	mu           sync.RWMutex

	// 用于清理超时会话
	stopCleaner chan struct{}
}

func NewSignalingUseCase(config SignalingConfig, connManager out.ConnectionManager) in.SignalingUseCase {
	uc := &SignalingUseCaseImpl{
		config:       config,
		connManager:  connManager,
		callSessions: make(map[string]*CallSession),
		userCalls:    make(map[uint64]string),
		stopCleaner:  make(chan struct{}),
	}

	// 启动清理协程
	go uc.cleanupRoutine()

	return uc
}

// HandleSignaling 处理信令消息
func (uc *SignalingUseCaseImpl) HandleSignaling(ctx context.Context, userID uint64, deviceID, action string, payload json.RawMessage) (interface{}, error) {
	switch action {
	case "call":
		var req struct {
			CalleeID       uint64      `json:"callee_id"`
			ConversationID uint64      `json:"conversation_id"`
			CallType       in.CallType `json:"call_type"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid call request: %w", err)
		}
		return uc.InitiateCall(ctx, &in.CallRequest{
			CallerID:       userID,
			CallerDeviceID: deviceID,
			CalleeID:       req.CalleeID,
			ConversationID: req.ConversationID,
			CallType:       req.CallType,
		})

	case "accept":
		var req struct {
			CallID string `json:"call_id"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid accept request: %w", err)
		}
		return nil, uc.AcceptCall(ctx, &in.AcceptCallRequest{
			CallID:   req.CallID,
			UserID:   userID,
			DeviceID: deviceID,
		})

	case "reject":
		var req struct {
			CallID string `json:"call_id"`
			Reason string `json:"reason"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid reject request: %w", err)
		}
		return nil, uc.RejectCall(ctx, &in.RejectCallRequest{
			CallID:   req.CallID,
			UserID:   userID,
			DeviceID: deviceID,
			Reason:   req.Reason,
		})

	case "hangup":
		var req struct {
			CallID string `json:"call_id"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid hangup request: %w", err)
		}
		return nil, uc.HangupCall(ctx, &in.HangupCallRequest{
			CallID:   req.CallID,
			UserID:   userID,
			DeviceID: deviceID,
		})

	case "offer":
		var req in.SDPRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid offer request: %w", err)
		}
		req.UserID = userID
		req.DeviceID = deviceID
		req.SDPType = "offer"
		return nil, uc.SendOffer(ctx, &req)

	case "answer":
		var req in.SDPRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid answer request: %w", err)
		}
		req.UserID = userID
		req.DeviceID = deviceID
		req.SDPType = "answer"
		return nil, uc.SendAnswer(ctx, &req)

	case "ice_candidate":
		var req in.ICECandidateRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid ice candidate request: %w", err)
		}
		req.UserID = userID
		req.DeviceID = deviceID
		return nil, uc.SendIceCandidate(ctx, &req)

	case "get_state":
		var req struct {
			CallID string `json:"call_id"`
		}
		if err := json.Unmarshal(payload, &req); err != nil {
			return nil, fmt.Errorf("invalid get_state request: %w", err)
		}
		return uc.GetCallState(ctx, req.CallID)

	default:
		return nil, fmt.Errorf("unknown signaling action: %s", action)
	}
}

// InitiateCall 发起呼叫
func (uc *SignalingUseCaseImpl) InitiateCall(ctx context.Context, req *in.CallRequest) (*in.CallResponse, error) {
	uc.mu.Lock()

	// 检查用户是否已在通话中
	if _, ok := uc.userCalls[req.CallerID]; ok {
		uc.mu.Unlock()
		return nil, fmt.Errorf("caller is already in a call")
	}
	if _, ok := uc.userCalls[req.CalleeID]; ok {
		uc.mu.Unlock()
		return &in.CallResponse{
			Status: in.CallStatusBusy,
		}, nil
	}

	// 创建通话会话
	callID := uuid.New().String()
	now := time.Now().Unix()

	session := &CallSession{
		CallID:         callID,
		CallerID:       req.CallerID,
		CallerDeviceID: req.CallerDeviceID,
		CalleeID:       req.CalleeID,
		ConversationID: req.ConversationID,
		CallType:       req.CallType,
		Status:         in.CallStatusInitiated,
		StartedAt:      now,
	}

	uc.callSessions[callID] = session
	uc.userCalls[req.CallerID] = callID
	uc.userCalls[req.CalleeID] = callID

	uc.mu.Unlock()

	// 发送呼叫通知给被叫方
	signalMsg := in.SignalingMessage{
		Action:     "incoming_call",
		CallID:     callID,
		FromUser:   req.CallerID,
		FromDevice: req.CallerDeviceID,
		Timestamp:  now,
	}

	callInfo, _ := json.Marshal(map[string]interface{}{
		"call_type":       req.CallType,
		"conversation_id": req.ConversationID,
		"stun_servers":    uc.config.STUNServers,
		"turn_servers":    uc.config.TURNServers,
	})
	signalMsg.Payload = callInfo

	uc.sendSignalingMessage(req.CalleeID, signalMsg)

	// 更新状态为响铃
	session.mu.Lock()
	session.Status = in.CallStatusRinging
	session.RingingAt = time.Now().Unix()
	session.mu.Unlock()

	return &in.CallResponse{
		CallID:      callID,
		Status:      in.CallStatusRinging,
		STUNServers: uc.config.STUNServers,
		TURNServers: uc.config.TURNServers,
	}, nil
}

// AcceptCall 接受呼叫
func (uc *SignalingUseCaseImpl) AcceptCall(ctx context.Context, req *in.AcceptCallRequest) error {
	session, err := uc.getSession(req.CallID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	if session.Status != in.CallStatusRinging {
		session.mu.Unlock()
		return fmt.Errorf("call is not in ringing state")
	}

	if session.CalleeID != req.UserID {
		session.mu.Unlock()
		return fmt.Errorf("user is not the callee")
	}

	session.Status = in.CallStatusAccepted
	session.CalleeDeviceID = req.DeviceID
	session.mu.Unlock()

	// 通知主叫方呼叫被接受
	signalMsg := in.SignalingMessage{
		Action:     "call_accepted",
		CallID:     req.CallID,
		FromUser:   req.UserID,
		FromDevice: req.DeviceID,
		Timestamp:  time.Now().Unix(),
	}
	uc.sendSignalingMessage(session.CallerID, signalMsg)

	return nil
}

// RejectCall 拒绝呼叫
func (uc *SignalingUseCaseImpl) RejectCall(ctx context.Context, req *in.RejectCallRequest) error {
	session, err := uc.getSession(req.CallID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	if session.Status != in.CallStatusRinging && session.Status != in.CallStatusInitiated {
		session.mu.Unlock()
		return fmt.Errorf("call cannot be rejected in current state")
	}

	session.Status = in.CallStatusRejected
	session.EndedAt = time.Now().Unix()
	session.mu.Unlock()

	// 通知主叫方呼叫被拒绝
	payload, _ := json.Marshal(map[string]string{"reason": req.Reason})
	signalMsg := in.SignalingMessage{
		Action:     "call_rejected",
		CallID:     req.CallID,
		FromUser:   req.UserID,
		FromDevice: req.DeviceID,
		Payload:    payload,
		Timestamp:  time.Now().Unix(),
	}
	uc.sendSignalingMessage(session.CallerID, signalMsg)

	// 清理会话
	uc.cleanupSession(req.CallID)

	return nil
}

// HangupCall 挂断通话
func (uc *SignalingUseCaseImpl) HangupCall(ctx context.Context, req *in.HangupCallRequest) error {
	session, err := uc.getSession(req.CallID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.Status = in.CallStatusEnded
	session.EndedAt = time.Now().Unix()
	session.mu.Unlock()

	// 确定对方用户
	var targetID uint64
	if session.CallerID == req.UserID {
		targetID = session.CalleeID
	} else {
		targetID = session.CallerID
	}

	// 通知对方通话已挂断
	signalMsg := in.SignalingMessage{
		Action:     "call_ended",
		CallID:     req.CallID,
		FromUser:   req.UserID,
		FromDevice: req.DeviceID,
		Timestamp:  time.Now().Unix(),
	}
	uc.sendSignalingMessage(targetID, signalMsg)

	// 清理会话
	uc.cleanupSession(req.CallID)

	return nil
}

// SendOffer 发送SDP Offer
func (uc *SignalingUseCaseImpl) SendOffer(ctx context.Context, req *in.SDPRequest) error {
	session, err := uc.getSession(req.CallID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.CallerOffer = req.SDP
	session.mu.Unlock()

	// 转发给对方
	payload, _ := json.Marshal(map[string]string{
		"sdp_type": "offer",
		"sdp":      req.SDP,
	})
	signalMsg := in.SignalingMessage{
		Action:     "sdp",
		CallID:     req.CallID,
		FromUser:   req.UserID,
		FromDevice: req.DeviceID,
		Payload:    payload,
		Timestamp:  time.Now().Unix(),
	}
	uc.sendSignalingMessage(req.TargetID, signalMsg)

	return nil
}

// SendAnswer 发送SDP Answer
func (uc *SignalingUseCaseImpl) SendAnswer(ctx context.Context, req *in.SDPRequest) error {
	session, err := uc.getSession(req.CallID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.CalleeAnswer = req.SDP
	// SDP协商完成，标记为已连接
	if session.Status == in.CallStatusAccepted {
		session.Status = in.CallStatusConnected
		session.ConnectedAt = time.Now().Unix()
	}
	session.mu.Unlock()

	// 转发给对方
	payload, _ := json.Marshal(map[string]string{
		"sdp_type": "answer",
		"sdp":      req.SDP,
	})
	signalMsg := in.SignalingMessage{
		Action:     "sdp",
		CallID:     req.CallID,
		FromUser:   req.UserID,
		FromDevice: req.DeviceID,
		Payload:    payload,
		Timestamp:  time.Now().Unix(),
	}
	uc.sendSignalingMessage(req.TargetID, signalMsg)

	return nil
}

// SendIceCandidate 发送ICE候选
func (uc *SignalingUseCaseImpl) SendIceCandidate(ctx context.Context, req *in.ICECandidateRequest) error {
	session, err := uc.getSession(req.CallID)
	if err != nil {
		return err
	}

	// 保存ICE候选
	session.mu.Lock()
	if session.CallerID == req.UserID {
		session.CallerCandidates = append(session.CallerCandidates, req.Candidate)
	} else {
		session.CalleeCandidates = append(session.CalleeCandidates, req.Candidate)
	}
	session.mu.Unlock()

	// 转发给对方
	payload, _ := json.Marshal(map[string]interface{}{
		"candidate":       req.Candidate,
		"sdp_mid":         req.SDPMid,
		"sdp_mline_index": req.SDPMLineIndex,
	})
	signalMsg := in.SignalingMessage{
		Action:     "ice_candidate",
		CallID:     req.CallID,
		FromUser:   req.UserID,
		FromDevice: req.DeviceID,
		Payload:    payload,
		Timestamp:  time.Now().Unix(),
	}
	uc.sendSignalingMessage(req.TargetID, signalMsg)

	return nil
}

// GetCallState 获取通话状态
func (uc *SignalingUseCaseImpl) GetCallState(ctx context.Context, callID string) (*in.CallState, error) {
	session, err := uc.getSession(callID)
	if err != nil {
		return nil, err
	}

	return session.GetState(), nil
}

func (uc *SignalingUseCaseImpl) getSession(callID string) (*CallSession, error) {
	uc.mu.RLock()
	defer uc.mu.RUnlock()

	session, ok := uc.callSessions[callID]
	if !ok {
		return nil, fmt.Errorf("call session not found: %s", callID)
	}

	return session, nil
}

func (uc *SignalingUseCaseImpl) sendSignalingMessage(userID uint64, msg in.SignalingMessage) {
	data, _ := json.Marshal(map[string]interface{}{
		"type": "signaling",
		"data": msg,
		"ts":   time.Now().UnixMilli(),
	})

	if err := uc.connManager.Send(userID, data); err != nil {
		fmt.Printf("failed to send signaling message to user %d: %v\n", userID, err)
	}
}

func (uc *SignalingUseCaseImpl) cleanupSession(callID string) {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	session, ok := uc.callSessions[callID]
	if !ok {
		return
	}

	delete(uc.userCalls, session.CallerID)
	delete(uc.userCalls, session.CalleeID)
	delete(uc.callSessions, callID)
}

func (uc *SignalingUseCaseImpl) cleanupRoutine() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-uc.stopCleaner:
			return
		case <-ticker.C:
			uc.cleanupTimeoutSessions()
		}
	}
}

func (uc *SignalingUseCaseImpl) cleanupTimeoutSessions() {
	uc.mu.Lock()
	defer uc.mu.Unlock()

	now := time.Now().Unix()
	toCleanup := make([]string, 0)

	for callID, session := range uc.callSessions {
		session.mu.RLock()

		// 检查响铃超时
		if session.Status == in.CallStatusRinging {
			if now-session.RingingAt > int64(callTimeout.Seconds()) {
				session.mu.RUnlock()
				session.mu.Lock()
				session.Status = in.CallStatusTimeout
				session.EndedAt = now
				session.mu.Unlock()

				// 通知双方超时
				signalMsg := in.SignalingMessage{
					Action:    "call_timeout",
					CallID:    callID,
					Timestamp: now,
				}
				go uc.sendSignalingMessage(session.CallerID, signalMsg)
				go uc.sendSignalingMessage(session.CalleeID, signalMsg)

				toCleanup = append(toCleanup, callID)
				continue
			}
		}

		// 检查通话时长超时
		if session.Status == in.CallStatusConnected {
			if now-session.ConnectedAt > int64(maxCallDuration.Seconds()) {
				session.mu.RUnlock()
				session.mu.Lock()
				session.Status = in.CallStatusEnded
				session.EndedAt = now
				session.mu.Unlock()

				// 通知双方通话结束
				signalMsg := in.SignalingMessage{
					Action:    "call_ended",
					CallID:    callID,
					Timestamp: now,
				}
				go uc.sendSignalingMessage(session.CallerID, signalMsg)
				go uc.sendSignalingMessage(session.CalleeID, signalMsg)

				toCleanup = append(toCleanup, callID)
				continue
			}
		}

		// 清理已结束的会话
		if session.Status == in.CallStatusEnded || session.Status == in.CallStatusRejected ||
			session.Status == in.CallStatusTimeout || session.Status == in.CallStatusCancelled {
			if now-session.EndedAt > 60 { // 保留60秒后清理
				toCleanup = append(toCleanup, callID)
			}
		}

		session.mu.RUnlock()
	}

	// 执行清理
	for _, callID := range toCleanup {
		if session, ok := uc.callSessions[callID]; ok {
			delete(uc.userCalls, session.CallerID)
			delete(uc.userCalls, session.CalleeID)
			delete(uc.callSessions, callID)
		}
	}
}

// Stop 停止信令服务
func (uc *SignalingUseCaseImpl) Stop() {
	close(uc.stopCleaner)
}
