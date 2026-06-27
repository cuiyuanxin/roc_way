// Package dto 跨层入参/出参 POJO 的验证规则注册。
//
// 把所有与 dto 字段相关的验证规则集中在此，由 main 启动时显式调用
// RegisterAll(v) 注入到 validator 实例。
//
// 优点：
//   - 紧耦合：规则与 dto 字段在一起，便于维护
//   - 无 init 副作用：import dto 不会触发任何注册行为
//   - 可测试：测试时可以不调用 RegisterAll 即可关闭规则
package dto

import (
	"reflect"
	"regexp"

	govalidator "github.com/go-playground/validator/v10"

	rocvalidator "github.com/cuiyuanxin/roc_way/internal/pkg/validator"
)

// mobileRe 中国大陆手机号正则。
var mobileRe = regexp.MustCompile(`^1[3-9]\d{9}$`)

// RegisterAll 把本应用所有自定义验证规则注册到 v。
// 调用方式：v := validator.New(); dto.RegisterAll(v)
func RegisterAll(v *rocvalidator.Validator) {
	for _, r := range rules() {
		v.RegisterRule(r)
	}
	// password 单 tag + 内部分段：把 per-tag 的细分翻译回调挂到 Validator。
	// mapErr 阶段命中 tag="password" 时会用本回调返回具体错误消息。
	v.RegisterTagHook("password", translatePassword)
}

// rules 返回本应用所有自定义验证规则。
func rules() []rocvalidator.Rule {
	return []rocvalidator.Rule{
		{
			Tag: "mobile",
			Fn: func(fl govalidator.FieldLevel) bool {
				return mobileRe.MatchString(fl.Field().String())
			},
			ZHMessage: "{0} 手机号格式不正确",
			ENMessage: "{0} mobile format invalid",
		},

		// ===== 密码：单 tag 入口 + 内部分段 =====
		//
		// 用法：binding:"required,password"
		// fn 内部按前端 validatePassword 顺序逐段校验，任一失败返回 false。
		// 细分错误消息由 translatePassword 拿到 field 值后重跑 passwordCheck 得到首个失败子项。
		{
			Tag:       "password",
			Fn:        Password,
			ZHMessage: "密码格式不合法",
			ENMessage: "invalid password",
		},
	}
}

// Password 单 tag 入口：调 passwordCheck，0 个失败子项 → true，否则 false。
// 失败时具体哪一段由 translatePassword 通过 passwordCheck 再次扫描得出。
func Password(fl govalidator.FieldLevel) bool {
	return len(passwordCheck(fl.Field().String())) == 0
}

// translatePassword password tag 的细分翻译回调。
//
// 拿到 fe 对应的字段值后，重跑 passwordCheck 得到首个失败子项，返回针对性文案。
func translatePassword(_ govalidator.FieldError, fieldVal reflect.Value, lang string) string {
	failed := passwordCheck(fieldVal.String())
	if len(failed) == 0 {
		// 理论不应走到这：tag="password" 意味着 Password 返回 false。
		return passwordSubMsgs[lang]["default"]
	}
	if msg, ok := passwordSubMsgs[lang][failed[0]]; ok {
		return msg
	}
	return passwordSubMsgs[lang]["default"]
}

// passwordCheck 按前端 validatePassword 顺序逐段校验，返回失败子项名列表。
//
// 顺序：len → upper → lower → digit → special → strong。
// 长度不达标时短路返回，前端也是这个行为。
func passwordCheck(s string) []string {
	var failed []string
	if !pwdLenCheck(s) {
		failed = append(failed, "len")
		return failed
	}
	if !pwdUpperCheck(s) {
		failed = append(failed, "upper")
	}
	if !pwdLowerCheck(s) {
		failed = append(failed, "lower")
	}
	if !pwdDigitCheck(s) {
		failed = append(failed, "digit")
	}
	if !pwdSpecialCheck(s) {
		failed = append(failed, "special")
	}
	if !pwdStrongCheck(s) {
		failed = append(failed, "strong")
	}
	return failed
}

// passwordSubMsgs 按「语言 → 子规则名 → 文案」二级映射。
//
// 子规则名 → 错误描述（与前端 validatePassword 6 个 return 文案一一对应）。
// "default" 是兜底文案（理论上走不到，但保险）。
var passwordSubMsgs = map[string]map[string]string{
	"zh": {
		"len":     "密码长度需为 12-24 位",
		"upper":   "需包含大写字母",
		"lower":   "需包含小写字母",
		"digit":   "需包含数字",
		"special": "需包含特殊字符",
		"strong":  "大写、小写、数字、特殊字符至少需包含 3 种",
		"default": "密码格式不合法",
	},
	"en": {
		"len":     "password length must be 12-24",
		"upper":   "must contain an uppercase letter",
		"lower":   "must contain a lowercase letter",
		"digit":   "must contain a digit",
		"special": "must contain a special character",
		"strong":  "must contain at least 3 of uppercase, lowercase, digit and special",
		"default": "invalid password",
	},
}

// ===== 校验原子函数 =====

// 密码字符分类（与前端完全一致，便于跨端对齐）。
const (
	PwdUpperChars   = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	PwdLowerChars   = "abcdefghijklmnopqrstuvwxyz"
	PwdDigitChars   = "0123456789"
	PwdSpecialChars = "!@#$%^&*()_+-=[]{}|;:,.~"
)

// 长度边界。
const (
	PwdMinLen = 12
	PwdMaxLen = 24
)

// MinPwdCategories 至少满足几类才「强度合格」。
const MinPwdCategories = 3

// pwdLenCheck 长度 12-24。
func pwdLenCheck(s string) bool {
	return len(s) >= PwdMinLen && len(s) <= PwdMaxLen
}

// pwdUpperCheck 至少含 1 个大写字母。
func pwdUpperCheck(s string) bool {
	return indexAny(s, PwdUpperChars) >= 0
}

// pwdLowerCheck 至少含 1 个小写字母。
func pwdLowerCheck(s string) bool {
	return indexAny(s, PwdLowerChars) >= 0
}

// pwdDigitCheck 至少含 1 个数字。
func pwdDigitCheck(s string) bool {
	return indexAny(s, PwdDigitChars) >= 0
}

// pwdSpecialCheck 至少含 1 个特殊字符。
func pwdSpecialCheck(s string) bool {
	return indexAny(s, PwdSpecialChars) >= 0
}

// pwdStrongCheck 4 类中至少满足 3 类（长度由 pwdLenCheck 提前保证）。
func pwdStrongCheck(s string) bool {
	cats := 0
	for _, set := range []string{PwdUpperChars, PwdLowerChars, PwdDigitChars, PwdSpecialChars} {
		if indexAny(s, set) >= 0 {
			cats++
		}
	}
	return cats >= MinPwdCategories
}

// indexAny 返回 s 中任一字符出现在 chars 里的索引；都找不到返回 -1。
//
// 等价于 strings.ContainsAny 的前缀检查版，但避免每次循环都构造布尔返回值。
func indexAny(s, chars string) int {
	for i, c := range s {
		for _, k := range chars {
			if c == k {
				return i
			}
		}
	}
	return -1
}
