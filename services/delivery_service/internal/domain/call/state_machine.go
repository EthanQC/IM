package call

import (
	"errors"
	"sync"
	"time"
)

// CallState 通话状态
type CallState string

const (
	StateIdle       CallState = "idle"       // 空闲
	StateRinging    CallState = "ringing"    // 响铃中
	StateConnecting CallState = "connecting" // 连接中
	StateConnected  CallState = "connected"  // 已连接
	StateEnded      CallState = "ended"      // 已结束
)

// CallEvent 通话事件
type CallEvent string

const (
	EventInitiate CallEvent = "initiate" // 发起通话
	EventAccept   CallEvent = "accept"   // 接受通话
	EventReject   CallEvent = "reject"   // 拒绝通话
	EventCancel   CallEvent = "cancel"   // 取消通话
	EventConnect  CallEvent = "connect"  // 连接建立
	EventHangup   CallEvent = "hangup"   // 挂断
	EventTimeout  CallEvent = "timeout"  // 超时
)

// CallStateMachine 通话状态机
type CallStateMachine struct {
	callID      string
	callerID    string
	calleeID    string
	state       CallState
	startTime   time.Time
	connectTime time.Time
	endTime     time.Time
	mu          sync.RWMutex
	transitions map[stateEvent]CallState
}

type stateEvent struct {
	state CallState
	event CallEvent
}

// NewCallStateMachine 创建状态机
func NewCallStateMachine(callID, callerID, calleeID string) *CallStateMachine {
	sm := &CallStateMachine{
		callID:    callID,
		callerID:  callerID,
		calleeID:  calleeID,
		state:     StateIdle,
		startTime: time.Now(),
	}
	sm.initTransitions()
	return sm
}

func (sm *CallStateMachine) initTransitions() {
	sm.transitions = map[stateEvent]CallState{
		{StateIdle, EventInitiate}:       StateRinging,
		{StateRinging, EventAccept}:      StateConnecting,
		{StateRinging, EventReject}:      StateEnded,
		{StateRinging, EventCancel}:      StateEnded,
		{StateRinging, EventTimeout}:     StateEnded,
		{StateConnecting, EventConnect}:  StateConnected,
		{StateConnecting, EventTimeout}:  StateEnded,
		{StateConnecting, EventHangup}:   StateEnded,
		{StateConnected, EventHangup}:    StateEnded,
	}
}

var (
	ErrInvalidTransition = errors.New("invalid state transition")
	ErrCallEnded         = errors.New("call has already ended")
)

// Transition 执行状态转换
func (sm *CallStateMachine) Transition(event CallEvent) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.state == StateEnded {
		return ErrCallEnded
	}

	key := stateEvent{sm.state, event}
	newState, ok := sm.transitions[key]
	if !ok {
		return ErrInvalidTransition
	}

	// 记录时间
	if newState == StateConnected {
		sm.connectTime = time.Now()
	}
	if newState == StateEnded {
		sm.endTime = time.Now()
	}

	sm.state = newState
	return nil
}

// GetState 获取当前状态
func (sm *CallStateMachine) GetState() CallState {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state
}

// GetCallID 获取通话ID
func (sm *CallStateMachine) GetCallID() string {
	return sm.callID
}

// GetDuration 获取通话时长
func (sm *CallStateMachine) GetDuration() time.Duration {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.getDurationUnsafe()
}

func (sm *CallStateMachine) getDurationUnsafe() time.Duration {
	if sm.connectTime.IsZero() {
		return 0
	}
	if sm.endTime.IsZero() {
		return time.Since(sm.connectTime)
	}
	return sm.endTime.Sub(sm.connectTime)
}

// IsActive 是否活跃
func (sm *CallStateMachine) IsActive() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.state != StateIdle && sm.state != StateEnded
}

// GetCallInfo 获取通话信息
func (sm *CallStateMachine) GetCallInfo() CallInfo {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return CallInfo{
		CallID:      sm.callID,
		CallerID:    sm.callerID,
		CalleeID:    sm.calleeID,
		State:       sm.state,
		StartTime:   sm.startTime,
		ConnectTime: sm.connectTime,
		EndTime:     sm.endTime,
		Duration:    sm.getDurationUnsafe(),
	}
}

// CallInfo 通话信息
type CallInfo struct {
	CallID      string
	CallerID    string
	CalleeID    string
	State       CallState
	StartTime   time.Time
	ConnectTime time.Time
	EndTime     time.Time
	Duration    time.Duration
}
