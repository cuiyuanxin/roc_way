package errcode

var (
	ErrUserNotFound       = Code{1001, "用户不存在", 404}
	ErrUserExists         = Code{1002, "用户已存在", 409}
	ErrUnauthorized       = Code{2001, "未登录或登录已过期", 401}
	ErrAccountLocked      = Code{2005, "账号已锁定，请稍后再试", 423}
	ErrPasswordMismatched = Code{2010, "密码错误", 401}
)
