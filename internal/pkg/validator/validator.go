// Package validator 封装 go-playground/validator/v10 并提供多语言翻译。
//
// 翻译基于 [go-playground/validator/v10/translations] 官方支持，
// 不需要手写每条 tag 的翻译。
//
// 自定义规则推荐通过 New(WithRule(...)) 注入，避免 init 副作用。
package validator

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/locales/en"
	"github.com/go-playground/locales/zh"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	enTranslations "github.com/go-playground/validator/v10/translations/en"
	zhTranslations "github.com/go-playground/validator/v10/translations/zh"

	"github.com/cuiyuanxin/roc_way/internal/pkg/errcode"
)

// Rule 自定义验证规则。
type Rule struct {
	Tag       string         // 验证 tag 名（如 "mobile"）
	Fn        validator.Func // 验证函数
	ZHMessage string         // 中文错误模板，{0} 会被替换为字段名
	ENMessage string         // 英文错误模板
}

// Option 配置项。
type Option func(*Validator)

// WithRule 注册一条自定义规则。
// 多次注册同名 tag，后者会覆盖前者。
func WithRule(r Rule) Option {
	return func(vs *Validator) {
		vs.registerRule(r)
	}
}

// WithRules 批量注册自定义规则。
func WithRules(rs ...Rule) Option {
	return func(vs *Validator) {
		for _, r := range rs {
			vs.registerRule(r)
		}
	}
}

// Validator 包装 validator/v10，提供多语言翻译。
type Validator struct {
	v          *validator.Validate
	zh         ut.Translator
	en         ut.Translator
	tagHooks   map[string]TagTranslateFunc // 业务侧注入的 per-tag 细分翻译回调
	tagHooksMu sync.RWMutex                // 保护 tagHooks 并发读写
}

// TagTranslateFunc per-tag 细分翻译回调。
//
// 入参：
//   - fe: validator.FieldError，含 Tag/StructField/Param 等元信息
//   - fieldVal: fe 对应的字段值（reflect.Value）
//   - lang: "zh" / "en"
//
// 返回值：翻译好的错误文案。返回 "" 时降级走 validator 默认翻译。
//
// 使用场景：单 tag 内做分段校验（dto password = 单 tag + 内部分段校验），
// 业务侧 fn 只返回 false，具体哪一段失败的细分消息由 hook 重新扫一遍字段值确定。
type TagTranslateFunc func(fe validator.FieldError, fieldVal reflect.Value, lang string) string

// RegisterTagHook 注册某条 tag 的细分翻译回调。
//
// 并发安全：内部 RWMutex 保护。同 tag 后注册覆盖前注册。
func (vs *Validator) RegisterTagHook(tag string, fn TagTranslateFunc) {
	vs.tagHooksMu.Lock()
	defer vs.tagHooksMu.Unlock()
	if vs.tagHooks == nil {
		vs.tagHooks = make(map[string]TagTranslateFunc)
	}
	vs.tagHooks[tag] = fn
}

// New 创建 Validator 实例，复用 gin 全局 validator engine，注册中英文默认翻译 + 内置 fieldmatch。
//
// 设计要点：
//
//   - **复用** binding.Validator.Engine() 的全局单例，让 dto 的 binding tag
//     与本 Validator 的 Struct() 校验走同一个 engine，规则只注册一次。
//
//   - 不创建新的 validator.Validate 实例（避免 dto 的 binding tag 失效）。
//
//     用法示例：
//     v := validator.New(
//     validator.WithRule(validator.Rule{
//     Tag: "mobile",
//     Fn: func(fl validator.FieldLevel) bool { ... },
//     ZHMessage: "{0} 手机号格式不正确",
//     ENMessage: "{0} mobile format invalid",
//     }),
//     )
func New(opts ...Option) *Validator {
	uni := ut.New(en.New(), zh.New())
	zhTr, _ := uni.GetTranslator("zh")
	enTr, _ := uni.GetTranslator("en")

	// 关键：复用 gin 全局 validator engine，让 dto binding tag 与本包 Struct() 校验共享同一份规则。
	v, ok := binding.Validator.Engine().(*validator.Validate)
	if !ok {
		// 理论上 gin 默认 engine 一定是 *validator.Validate，这里兜底降级。
		v = validator.New()
	}
	_ = zhTranslations.RegisterDefaultTranslations(v, zhTr)
	_ = enTranslations.RegisterDefaultTranslations(v, enTr)

	vs := &Validator{v: v, zh: zhTr, en: enTr}

	// 内置 fieldmatch
	_ = vs.v.RegisterValidation("fieldmatch", FieldMatch)
	vs.registerTranslationLocked("fieldmatch", "{0} 格式不匹配", "{0} format invalid")

	// 应用用户配置
	for _, opt := range opts {
		opt(vs)
	}
	return vs
}

// RegisterRule 注册一条自定义规则（公开 API）。
// 通常在 dto 包中提供 RegisterAll(v) 函数集中调用。
func (vs *Validator) RegisterRule(r Rule) {
	vs.registerRule(r)
}

// registerRule 注册自定义规则到 gin 全局 validator engine。
//
// 不加锁的原因：
//   - validator/v10 内部的 RegisterValidation / RegisterTranslation 自身有读写锁保护，
//     并发安全且「后注册者覆盖前注册者」，不会 panic。
//   - 本方法主要用于 New() 初始化阶段调用，启动后基本不再被触发。
func (vs *Validator) registerRule(r Rule) {
	_ = vs.v.RegisterValidation(r.Tag, r.Fn)
	vs.registerTranslationLocked(r.Tag, r.ZHMessage, r.ENMessage)
}

func (vs *Validator) registerTranslationLocked(tag, zhMsg, enMsg string) {
	_ = vs.v.RegisterTranslation(tag, vs.zh, func(ut ut.Translator) error {
		return ut.Add(tag, zhMsg, true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(tag, fe.Field())
		return t
	})
	_ = vs.v.RegisterTranslation(tag, vs.en, func(ut ut.Translator) error {
		return ut.Add(tag, enMsg, true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T(tag, fe.Field())
		return t
	})
}

// Engine 返回底层的 validator.Validate 实例。
func (vs *Validator) Engine() *validator.Validate { return vs.v }

// Bind 从 gin.Context 绑定并校验请求体到 dst，错误统一包装为 errcode.ErrInvalidParam。
// 根据 Accept-Language 头自动选择翻译语言（zh/en），默认中文。
func (vs *Validator) Bind(c *gin.Context, dst any) error {
	if err := c.ShouldBindJSON(dst); err != nil {
		return vs.mapErr(c, err, dst)
	}
	if err := vs.v.Struct(dst); err != nil {
		return vs.mapErr(c, err, dst)
	}
	return nil
}

func (vs *Validator) mapErr(c *gin.Context, err error, dst any) error {
	tr, lang := vs.pickTranslator(c)

	var verr validator.ValidationErrors
	if errors.As(err, &verr) {
		dstVal := reflect.ValueOf(dst)
		if dstVal.Kind() == reflect.Ptr {
			dstVal = dstVal.Elem()
		}
		t := dstVal.Type()
		fields := make([]string, 0, len(verr))
		for _, fe := range verr {
			structField, _ := t.FieldByName(fe.StructField())
			name := structField.Tag.Get("json")
			if name == "" {
				name = fe.StructField()
			}
			fields = append(fields, fmt.Sprintf("%s(%s)", name, vs.translateField(fe, dstVal, tr, lang)))
		}
		prefix := "字段不合法"
		if lang == "en" {
			prefix = "invalid fields"
		}
		return errcode.New(errcode.ErrInvalidParam.WithMessage(prefix+": "+strings.Join(fields, ", ")), err)
	}
	return errcode.New(errcode.ErrInvalidParam.WithMessage(err.Error()), err)
}

// translateField 单字段翻译：先查 tag hook（业务侧注册的细分翻译），
// 没注册或 hook 返回 "" 时降级走 validator 默认翻译。
func (vs *Validator) translateField(fe validator.FieldError, dstVal reflect.Value, tr ut.Translator, lang string) string {
	vs.tagHooksMu.RLock()
	hook, ok := vs.tagHooks[fe.Tag()]
	vs.tagHooksMu.RUnlock()
	if ok {
		if msg := hook(fe, dstVal.FieldByName(fe.StructField()), lang); msg != "" {
			return msg
		}
	}
	return fe.Translate(tr)
}

func (vs *Validator) pickTranslator(c *gin.Context) (ut.Translator, string) {
	al := c.GetHeader("Accept-Language")
	if strings.HasPrefix(strings.ToLower(al), "en") {
		return vs.en, "en"
	}
	return vs.zh, "zh"
}

// FieldMatch 内置正则验证函数（导出供 gin default validator 启动时注册）。
// 用法：binding:"required,fieldmatch=REGEX[:错误消息]"
func FieldMatch(fl validator.FieldLevel) bool {
	field := fl.Field()
	regex, _ := splitFieldMatchParam(fl.Param())
	if regex == "" {
		return true
	}
	matched, _ := regexp.MatchString(regex, field.String())
	return matched
}

// splitFieldMatchParam 解析 fieldmatch 参数，格式：REGEX 或 REGEX:错误消息。
func splitFieldMatchParam(param string) (regex, errMsg string) {
	depth := 0
	for i, c := range param {
		switch c {
		case '[':
			depth++
		case ']':
			depth--
		case ':':
			if depth == 0 {
				return param[:i], param[i+1:]
			}
		}
	}
	return param, ""
}
