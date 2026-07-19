package model

// Type constants mirror the Java `Types` enum values.
const (
	TypeTag          = "tag"
	TypeCategory     = "category"
	TypeArticle      = "post"
	TypePublish      = "publish"
	TypePage         = "page"
	TypeDraft        = "draft"
	TypeLink         = "link"
	TypeImage        = "image"
	TypeFile         = "file"
	TypeCommentsFreq = "comments:frequency"
	TypeAttachURL    = "attach_url"
	TypeBlockIPs     = "site_block_ips"
)

// Log action labels mirror the Java `LogActions` enum.
const (
	LogLogin      = "登录后台"
	LogUpPwd      = "修改密码"
	LogUpInfo     = "修改个人信息"
	LogDelArticle = "删除文章"
	LogDelPage    = "删除页面"
	LogSysBackup  = "系统备份"
	LogSysSetting = "保存系统设置"
	LogInitSite   = "初始化站点"
)

// Web-wide constants mirror the Java `WebConst`.
const (
	LoginSessionKey = "login_user"
	UserInCookie    = "S_L_ID"
	MaxPosts        = 9999
	MaxPage         = 100
	MaxTextCount    = 200000
	MaxTitleCount   = 200
	HitExceed       = 10
	MaxFileSize     = 1048576 // 1MB
	BadRequest      = "BAD REQUEST"
)
