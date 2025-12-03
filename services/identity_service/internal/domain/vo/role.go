package vo

type Role struct {
	Name        string   // 角色名称
	Permissions []string // 角色拥有的权限列表
}

func NewRole(name string, permissions []string) *Role {
	return &Role{
		Name:        name,
		Permissions: permissions,
	}
}

// 检查是否拥有某个权限
func (r *Role) HasPermission(permission string) bool {
	for _, p := range r.Permissions {
		if p == permission {
			return true
		}
	}

	return false
}
