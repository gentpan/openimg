package i18n

// The canonical catalogue.
//
// Chinese is the source of truth: every message starts here, and en.go is
// checked against it by TestCataloguesAgree. Adding a message means adding it
// to both — the test fails otherwise, which is the point.
//
// Keys are 模块.场景. They are also what an API consumer sees when a key is
// missing, so they should read as something, not as m17.
//
// Two rules the whole file depends on:
//
//   - translate() ends in fmt.Sprintf, so the placeholders are positional. The
//     two languages must take the SAME arguments in the SAME order. Where
//     English wanted a different word order, the sentence was rewritten rather
//     than the arguments reordered.
//   - %w is not valid for Sprintf. Call sites that wrapped an error with
//     fmt.Errorf pass err.Error() into T() instead; the catalogue carries %s.
var zh = map[string]string{
	// Upload
	"upload.email_unverified": "请先验证邮箱后再上传",
	"upload.suspended":        "账号已被停用",
	"upload.daily_limit":      "今日上传数量已达上限",
	"upload.too_large":        "文件超过大小上限 %.1f MB",
	"upload.missing_field":    "缺少上传文件字段 file",
	"upload.read_failed":      "读取上传内容失败：%s",
	"upload.format_denied":    "当前用户组不允许上传 %s 格式",
	"upload.rate_limited":     "上传过于频繁，请稍后再试",
	"upload.key_failed":       "生成存储路径失败，请重试",

	// Storage
	"storage.unavailable":           "存储不可用：%s",
	"storage.unavailable_bare":      "存储不可用",
	"storage.quota":                 "空间不足：需要 %s，剩余 %s",
	"storage.write_failed":          "写入存储失败：%s",
	"storage.write_variant_failed":  "写入衍生格式 %s 失败：%s",
	"storage.local_write_failed":    "写入本地存储失败：%s",
	"storage.local_verify_mismatch": "本地存储读写内容不一致",
	"storage.platform_pool":         "平台存储池",
	"storage.s3.unreachable":        "无法访问 bucket %q：%s",
	"storage.s3.test_write_failed":  "写入测试对象失败（凭据可能只有只读权限）：%s",
	"storage.s3.test_read_failed":   "读回测试对象失败：%s",
	"storage.s3.test_body_failed":   "读取测试对象内容失败：%s",
	"storage.s3.test_mismatch":      "测试对象内容不一致，bucket 可能被其他服务占用",

	// Auth
	"auth.not_authenticated": "尚未登录",
	"auth.bad_params":        "参数错误",
	"auth.email_invalid":     "请输入有效的邮箱",
	"auth.email_taken":       "该邮箱已注册，请直接登录",
	"auth.credentials_short": "请填写邮箱和至少 8 位的密码",
	"auth.name_required":     "请填写昵称",
	"auth.name_too_long":     "昵称最多 32 个字符",
	"auth.otp_required":      "请输入 6 位邮箱验证码",
	"auth.otp_needed":        "需要 6 位邮箱验证码",
	"auth.password_short":    "新密码至少 8 位",
	"auth.password_hash":     "密码处理失败",
	"auth.native_code":       "登录码无效或已过期，请重新登录",

	// Account
	"account.not_found":         "账号不存在",
	"account.delete.mismatch":   "确认文本与账号邮箱不一致",
	"account.delete.last_admin": "这是最后一个管理员账号，不能删除",
	"account.delete.failed":     "删除失败：%s",

	// Email codes
	"otp.not_configured":     "邮件服务未配置，无法发送验证码",
	"otp.rate_limited":       "请稍后再试，距上次发送不到 1 分钟",
	"otp.invalid_or_expired": "验证码无效或已过期",
	"otp.too_many_attempts":  "尝试次数过多，请重新申请验证码",
	"otp.incorrect":          "验证码错误",
	"otp.purpose_unknown":    "不支持的验证用途",

	// 验证码用途，拼进邮件标题里用
	"otp.purpose.password": "修改密码",
	"otp.purpose.passkey":  "添加 Passkey",
	"otp.purpose.purge":    "清空图库",
	"otp.purpose.storage":  "修改存储配置", "otp.purpose.reset": "重置密码",
	"ai.disabled":          "本站未开启 AI 生成",
	"ai.prompt_required":   "请先描述你想要的画面",
	"ai.prompt_too_long":   "描述太长了，请精简到 1000 字以内",
	"ai.daily_limit":       "今天的生成次数已用完，明天再来",
	"ai.no_credits":        "本月生成次数已用完，签到可以再领",
	"ai.submit_failed":     "提交生成失败：%s",
	"otp.purpose.register": "注册",
	"otp.purpose.login":    "登录",

	// Email bodies
	"email.otp.subject":    "Openimg %s验证码: %s",
	"email.otp.heading":    "Openimg 验证码",
	"email.otp.body":       "请在登录页输入下方验证码完成登录。验证码 %d 分钟内有效。",
	"email.otp.warning":    "如果不是你本人操作，请忽略此邮件。任何人（包括 Openimg 工作人员）都不会向你索要这个验证码。",
	"email.footer.tagline": "Openimg · 免费图床",

	// Passkeys
	"passkey.otp_required":     "需要邮箱验证码",
	"passkey.none_registered":  "此账号未绑定 Passkey",
	"passkey.login_failed":     "登录失败",
	"passkey.register_expired": "注册会话已过期，请重新尝试",
	"passkey.login_expired":    "登录会话已过期，请重新尝试",

	// OAuth
	"oauth.no_user_id":       "provider %s 不支持 / 没拿到 user id",
	"oauth.linked_elsewhere": "此 %s 账号已绑定到其他用户 (%s)",
	"oauth.unlink_last":      "无法解绑：这是你唯一的登录方式，请先设置密码或绑定其他账号",

	// Image processing
	"image.not_found":          "图片不存在",
	"image.too_large":          "图片尺寸 %dx%d 超出上限 %dx%d",
	"image.format_unsupported": "不支持的图片格式：%s",
	"imageproc.decode_failed":  "解码失败：%s",
	"imageproc.rotate_failed":  "旋转失败：%s",
	"imageproc.resize_failed":  "缩放失败：%s",
	"imageproc.strip_failed":   "移除元数据失败：%s",
	"imageproc.crop_failed":    "裁剪头像失败：%s",
	"imageproc.encode_failed":  "编码头像失败：%s",

	// Derivative sizes
	"variant.size_unsupported": "不支持的尺寸",
	"variant.read_failed":      "读取原图失败：%s",
	"variant.generate_failed":  "生成失败：%s",
	"variant.no_space":         "空间不足，无法生成该尺寸（需要 %s）",
	"variant.not_worth_it":     "请求的尺寸不小于原图，无需生成",

	// Avatars
	"avatar.choose_image": "请选择一张图片",
	"avatar.too_large":    "头像不能超过 8 MB",
	"avatar.read_failed":  "读取失败：%s",
	"avatar.bad_format":   "不是受支持的图片格式",

	// Gallery
	"gallery.purge_otp_required": "清空图库需要邮箱验证码",
	"gallery.bad_image_id":       "无效的图片 ID",
	"gallery.no_selection":       "没有选中任何图片",

	// Share page
	"share.link_not_found": "链接不存在或已失效",
	"share.image_blocked":  "该图片已被屏蔽",
	"share.bad_reaction":   "不支持的表情",

	// Reports
	"report.received":           "举报已收到，我们会尽快处理",
	"report.bad_image_id":       "image_id 无效",
	"report.reason_required":    "请选择举报类型，或填写说明",
	"report.rate_limited":       "举报过于频繁，请稍后再试",
	"report.category.porn":      "涉黄内容",
	"report.category.copyright": "侵犯版权",
	"report.category.violence":  "涉恐或血腥",
	"report.category.privacy":   "个人隐私",
	"report.category.fraud":     "诈骗或恶意链接",
	"report.category.other":     "其他违规",

	// API tokens
	"token.limit_reached": "有效 Token 数量已达上限，请先删除不用的",
	"token.shown_once":    "此 Token 只显示这一次，请立即保存",
	"token.bad_id":        "token id 无效",
	"token.not_found":     "Token 不存在",

	// Upload preferences
	"prefs.bad_upload_mode":    "upload_mode 必须是 optimized 或 original",
	"prefs.bad_variant_format": "variant_format 必须是 none / webp / avif",
	"prefs.bad_width_preset":   "不支持的宽度预设",
	"prefs.bad_thumb_width":    "缩略图宽度不在可选范围内",
	"prefs.bad_thumb_format":   "缩略图格式只支持 WebP、AVIF 或 JPEG",
	"prefs.serialize_failed":   "序列化失败",
	"prefs.save_failed":        "保存失败",

	// Daily check-in
	"checkin.already_done": "今天已经签到过了",
	"checkin.no_reward":    "当前用户组未配置签到奖励",
	"checkin.capped":       "签到成功，但空间已达上限，本次未发放",

	// 空间流水的 reason —— 普通用户在「空间明细」里看得到，不是内部日志
	"quota.reason.upload":             "上传 %s",
	"quota.reason.upload_dedup":       "上传 %s（秒传）",
	"quota.reason.upload_refund":      "上传失败回退",
	"quota.reason.dedup_refund":       "秒传失败回退",
	"quota.reason.delete_image":       "删除图片 %s",
	"quota.reason.bulk_delete":        "批量删除 %d 张图片",
	"quota.reason.generate_size":      "生成 %s 尺寸",
	"quota.reason.generate_refund":    "生成尺寸失败回退",
	"quota.reason.signup_bonus":       "注册赠送空间",
	"quota.reason.initial_backfill":   "初始空间（账本补录）",
	"quota.reason.referral_invitee":   "受邀注册奖励（来自 %s）",
	"quota.reason.referral_referrer":  "邀请 %s 注册奖励",
	"quota.reason.admin_adjust":       "管理员手动调整",
	"quota.reason.checkin":            "每日签到（连续 %d 天）",
	"quota.reason.checkin_week":       "每日签到（连续 %d 天 · 满周奖励）",
	"quota.reason.checkin_month":      "每日签到（连续 %d 天 · 满月奖励）",
	"quota.reason.checkin_week_month": "每日签到（连续 %d 天 · 满周 + 满月奖励）",

	// Short links
	"shortlink.failed": "生成短链失败",

	// Rate limits
	"ratelimit.too_frequent": "操作过于频繁",

	// Endpoint validation
	"endpoint.required":          "endpoint 不能为空",
	"endpoint.malformed":         "endpoint 格式无效：%s",
	"endpoint.scheme_required":   "endpoint 必须以 http:// 或 https:// 开头",
	"endpoint.https_required":    "endpoint 必须使用 https（明文 http 会在传输中泄露你的密钥）",
	"endpoint.host_missing":      "endpoint 缺少主机名",
	"endpoint.host_unresolvable": "无法解析主机 %q：%s",
	"endpoint.loopback":          "endpoint 指向本机地址（%s），不被允许",
	"endpoint.private":           "endpoint 指向内网地址（%s），不被允许",
	"endpoint.link_local":        "endpoint 指向链路本地地址（%s），不被允许",
	"endpoint.unspecified":       "endpoint 指向未指定地址（%s），不被允许",
	"endpoint.multicast":         "endpoint 指向组播地址（%s），不被允许",
	"endpoint.cgnat":             "endpoint 指向运营商级 NAT 地址（%s），不被允许",
	"endpoint.public_required":   "公开访问地址不能为空",
	"endpoint.public_malformed":  "公开访问地址格式无效：%s",
	"endpoint.public_scheme":     "公开访问地址必须以 http:// 或 https:// 开头",
	"endpoint.public_domain":     "公开访问地址缺少域名",

	// Storage locations
	"profile.byos_denied":       "当前用户组不允许绑定自有存储",
	"profile.no_master_key":     "服务端未配置存储加密密钥，无法安全保存凭据，请联系管理员",
	"profile.limit_reached":     "已达到存储位置数量上限",
	"profile.test_failed":       "连接测试失败：%s",
	"profile.not_ready":         "该存储当前不可用，请先测试连接",
	"profile.backup_as_default": "备份存储不能作为默认上传目标",
	"profile.platform_backup":   "平台存储的备份关系由管理员配置",
	"profile.bad_backup_of_id":  "backup_of_id 无效",
	"profile.backup_self":       "不能把存储设为自己的备份",
	"profile.source_not_found":  "源存储不存在",
	"profile.source_is_backup":  "源存储本身已是备份，不能再作为备份源",
	"profile.backup_exists":     "该存储已经有一个备份目标了",
	"profile.platform_locked":   "平台存储不可删除",
	"profile.has_images":        "该存储下还有图片，删除后这些图片将无法访问",
	"profile.bad_id":            "存储 id 无效",
	"profile.not_found":         "存储位置不存在",
	"profile.platform_readonly": "无权修改平台存储",
	"profile.bucket_required":   "bucket 不能为空",
	"profile.keys_required":     "access key 与 secret key 不能为空",
	"profile.removed_location":  "已移除的存储",
	"profile.platform":          "平台空间",

	// Tier descriptions
	"tier.admin":   "管理员（无限制）",
	"tier.trusted": "受信任用户（长期活跃）",
	"tier.free":    "注册免费用户",

	// 以下只出现在管理后台，或发给站长的举报通知邮件里。
	// 不需要英文后台的话，整段删掉即可，不会牵连上面的用户端文案。
	"admin.report.bad_id":        "report id 无效",
	"admin.report.not_found":     "举报不存在",
	"admin.report.bad_action":    "action 必须是 dismiss / block / block_and_ban",
	"admin.image.bad_id":         "image id 无效",
	"admin.user.bad_id":          "user id 无效",
	"admin.user.not_found":       "用户不存在",
	"admin.user.is_admin":        "不能封禁管理员账号",
	"admin.reconcile_failed":     "对账失败：%s",
	"admin.owner_deleted":        "（账号已删除）",
	"admin.storage.missing":      "尚未配置平台存储",
	"admin.storage.thumb_domain": "缩略图域名：%s",
	"admin.oauth.no_master_key":  "服务端未配置 STORAGE_MASTER_KEY，无法加密保存 OAuth 密钥；请改用环境变量配置",
	"admin.oauth.bad_provider":   "provider 必须是 google 或 github",
	"admin.oauth.secret_needed":  "填写 Client ID 时必须同时提供 Client Secret",
	"admin.oauth.from_console":   "已配置（后台）",
	"admin.oauth.from_env":       "已配置（环境变量）",

	"admin.mail.subject":     "[Openimg] 新举报：%s",
	"admin.mail.no_note":     "（举报人未填写补充说明）",
	"admin.mail.no_contact":  "未留",
	"admin.mail.anonymous":   "匿名访客",
	"admin.mail.signed_in":   "已登录用户",
	"admin.mail.heading":     "收到新的举报",
	"admin.mail.reason":      "举报理由",
	"admin.mail.image":       "被举报图片",
	"admin.mail.spec":        "规格",
	"admin.mail.reporter":    "举报人",
	"admin.mail.contact":     "联系方式",
	"admin.mail.direct_link": "图片直链 · 请自行判断是否打开",
	"admin.mail.cta":         "前往后台处理",
	"admin.mail.footer":      "Openimg · 举报通知",
}
