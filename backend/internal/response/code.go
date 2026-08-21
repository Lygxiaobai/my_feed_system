package response

// Code 是错误码，采用《阿里巴巴 Java 开发手册》(黄山版) 的错误码规约：
// 字符串类型、共 5 位，由「错误产生来源」+「四位数字编号」组成。
//
//	A —— 错误来源于用户，例如参数不合法、登录过期、上传文件超限；
//	B —— 错误来源于当前系统，例如业务逻辑出错、程序健壮性不足；
//	C —— 错误来源于第三方服务，例如数据库、消息队列、缓存出错。
//
// 编号在大类之间预留 100 的步长，便于日后在类内追加而不与既有编号冲突。
// 手册强调错误码不体现错误等级、也与 HTTP 状态码无关：等级由日志级别表达，
// HTTP 状态码由传输语义表达，三者各司其职。
//
// 下列编号是本项目按手册规约「先到先得」登记的子集，一经使用即固定，只做追加不做重排。
type Code string

const (
	// Success 表示一切正常。手册规定正常返回填充五个零。
	Success Code = "00000"

	// ---------- A：用户端错误 ----------

	// UserError 是用户端错误的一级宏观错误码，仅在无法更具体归类时兜底使用。
	UserError Code = "A0001"

	// A01xx 用户注册错误
	UserRegisterError Code = "A0100"
	UsernameTaken     Code = "A0111"

	// A02xx 用户登录异常
	UserLoginError  Code = "A0200"
	AccountNotFound Code = "A0201"
	CredentialWrong Code = "A0202"
	LoginExpired    Code = "A0230"
	LoginRequired   Code = "A0231"

	// A03xx 访问权限异常
	AccessDenied Code = "A0300"

	// A04xx 用户请求参数错误
	ParamError          Code = "A0400"
	ParamMissing        Code = "A0402"
	ParamFormatError    Code = "A0410"
	ResourceNotFound    Code = "A0420"
	UploadTypeInvalid   Code = "A0421"
	UploadTooLarge      Code = "A0422"
	InsufficientBalance Code = "A0431"

	// A05xx 用户请求服务异常
	RequestServiceError Code = "A0500"
	RateLimited         Code = "A0501"
	DuplicatedRequest   Code = "A0502"
	// DMQuotaExceeded 未互关时已经用掉唯一一条私信额度。
	DMQuotaExceeded Code = "A0503"

	// ---------- B：当前系统错误 ----------

	// SystemError 是系统执行出错的一级宏观错误码。
	SystemError Code = "B0001"
	// SystemTimeout 对应系统执行超时。
	SystemTimeout Code = "B0100"

	// ---------- C：第三方服务错误 ----------

	// ThirdPartyError 是调用第三方服务出错的一级宏观错误码。
	ThirdPartyError Code = "C0001"
	// MiddlewareError 对应中间件服务出错（二级）。
	MiddlewareError Code = "C0100"
	// MessageQueueError 对应消息队列出错（三级）。
	MessageQueueError Code = "C0110"
	// CacheError 对应缓存服务出错（三级）。
	CacheError Code = "C0120"
	// DatabaseError 对应数据库服务出错（二级）。
	DatabaseError Code = "C0300"
)

// defaultUserTips 是各错误码对应的默认提示信息（user_tip）。
//
// 手册强制要求：错误码不能直接输出给用户作为提示信息使用；
// 堆栈、错误信息、错误码、提示信息是互相关联但不可越俎代庖的四样东西。
// 因此这里的文案面向终端用户，不包含任何内部实现细节，
// 真正的 error_message 与堆栈只写入日志。
var defaultUserTips = map[Code]string{
	Success: "",

	UserError:         "请求有误，请稍后重试",
	UserRegisterError: "注册失败，请检查填写内容",
	UsernameTaken:     "该用户名已被占用",

	UserLoginError:  "登录失败，请重试",
	AccountNotFound: "账号不存在",
	CredentialWrong: "用户名或密码错误",
	LoginExpired:    "登录已过期，请重新登录",
	LoginRequired:   "请先登录",

	AccessDenied: "没有操作该内容的权限",

	ParamError:          "请求参数有误",
	ParamMissing:        "缺少必填参数",
	ParamFormatError:    "参数格式不正确",
	ResourceNotFound:    "内容不存在或已被删除",
	UploadTypeInvalid:   "文件类型不支持",
	UploadTooLarge:      "文件超出大小限制",
	InsufficientBalance: "积分不足",

	RequestServiceError: "请求处理失败，请稍后重试",
	RateLimited:         "操作过于频繁，请稍后再试",
	DuplicatedRequest:   "请勿重复提交",
	DMQuotaExceeded:     "互相关注后才能继续发私信",

	SystemError:   "服务暂时不可用，请稍后重试",
	SystemTimeout: "服务响应超时，请稍后重试",

	ThirdPartyError:   "服务暂时不可用，请稍后重试",
	MiddlewareError:   "服务暂时不可用，请稍后重试",
	MessageQueueError: "服务暂时不可用，请稍后重试",
	CacheError:        "服务暂时不可用，请稍后重试",
	DatabaseError:     "服务暂时不可用，请稍后重试",
}

// UserTip 返回错误码的默认提示信息，调用方未提供自定义文案时使用。
func (c Code) UserTip() string {
	if tip, ok := defaultUserTips[c]; ok {
		return tip
	}
	return defaultUserTips[SystemError]
}

// Source 返回错误来源标识（A/B/C），用于日志聚合与告警分流。
// Success 没有来源，返回空字符串。
func (c Code) Source() string {
	if c == Success || len(c) == 0 {
		return ""
	}
	return string(c[0])
}
