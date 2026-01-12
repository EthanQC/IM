package chunked

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"sync"
	"time"
)

var (
	ErrUploadNotFound  = errors.New("upload not found")
	ErrChunkExists     = errors.New("chunk already exists")
	ErrInvalidChunk    = errors.New("invalid chunk")
	ErrUploadExpired   = errors.New("upload expired")
	ErrUploadIncomplete = errors.New("upload incomplete")
)

// ChunkInfo 分片信息
type ChunkInfo struct {
	Index      int
	Size       int64
	Hash       string
	UploadedAt time.Time
}

// UploadSession 上传会话
type UploadSession struct {
	UploadID    string
	FileName    string
	FileSize    int64
	ChunkSize   int64
	TotalChunks int
	Chunks      map[int]*ChunkInfo
	CreatedAt   time.Time
	ExpiresAt   time.Time
	mu          sync.RWMutex
}

// NewUploadSession 创建上传会话
func NewUploadSession(uploadID, fileName string, fileSize, chunkSize int64, ttl time.Duration) *UploadSession {
	totalChunks := int((fileSize + chunkSize - 1) / chunkSize)
	return &UploadSession{
		UploadID:    uploadID,
		FileName:    fileName,
		FileSize:    fileSize,
		ChunkSize:   chunkSize,
		TotalChunks: totalChunks,
		Chunks:      make(map[int]*ChunkInfo),
		CreatedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(ttl),
	}
}

// AddChunk 添加分片
func (s *UploadSession) AddChunk(index int, size int64, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if index < 0 || index >= s.TotalChunks {
		return ErrInvalidChunk
	}

	if _, exists := s.Chunks[index]; exists {
		return ErrChunkExists
	}

	hash := md5.Sum(data)
	s.Chunks[index] = &ChunkInfo{
		Index:      index,
		Size:       size,
		Hash:       hex.EncodeToString(hash[:]),
		UploadedAt: time.Now(),
	}
	return nil
}

// IsComplete 检查是否完整
func (s *UploadSession) IsComplete() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.Chunks) == s.TotalChunks
}

// IsExpired 检查是否过期
func (s *UploadSession) IsExpired() bool {
	return time.Now().After(s.ExpiresAt)
}

// GetProgress 获取上传进度
func (s *UploadSession) GetProgress() float64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.TotalChunks == 0 {
		return 0
	}
	return float64(len(s.Chunks)) / float64(s.TotalChunks) * 100
}

// GetMissingChunks 获取缺失的分片索引
func (s *UploadSession) GetMissingChunks() []int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	missing := make([]int, 0)
	for i := 0; i < s.TotalChunks; i++ {
		if _, exists := s.Chunks[i]; !exists {
			missing = append(missing, i)
		}
	}
	return missing
}

// UploadManager 上传管理器
type UploadManager struct {
	sessions    map[string]*UploadSession
	mu          sync.RWMutex
	defaultTTL  time.Duration
	chunkSize   int64
}

// NewUploadManager 创建上传管理器
func NewUploadManager(chunkSize int64, defaultTTL time.Duration) *UploadManager {
	if chunkSize <= 0 {
		chunkSize = 5 * 1024 * 1024 // 默认 5MB
	}
	if defaultTTL <= 0 {
		defaultTTL = 24 * time.Hour
	}
	return &UploadManager{
		sessions:   make(map[string]*UploadSession),
		defaultTTL: defaultTTL,
		chunkSize:  chunkSize,
	}
}

// CreateSession 创建上传会话
func (m *UploadManager) CreateSession(uploadID, fileName string, fileSize int64) *UploadSession {
	session := NewUploadSession(uploadID, fileName, fileSize, m.chunkSize, m.defaultTTL)
	
	m.mu.Lock()
	m.sessions[uploadID] = session
	m.mu.Unlock()
	
	return session
}

// GetSession 获取会话
func (m *UploadManager) GetSession(uploadID string) (*UploadSession, error) {
	m.mu.RLock()
	session, ok := m.sessions[uploadID]
	m.mu.RUnlock()

	if !ok {
		return nil, ErrUploadNotFound
	}
	if session.IsExpired() {
		m.DeleteSession(uploadID)
		return nil, ErrUploadExpired
	}
	return session, nil
}

// DeleteSession 删除会话
func (m *UploadManager) DeleteSession(uploadID string) {
	m.mu.Lock()
	delete(m.sessions, uploadID)
	m.mu.Unlock()
}

// CleanupExpired 清理过期会话
func (m *UploadManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	for id, session := range m.sessions {
		if session.IsExpired() {
			delete(m.sessions, id)
			count++
		}
	}
	return count
}
