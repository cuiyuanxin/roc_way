package model

// EventType 登录审计事件类型。
const (
	EventFailure   = "failure"    // 单次登录失败
	EventLockShort = "lock_short" // 触发短期锁定（5-9 次连败）
	EventLockLong  = "lock_long"  // 触发长期锁定（>=10 次连败）
)

// LoginAudit GORM 登录审计表（包含失败 / 锁定 / 长期锁定事件）。
//
// 单表设计：避免 account_locks + login_failures 双表膨胀。
// Redis 故障时降级查此表判断是否锁定 + 写入审计。
//
// 时间字段语义：
//   - CreatedAt / ExpiresAt 用 int64 存 unix 秒时间戳（与项目其它 audit 类表一致）。
//   - **GORM 不会自动填充 int64 类型的 CreatedAt**（只对 time.Time 自动填），
type LoginAudit struct {
	ID          uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Username    string `gorm:"size:64;index;not null;default:'';comment:用户名" json:"username"`
	EventType   string `gorm:"size:16;index;not null;default:'';comment:事件类型" json:"event_type"`
	FailedCount int    `gorm:"not null;default:0;comment:失败次数" json:"failed_count"`
	IP          string `gorm:"size:64;not null;default:'';comment:IP" json:"ip"`
	CreatedAt   int64  `gorm:"comment:创建时间" json:"created_at"`
	ExpiresAt   int64  `gorm:"comment:过期时间" json:"expires_at"`
}

// TableName 指定表名。
func (LoginAudit) TableName() string { return "login_audits" }
