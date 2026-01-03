package in

import (
	"context"
	"encoding/json"
)

// SignalingUseCase WebRTC信令用例接口
type SignalingUseCase interface {
	// HandleSignaling 处理信令消息
	HandleSignaling(ctx context.Context, userID uint64, deviceID, action string, payload json.RawMessage) (interface{}, error)

	// InitiateCall 发起呼叫
	InitiateCall(ctx context.Context, req *CallRequest) (*CallResponse, error)

	// AcceptCall 接受呼叫
	AcceptCall(ctx context.Context, req *AcceptCallRequest) error

	// RejectCall 拒绝呼叫
	RejectCall(ctx context.Context, req *RejectCallRequest) error

	// HangupCall 挂断通话
	HangupCall(ctx context.Context, req *HangupCallRequest) error

	// SendOffer 发送SDP Offer
	SendOffer(ctx context.Context, req *SDPRequest) error

	// SendAnswer 发送SDP Answer
	SendAnswer(ctx context.Context, req *SDPRequest) error

	// SendIceCandidate 发送ICE候选
	SendIceCandidate(ctx context.Context, req *ICECandidateRequest) error

	// GetCallState 获取通话状态
	GetCallState(ctx context.Context, callID string) (*CallState, error)
}

// CallType 通话类型
type CallType string

const (
	CallTypeAudio CallType = "audio"
	CallTypeVideo CallType = "video"
)

// CallStatus 通话状态
type CallStatus string

const (
	CallStatusInitiated CallStatus = "initiated" // 已发起
	CallStatusRinging   CallStatus = "ringing"   // 响铃中
	CallStatusAccepted  CallStatus = "accepted"  // 已接受
	CallStatusConnected CallStatus = "connected" // 已连接
	CallStatusEnded     CallStatus = "ended"     // 已结束
	CallStatusRejected  CallStatus = "rejected"  // 已拒绝
	CallStatusTimeout   CallStatus = "timeout"   // 超时
	CallStatusCancelled CallStatus = "cancelled" // 已取消
	CallStatusBusy      CallStatus = "busy"      // 忙线
)

// CallRequest 呼叫请求
type CallRequest struct {
	CallerID       uint64   `json:"caller_id"`
	CallerDeviceID string   `json:"caller_device_id"`
	CalleeID       uint64   `json:"callee_id"`
	ConversationID uint64   `json:"conversation_id"`
	CallType       CallType `json:"call_type"`
}

// CallResponse 呼叫响应
type CallResponse struct {
	CallID      string       `json:"call_id"`
	Status      CallStatus   `json:"status"`
	STUNServers []string     `json:"stun_servers"`
	TURNServers []TURNServer `json:"turn_servers,omitempty"`
}

// TURNServer TURN服务器配置
type TURNServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username"`
	Credential string   `json:"credential"`
}

// AcceptCallRequest 接受呼叫请求
type AcceptCallRequest struct {
	CallID   string `json:"call_id"`
	UserID   uint64 `json:"user_id"`
	DeviceID string `json:"device_id"`
}

// RejectCallRequest 拒绝呼叫请求
type RejectCallRequest struct {
	CallID   string `json:"call_id"`
	UserID   uint64 `json:"user_id"`
	DeviceID string `json:"device_id"`
	Reason   string `json:"reason,omitempty"` // busy, decline, timeout
}

// HangupCallRequest 挂断请求
type HangupCallRequest struct {
	CallID   string `json:"call_id"`
	UserID   uint64 `json:"user_id"`
	DeviceID string `json:"device_id"`
}

// SDPRequest SDP请求
type SDPRequest struct {
	CallID   string `json:"call_id"`
	UserID   uint64 `json:"user_id"`
	DeviceID string `json:"device_id"`
	TargetID uint64 `json:"target_id"`
	SDPType  string `json:"sdp_type"` // offer, answer
	SDP      string `json:"sdp"`
}

// ICECandidateRequest ICE候选请求
type ICECandidateRequest struct {
	CallID        string `json:"call_id"`
	UserID        uint64 `json:"user_id"`
	DeviceID      string `json:"device_id"`
	TargetID      uint64 `json:"target_id"`
	Candidate     string `json:"candidate"`
	SDPMid        string `json:"sdp_mid"`
	SDPMLineIndex int    `json:"sdp_mline_index"`
}

// CallState 通话状态
type CallState struct {
	CallID         string     `json:"call_id"`
	CallerID       uint64     `json:"caller_id"`
	CalleeID       uint64     `json:"callee_id"`
	ConversationID uint64     `json:"conversation_id"`
	CallType       CallType   `json:"call_type"`
	Status         CallStatus `json:"status"`
	StartedAt      int64      `json:"started_at"`
	ConnectedAt    int64      `json:"connected_at,omitempty"`
	EndedAt        int64      `json:"ended_at,omitempty"`
	Duration       int64      `json:"duration,omitempty"` // 通话时长（秒）
}

// SignalingMessage 信令消息（用于推送给对方）
type SignalingMessage struct {
	Action     string          `json:"action"`
	CallID     string          `json:"call_id"`
	FromUser   uint64          `json:"from_user"`
	FromDevice string          `json:"from_device"`
	Payload    json.RawMessage `json:"payload,omitempty"`
	Timestamp  int64           `json:"timestamp"`
}
