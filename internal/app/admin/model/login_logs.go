package model

// 登录日志事件类型（与 login_audits 锁定跟踪表严格区分）。
//
// 设计说明：
//   - auth_login_logs：本表，登录日志（"谁在什么时候、用什么客户端、登录结果如何"）。
//     一次性写满即可，**不参与锁定决策**。
//   - login_audits：已有表，锁定跟踪（failure_count / lock_short / lock_long
//     决定是否触发账号锁定，Redis 故障时降级查此表）。
//
// 两张表**不混用**：审计/留痕 vs 锁定控制是两个维度。
const (
	LoginStatusSuccess       = "success"        // 登录成功
	LoginStatusFailure       = "failure"        // 登录失败（用户存在但密码错 / 用户不存在）
	LoginStatusLockedAttempt = "locked_attempt" // 账号被锁定，用户尝试登录被拦截
	LoginStatusInvalidParam  = "invalid_param"  // 入参非法（username / password 为空等）
)

// LoginLog GORM 登录日志表。
//
// 字段语义：
//   - Status：上述 4 种事件之一
//   - UserID：成功登录时记录 user.id；失败 / 锁定 / 参数错误时为 0
//   - Reason：失败原因（password_mismatch / user_not_found / account_locked_short / ...），
//     成功 / 参数错误时为空
//   - UserAgent：浏览器 / 客户端 UA，便于事后追溯
//   - CreateAt：发生时间
type LoginLog struct {
	ID        uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Username  string `gorm:"size:64;index;not null;default:'';comment:用户名" json:"username"`
	UserID    uint   `gorm:"index;not null;default:0;comment:用户ID" json:"user_id"`
	Status    string `gorm:"size:32;index;not null;default:'';comment:登录状态" json:"status"`
	Reason    string `gorm:"text;not null;default:'';comment:失败原因" json:"reason"`
	IP        string `gorm:"size:64;not null;default:'';comment:IP地址" json:"ip"`
	UserAgent string `gorm:"text;not null;default:'';comment:用户代理" json:"user_agent"`
	CreatedAt int64  `gorm:"comment:发生时间" json:"created_at"`
}

// TableName 指定表名。
func (LoginLog) TableName() string { return "login_logs" }
