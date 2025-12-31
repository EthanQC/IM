package entity

import "time"

// FileKind 文件类型
type FileKind string

const (
	FileKindImage FileKind = "image"
	FileKindFile  FileKind = "file"
	FileKindAudio FileKind = "audio"
	FileKindVideo FileKind = "video"
)

// FileStatus 文件状态
type FileStatus int8

const (
	FileStatusPending   FileStatus = 0 // 待上传
	FileStatusUploaded  FileStatus = 1 // 已上传
	FileStatusConfirmed FileStatus = 2 // 已确认
	FileStatusDeleted   FileStatus = 3 // 已删除
)

// FileUpload 文件上传记录
type FileUpload struct {
	ID          uint64     `json:"id"`
	UserID      uint64     `json:"user_id"`
	ObjectKey   string     `json:"object_key"`
	FileName    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	SizeBytes   int64      `json:"size_bytes"`
	Kind        FileKind   `json:"kind"`
	Status      FileStatus `json:"status"`
	Bucket      string     `json:"bucket"`
	URL         string     `json:"url,omitempty"`
	Thumbnail   string     `json:"thumbnail,omitempty"`
	Width       int32      `json:"width,omitempty"`
	Height      int32      `json:"height,omitempty"`
	Duration    int32      `json:"duration,omitempty"` // 音视频时长(秒)
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// UploadToken 上传凭证
type UploadToken struct {
	ObjectKey   string    `json:"object_key"`
	UploadURL   string    `json:"upload_url"`
	CallbackURL string    `json:"callback_url,omitempty"`
	ExpiredAt   time.Time `json:"expired_at"`
}
