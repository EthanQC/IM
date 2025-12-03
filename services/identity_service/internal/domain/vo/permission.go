package vo

type Permission struct {
	Resource string // 资源类型，User/Group/Message
	Action   string // 操作类型，Read/Write/Manage/Send
}

// 资源类型定义
const (
	ResourceUser    = "User"    // 用户资源
	ResourceGroup   = "Group"   // 群组资源
	ResourceMessage = "Message" // 消息资源
)

// 操作类型定义
const (
	ActionRead   = "Read"   // 读取
	ActionWrite  = "Write"  // 写入
	ActionManage = "Manage" // 管理
	ActionSend   = "Send"   // 发送
)

func NewPermission(resource string, action string) *Permission {
	return &Permission{
		Resource: resource,
		Action:   action,
	}
}

// 验证权限格式是否合法
func (p *Permission) IsValid() bool {
	// 验证 Resource
	validResources := []string{ResourceUser, ResourceGroup, ResourceMessage}
	resourceValid := false

	for _, r := range validResources {
		if p.Resource == r {
			resourceValid = true
			break
		}
	}

	// 验证 Action
	validActions := []string{ActionRead, ActionWrite, ActionManage, ActionSend}
	actionValid := false

	for _, a := range validActions {
		if p.Action == a {
			actionValid = true
			break
		}
	}

	return resourceValid && actionValid
}
