// Package dto 跨层入参/出参 POJO（无框架依赖）。
//
// 设计原则：
//   - dto 只放「跨层传递的纯数据」，不含方法、不含 binding 标签
//   - gin binding 标签留在 handler 层的请求结构上（避免 dto 反向依赖 gin）
//   - service 层方法签名直接接收 dto，避免 service 暴露过多入参类型
package dto

// LoginReq 登录请求参数。
//
// 登录场景不重新校验密码强度（注册时已校验过），只校验非空。
// 与 RegisterReq 区分：RegisterReq 的 Password 字段保留 `required,password` 触发完整规则链；
// LoginReq 改为 `required`，避免旧密码 / 测试种子密码长度受 12-24 限制无法登录。
//
// Phase 2：加 DeviceID 字段（设备指纹），登录成功后绑定到 JWT claims，
// 后续请求必须带 X-Device-ID 头才能通过中间件校验（防 token 泄露被异设备用）。
type LoginReq struct {
	Username string `json:"username"    binding:"required,min=5,max=24,fieldmatch=^[a-zA-Z0-9_-]+$:用户名为5-24位字母、数字、下划线、短横线"`
	Password string `json:"password"    binding:"required"`
	DeviceID string `json:"device_id"   binding:"omitempty,min=8,max=128"` // 设备指纹，UUID v4 风格，可选
}

// RegisterReq 注册请求参数。
//
// 密码用单一 password tag（与 LoginReq 一致），细分错误消息由 translatePassword hook 返回。
// 详细规则见 rules.go 的 passwordCheck 顺序（长度 → 大写 → 小写 → 数字 → 特殊 → 强度）。
type RegisterReq struct {
	Name     string `json:"name"     binding:"required,min=2,max=64"`
	Email    string `json:"email"    binding:"required,email"`
	Password string `json:"password" binding:"required,password"`
}

// RegisterInput 注册业务入参。
type RegisterInput struct {
	Username string
	Email    string
	Name     string
	Password string
}

// LoginInput 登录业务入参。
//
// Username 作为登录账号；IP / UserAgent 由 handler 从 c.ClientIP() / c.GetHeader("User-Agent") 注入，
// 供 service 层写入 auth_login_logs 审计表。
//
// Phase 2：DeviceID 由 handler 从请求头 X-Device-ID（或 req.DeviceID 兜底）注入，
// 用于绑定到 JWT claims。
type LoginInput struct {
	Username  string
	Password  string
	IP        string
	UserAgent string
	DeviceID  string
}

// UpdateNameInput 改昵称业务入参。
type UpdateNameInput struct {
	ID   uint
	Name string
}

// ListInput 列表查询业务入参。
type ListInput struct {
	Page     int
	PageSize int
}
