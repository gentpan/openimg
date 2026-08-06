/**
 * 中文目录，也是这套字典的事实来源。
 *
 * 结构上的两条约定：
 *  1. 会在两个以上位置出现的文案放进 `common`，各页面不再自建一份；
 *  2. 带变量的条目一律写成箭头函数，`en.ts` 里同名函数的参数表必须逐个对上
 *     —— 参数顺序由调用方决定，两种语言可以把变量放在句子的不同位置。
 *
 * 注意这里没有 `as const`：`index.ts` 把 `Dict` 定义成 `typeof zh`，一旦加上
 * `as const`，每个字符串都会收窄成字面量类型，`en: Dict` 会因为 "Save" 不是
 * "保存" 而编译失败——那正好废掉了 index.ts 想要的那道检查。
 */
export const zh = {
  common: {
    loading: "加载中…",
    noData: "暂无数据",

    save: "保存",
    saved: "已保存",
    saveFailed: "保存失败",
    cancel: "取消",
    cancelled: "已取消",
    close: "关闭",
    confirmChange: "确认修改",
    delete: "删除",
    deleteFailed: "删除失败",
    remove: "移除",
    edit: "编辑",
    copy: "复制",
    copied: "已复制",
    download: "下载",
    optional: "（可选）",
    verified: "已验证",
    backToHome: "返回首页",

    signIn: "登录",
    signUp: "注册",
    signOut: "退出",
    authRequired: (link: string) => `请先 ${link}`,
    authRequiredLink: "登录",

    email: "邮箱",
    displayName: "昵称",
    emailPassword: "邮箱 + 密码",
    newPassword: "新密码",
    enterAgain: "再输入一次",
    passwordMismatch: "两次输入不一致",
    setPassword: "设置密码",
    changePassword: "修改密码",
    addPasskey: "添加 Passkey",

    emailCode: "邮箱验证码",
    otpPlaceholder: "6 位数字",
    sendCode: "发送验证码",
    sendingCode: "发送中…",
    resend: "重新发送",
    otpSentTo: (email: string) => `验证码已发送到 ${email}，5 分钟内有效。`,

    upload: "上传",
    recentUploads: "最近上传",
    noUploadsYet: "还没有上传过图片",
    uploadFirst: "去上传第一张 →",
    availableStorage: "可用空间",
    shortLink: "短链",

    /** 数量、容量的通用短语。数字都写在串里，别再由组件另外渲染一遍。 */
    imageCount: (n: number) => `${n} 张`,
    countAndSize: (n: number, size: string) => `${n} 张 · ${size}`,
    days: (n: number) => `${n} 天`,
    lastUsed: (date: string) => `最近使用 ${date}`,

    pager: {
      prev: "上一页",
      next: "下一页",
    },
  },

  home: {
    highlight: {
      optimizeOrOriginal: "压缩或留原图",
      webpAvif: "WebP / AVIF",
      globalCdn: "全球 CDN",
      stripExif: "自动抹除 EXIF",
      ownBucket: "可绑自有 R2 / S3",
      freeForever: "永久免费",
    },
    hero: {
      badge: "永久免费的公益图床",
      titleLead: "图片托管，",
      titleAccent: "从此不用操心",
      subtitle: "拖进来就好。剩下的压缩、转码、分发，我们全包了。",
    },
    features: {
      sectionTitle: "为什么用 Openimg",
      sectionSubtitle: "做好一件事：让图片又小又快",
      optimize: {
        title: "自动压缩转换",
        desc: "上传即生成 WebP / AVIF 与多档缩略图，体积通常只剩三分之一，画质肉眼无差。",
      },
      cdn: {
        title: "Cloudflare CDN",
        desc: "内容寻址的不可变地址，边缘缓存一年。国内外访问都走最近的节点。",
      },
      privacy: {
        title: "隐私优先",
        desc: "自动剥离 EXIF 与 GPS 定位信息，重新编码去掉一切夹带内容，不会泄露你的拍摄地点。",
      },
      ownStorage: {
        title: "绑定自有存储",
        desc: "可以把图片直接存进你自己的 Cloudflare R2 或 S3，容量不受平台限制，随时可迁走。",
      },
      checkin: {
        title: "签到得空间",
        desc: "注册即送 1 GB，每天签到随机领 1–20 MB，连续 7 天额外奖励，邀请好友双方各得 200 MB。容量永久累加。",
      },
      tools: {
        title: "工具直连",
        desc: "生成 API Token 后可直接在 PicGo、Typora 或 curl 里使用，写文章时无需切换页面。",
      },
    },
    steps: {
      sectionTitle: "三步开始",
      sectionSubtitle: "不到一分钟",
      signUp: {
        title: "注册账号",
        desc: "邮箱、Google、GitHub 或 Passkey，验证邮箱后即可上传。",
      },
      drop: {
        title: "拖进图片",
        desc: "拖拽、点击选择，或直接 Ctrl+V 粘贴截图，支持批量。",
      },
      copy: {
        title: "复制链接",
        desc: "一键复制 URL / Markdown / HTML / BBCode，粘到任何地方。",
      },
    },
    integration: {
      sectionTitle: "接入你的工作流",
      sectionSubtitle: "在设置页生成 API Token 后即可使用",
      curlCardTitle: "curl",
      picgoCardTitle: "PicGo 自定义图床",
      picgoConfigLabel: {
        apiUrl: "API 地址",
        postField: "POST 参数名",
        customHeader: "自定义 Header",
        jsonPath: "JSON 路径",
      },
    },
    cta: {
      title: "现在就开始，一分钱都不用花",
      subtitle: "这是一个公益项目，没有会员，没有广告，也不会卖你的数据。",
      button: "免费注册，领 1 GB",
    },
  },

  auth: {
    /**
     * AuthShell 把 title 拼在品牌名前面（`{title} Openimg`），所以这三条是
     * 可前置的短语而不是完整句子。英文同样依赖后面的品牌名。
     */
    shell: {
      signIn: "登录",
      signUp: "注册",
      reset: "重置",
    },
    modePasskey: "Passkey",
    password: "密码",
    forgotPassword: "忘记密码？",
    signingIn: "登录中…",
    noAccountPrompt: "没账号？",
    haveAccountPrompt: "已有账号？",
    resetCodeSentNeutral:
      "如果这个邮箱已注册，验证码已经发出。为避免泄露账号是否存在，我们不区分两种情况。",
    nativeHandoffFailed: "无法把登录结果交回应用，请重试",

    register: {
      emailRequiredFirst: "请先填写邮箱",
      displayNamePlaceholder: "别人看到的名字，可重复",
      password: "密码（至少 8 位）",
      confirmPassword: "确认密码",
      submitBusy: "注册中…",
      submit: "创建账号",
    },

    reset: {
      emailRequired: "请输入邮箱",
      codeSentNotice:
        "如果这个邮箱已注册，验证码已经发出。为避免泄露账号是否存在，我们不区分两种情况。",
      submitBusy: "设置中…",
      submit: "设置新密码并登录",
      backToSignIn: "← 返回登录",
    },

    passkey: {
      cancelledOrFailed: "已取消或失败",
      email: "邮箱（可选 — 留空让浏览器选择 Passkey）",
      emailPlaceholder: "留空使用本机 Passkey 直接登录",
      submitBusy: "等待 Passkey…",
      submit: "用 Passkey 登录",
      hint: "会调用本机 Touch ID / Face ID / 安全密钥",
    },

    oauth: {
      divider: "或使用",
      google: "用 Google 账号登录",
      github: "用 GitHub 账号登录",
      passkey: "用 Passkey / 指纹登录",
    },
  },

  nav: {
    overview: "概览",
    gallery: "图库",
    refer: "邀请",
    admin: "管理",
    checkin: "签到",
    checkinHint: "点击签到，随机领取空间",
    checkedInTodayStreak: (days: number) => `今日已签到 · 连续 ${days} 天`,
    storagePillTitle: (used: string, total: string) => `已用 ${used} / 共 ${total}`,
    switchToViolet: "切换到紫色",
    switchToGreen: "切换到绿色",
    checkinCappedTitle: "空间已达上限",
    checkinCappedDetail: "今天的签到没有增加容量",
    checkinSuccessTitle: (amount: string) => `签到成功，+${amount}`,
    checkinSuccessDetail: "已永久累加到你的总容量",
    checkinFailed: "签到失败",
  },

  footer: {
    docs: "接入文档",
    /** 数值由组件单独渲染，这四条只是它后面的标签。 */
    statImages: (_n: number) => "张图片",
    statUsers: (_n: number) => "位用户",
    statStored: "已存储",
    statToday: "今日上传",
    rightsReserved: "版权所有",
  },

  dashboard: {
    greeting: (name: string) => `你好，${name}`,
    welcomeBack: "欢迎回来",
    streakBadge: (days: number) => `已连续签到 ${days} 天`,
    uploadCta: "上传图片",
    kpi: {
      availableStorageSub: (used: string, percent: number, total: string) =>
        `已用 ${used} · ${percent}% · 共 ${total}`,
      totalImages: "图片总数",
      uploadsTodaySub: (count: number, limit: number) => `今日上传 ${count} / ${limit}`,
      checkinStreak: "连续签到",
      checkedInToday: "今日已签到",
      notCheckedInToday: "今天还没签到",
      savedByOptimizing: "压缩省下",
      origToStored: (orig: string, stored: string) => `原始 ${orig} → 存储 ${stored}`,
      acrossAllImages: "全部图片累计",
    },
    earnTrend: {
      title: "空间获得趋势",
      subtitle: "最近 30 天 · 签到 / 邀请 / 赠送",
    },
    optimization: {
      title: "压缩效果",
      subtitle: (count: number) => `全部 ${count} 张图片`,
      calculating: "统计中…",
      actuallyStored: "实际占用",
      saved: "已省下",
      breakdown: (primary: string, variants: string, thumbs: string) =>
        `占用明细 · 主图 ${primary} · 全尺寸变体 ${variants} · 缩略图 ${thumbs}`,
      unclassified: (size: string) => `早期未拆分 ${size}`,
    },
    storageSplit: {
      title: "存储分布",
      subtitle: "平台空间与你自己绑定的存储",
      byFormat: "按格式",
    },
    activity: {
      title: "上传活动",
      subtitle: "最近 30 天 · 悬停查看当天上传张数与占用",
    },
    recent: {
      subtitle: "点击进入图库管理",
      viewAll: "查看全部 →",
    },
  },

  space: {
    title: "我的空间",
    tx: {
      signupGrant: "注册赠送",
      checkin: "每日签到",
      referral: "邀请奖励",
      adminGrant: "管理员调整",
      upload: "上传占用",
      deleteRefund: "删除释放",
    },
    quota: {
      title: "空间使用",
      totalAndPercent: (total: string, percent: number) => `共 ${total} · 已用 ${percent}%`,
      total: "总容量",
      available: "剩余可用",
      images: "图片数",
    },
    checkin: {
      cappedMsg: "签到成功，但空间已达上限，本次未发放",
      successMsg: (granted: string, days: number) => `签到成功，+${granted}，已连续 ${days} 天`,
      /** 数字由组件单独渲染，这条只是紧跟其后的单位。 */
      streakUnit: (_n: number) => "天连续",
      randomRange: (min: string, max: string) => `每天随机 ${min} – ${max}`,
      neverExpires: "永久累加，不会过期",
      doneToday: "今日已签",
      button: (min: string, max: string) => `签到领 ${min}–${max}`,
      monthProgress: "本月进度",
    },
    ledger: {
      title: "空间流水",
      total: (count: number) => `共 ${count} 条`,
      perPage: "每页",
      /** 中文的量词，英文没有对应词——为空时组件应当直接不渲染这个节点。 */
      perPageSuffix: "条",
      empty: "还没有记录",
    },
  },

  refer: {
    copyFailed: (error: string) => `复制失败：${error}`,
    title: "邀请有奖",
    heroHeadline: (bonus: string) => `邀请好友，双方各得 ${bonus} 空间`,
    heroSub: "把链接分享给朋友，他们注册成功后，你和他都立即到账存储空间。",
    yourCode: "你的推荐码",
    theyGet: "他们获得",
    theyGetBonus: (bonus: string) => `一次性 ${bonus} — 通过你的链接注册即到账`,
    theyGetTier: "立即解锁所有免费用户额度",
    youGet: "你获得",
    youGetBonus: (bonus: string) => `一次性 ${bonus} — 每位有效注册的好友`,
    youGetMore: "邀请越多，空间越多（受用户组上限约束）",
    statsHeading: "数据",
    stat: {
      invited: "已邀请用户",
      totalEarned: "累计获得空间",
      perReferral: "单次奖励",
    },
    historyHeading: "邀请记录",
    table: {
      user: "用户",
      signedUp: "注册时间",
      empty: "还没有人通过你的链接注册，去复制链接分享给朋友吧",
    },
  },

  upload: {
    emailUnverified: "你的邮箱还没有验证，验证后才能上传图片。",
    goVerify: "去验证 →",
    storageDetails: "空间明细 →",
    checkInForStorage: "去签到领空间 →",
    currentLimits: (tier: string) => `当前限制（${tier}）`,
    maxFileSize: "单文件上限",
    uploadedToday: "今日已传",
    uploadedTodayValue: (used: number, limit: number) => `${used} / ${limit} 张`,
    supportedFormats: "支持格式",
    recentHint: "点击查看详情与各格式链接",
    viewGallery: "全部图库 →",
    networkError: "网络错误，上传失败",
  },

  uploader: {
    dropHere: "松开即可上传",
    dragOrClick: "拖拽图片到这里，或点击选择",
    pasteHint: "支持 Ctrl+V 粘贴截图 · JPEG / PNG / WebP / GIF / AVIF",
    signInToUpload: "登录后即可上传图片",
    signUpPerk: "注册送 1 GB，每天签到还能随机领 1–20 MB",
    signUpFree: "免费注册",
  },

  uploadPanel: {
    finishedWithErrors: (done: number, failed: number) => `完成 ${done} 张，${failed} 张失败`,
    finished: (n: number) => `已上传 ${n} 张`,
    uploading: (done: number, total: number) => `上传中 ${done}/${total}`,
    copyAll: (n: number) => `复制 ${n} 条`,
    clearFinished: "清除已完成",
    clearQueueTitle: "清空队列",
    clear: "清空",
    dedupTitle: "服务器上已有相同内容，直接秒传",
    dedupBadge: "秒传",
    saved: (saved: string) => `省 ${saved}`,
    copyLink: "复制链接",
    retry: "重试",
    remove: "移除",
  },

  gallery: {
    title: "我的图库",
    searchPlaceholder: "搜索文件名",
    clearSelection: "取消全选",
    selectAllOnPage: "全选本页",
    selectedCount: (n: number) => `已选 ${n} 张`,
    loadedCount: (loaded: number, total: number) => `已加载 ${loaded} / ${total}`,
    deleteSelected: (n: number) => `删除选中 ${n} 张`,
    deleteSelectedConfirm: (n: number) =>
      `确定删除选中的 ${n} 张图片？删除后会释放对应空间，且无法恢复。`,
    deletedToast: (n: number) => `已删除 ${n} 张图片`,
    deletedToastDetail: "对应空间已释放",
    wipeSearchTitle: "删除所有匹配当前搜索的图片",
    wipeAllTitle: "删除图库中的全部图片",
    wipeSearch: "删除全部搜索结果",
    wipeAll: "清空图库",
    wipeProgress: (n: number) => `已删除 ${n} 张…`,
    wipeConfirmSearch: (n: number, query: string) =>
      `将删除匹配「${query}」的 ${n} 张图片，并释放对应空间。此操作不可恢复。`,
    wipeConfirmAll: (n: number) => `将删除全部 ${n} 张图片，并释放对应空间。此操作不可恢复。`,
    wipeDoneToast: (n: number) => `图库已清空，共删除 ${n} 张`,
    wipeDoneToastDetail: "空间已全部释放",
    wipeFailed: "清空未完成",
    wipeFailedDetail: (n: number) => `已删除 ${n} 张后中断`,
    emptySearch: "没有匹配的图片",
    loadMore: "加载更多",
    pageSizeTitle: "每页显示数量",
    pageSizeUnit: "张",
    select: "选择",
    deselect: "取消选择",
    blocked: "已屏蔽",
    sort: {
      newest: "最新上传",
      oldest: "最早上传",
      largest: "占用最大",
      smallest: "占用最小",
      widest: "分辨率最高",
      name: "文件名",
    },
  },

  imageDetail: {
    deleteConfirm: "确定删除这张图片？删除后会释放对应空间，且无法恢复。",
    dimensions: "尺寸",
    originalSize: "原始大小",
    storedSize: "存储占用",
    format: "格式",
    uploadedAt: "上传时间",
    generated: "已生成",
    originalVariant: "原图",
    otherSizes: "其他规格",
    otherSizesHint: "· 按需生成，生成后才占用空间",
    generateSize: (width: number) => `生成 ${width}px`,
    originalTitle: "未经压缩、保留 EXIF 的上传原件",
    originalLink: (ext: string) => `原图 ${ext}`,
  },

  share: {
    goHome: "前往 Openimg 首页 →",
    formatDirectLink: "直链",
    formatThumbnail: "缩略图",
    uploadedBy: (name: string) => `由 ${name} 上传`,
    views: (count: number) => `${count} 次访问`,
    copyrightNotice:
      "本站仅提供图片存储与分发服务，不拥有所展示内容的任何版权。所有图片版权归原作者或上传者所有。 如发现内容侵犯您的合法权益，请点击右上角举报按钮提交投诉，我们将在收到通知后尽快核实并删除相关内容。",
    shortLinkCopied: "短链已复制",
    copyShortLink: "复制短链",
    targetWeibo: "微博",
    targetEmail: "邮件",
    reaction: {
      like: "赞",
      clap: "鼓掌",
      party: "庆祝",
      sparkle: "惊艳",
      smile: "微笑",
      undo: (label: string) => `取消${label}`,
      failed: "操作失败",
    },
  },

  /** 举报对话框：图片详情页和分享页共用同一套。 */
  report: {
    imageTitle: "举报这张图片",
    dialogTitle: "举报图片",
    categoryLabel: "举报类型",
    detailsLabel: "补充说明",
    detailsPlaceholder: "请描述具体问题…",
    detailsOptionalPlaceholder: "补充说明（可选）",
    reasonPlaceholder: "请说明问题，至少 4 个字",
    contactPlaceholder: "联系方式（可选，便于回复处理结果）",
    notice: "管理员会收到通知并核实。滥用举报功能可能导致你的账号被限制。",
    noLogin: "无需登录",
    submit: "提交举报",
    submitted: "举报已提交",
    submittedDetail: "管理员会收到通知并尽快核实",
    category: {
      porn: "涉黄内容",
      copyright: "侵犯版权",
      violence: "涉恐或血腥",
      privacy: "个人隐私",
      fraud: "诈骗或恶意链接",
      other: "其他违规",
    },
  },

  settings: {
    title: "账号设置",
    profile: {
      title: "个人资料",
      subtitle: "头像和昵称会显示在页面顶部，昵称可以重复",
      avatarUpdated: "头像已更新",
      avatarUploadFailed: "头像上传失败",
      avatarRemoved: "头像已移除",
      removeFailed: "移除失败",
      avatarButtonTitle: "点击更换头像",
      avatarChangeOverlay: "更换",
      avatarHint:
        "点击左侧头像更换 · 支持 JPG / PNG / WebP / HEIC，自动裁成 256px 方图并转为 AVIF",
      displayNameChanged: (name: string) => `昵称已改为「${name}」`,
      displayNameCleared: "昵称已清空",
      displayNameToastDetail: "邮箱仍是你的账号标识",
      displayNamePlaceholder: "未设置，可留空",
      displayNameHint: "昵称可重复，邮箱才是账号标识",
    },
    storage: {
      title: "存储位置",
      subtitle: "默认存在平台空间；也可以绑定你自己的 R2 / S3，容量不受平台限制",
    },
    convert: {
      title: "图片压缩与转换",
      subtitle: "只影响之后上传的图片，已上传的不受影响",
    },
    apiTokens: {
      subtitle: "用于 PicGo、Typora、curl 等外部工具上传",
      docsLink: "怎么接入",
    },
    accountInfo: {
      title: "账号信息",
      role: "角色",
      tier: "用户组",
    },
    loginMethods: {
      title: "登录方式",
      subtitle: "可以同时绑定多种登录方式，任意一种都能登录此账号",
      passwordSet: "已设置",
      passwordNotSet: "未设置（使用其他方式登录）",
      changePasswordAction: "改密码",
      emailOtpNote: "验证后可用邮箱验证码登录",
      verifyEmail: "验证邮箱",
      verifyEmailComingSoon: "验证邮箱功能即将上线",
      connected: "已绑定",
    },
    oauth: {
      link: "绑定",
      unlink: "解绑",
      linked: (provider: string) => `已成功绑定 ${provider}`,
      unlinked: (provider: string) => `已解绑 ${provider}`,
      unlinkConfirm: (provider: string) =>
        `确定要解绑 ${provider}？解绑后这个 ${provider} 账号不能再用来登录。`,
    },
    password: {
      updated: "密码已更新",
      updatedDetail: "下次登录请使用新密码",
      dialogIntro: "需要邮箱验证码才能修改。",
      tooShort: "密码至少 8 位",
    },
    passkey: {
      subtitle: "用 Touch ID / Face ID / 安全密钥免密码登录",
      namePrompt: "给这个 passkey 起个名（如：MacBook Touch ID、iPhone）",
      added: "Passkey 添加成功",
      addedDetail: "以后可以用指纹或面容直接登录",
      deleteConfirm: (name: string) =>
        `删除 passkey「${name}」？删除后这个设备不能再用 passkey 登录。`,
      otpDetail: "验证通过后浏览器会请求你的指纹或面容。",
      addedOn: (date: string) => `添加于 ${date}`,
      empty: "还没有 Passkey，点下方按钮添加第一个。",
      waiting: "等待授权…",
      defaultDeviceName: "我的设备",
    },
    danger: {
      title: "危险操作",
      subtitle: "这里的操作不可撤销，请谨慎",
    },
  },

  storageProfiles: {
    endpoint: {
      r2: "识别为 Cloudflare R2 · 虚拟主机寻址",
      s3: "识别为 AWS S3 · 虚拟主机寻址",
      b2: "识别为 Backblaze B2 · 虚拟主机寻址",
      spaces: "识别为 DigitalOcean Spaces · 虚拟主机寻址",
      oss: "识别为阿里云 OSS · 虚拟主机寻址",
      cos: "识别为腾讯云 COS · 虚拟主机寻址",
      custom: "按自建 S3（MinIO 等）处理 · 路径寻址",
    },
    testPassed: "连接测试通过，可以保存了",
    badge: {
      default: "默认",
      platform: "平台",
      backup: "备份",
      unavailable: "不可用",
    },
    platformHosted: "平台托管",
    setDefault: "设为默认",
    setDefaultDone: "已设为默认上传目标",
    connectionOk: "连接正常",
    test: "测试",
    testConnection: "测试连接",
    deleteConfirm: (name: string) => `删除存储位置「${name}」？`,
    deleted: "已删除",
    addTitle: "添加自有存储",
    editTitle: (name: string) => `编辑 ${name}`,
    field: {
      name: "名称",
      namePlaceholder: "我的存储",
      endpointPlaceholder: "https://<account>.r2.cloudflarestorage.com 或 https://s3.example.com",
      keyPrefix: "Key 前缀（可选）",
      unchangedPlaceholder: "留空表示不修改",
      publicBaseUrl: "公开访问地址",
    },
    securityNote:
      "建议创建仅限该 bucket、仅有对象读写权限的令牌。密钥会以 AES-256-GCM 加密存储，页面上永不回显。",
    securityNoteEmphasis: "仅限该 bucket、仅有对象读写权限",
  },

  apiTokens: {
    deleteConfirm: (name: string) => `删除 Token「${name}」？使用它的工具会立即失效。`,
    shownOnce: "这个 Token 只显示这一次，请立即保存",
    gotIt: "知道了",
    revoked: "已失效",
    neverUsed: "从未使用",
    expiresAt: (date: string) => `到期 ${date}`,
    namePlaceholder: "Token 名称，如 PicGo",
    expiry: {
      never: "永不过期",
      days30: "30 天",
      days90: "90 天",
      year1: "1 年",
    },
    generate: "生成",
    scopeNote: "Token 只能用于上传和管理图片，不能修改账号设置、存储凭据或 Passkey。",
  },

  convertSettings: {
    savedDetail: "对之后上传的图片生效，已上传的不受影响",
    mode: {
      title: "上传模式",
      optimized: "优化模式",
      optimizedLine1: "重新编码，剥离 EXIF 与 GPS",
      optimizedLine2: "可限制最大宽度",
      optimizedLine3: "体积通常只剩 10–30%",
      original: "原图模式",
      originalLine1: "字节不变，保留全部 EXIF 与拍摄地点",
      originalLine2: "不压缩、不缩放",
      originalLine3: "占用空间显著更大",
    },
    badgeRecommended: "推荐",
    original: {
      warning:
        "原图会连同 EXIF 与 GPS 定位一起公开 —— 任何拿到链接的人都能读出拍摄地点、时间和设备。分享手机拍摄的照片前请确认这是你想要的。",
      warningEmphasis: "EXIF 与 GPS 定位",
      formatNote:
        "为安全起见，上传仍会校验真实格式（拒绝 SVG 与非图片），并强制以图片类型返回。",
    },
    width: {
      title: "最大宽度",
      keepOriginal: "保持原尺寸",
      disabledHint: "原图模式下不缩放",
      hint: "超过该宽度的图片会等比缩小后再存储，小图不会被放大",
    },
    variant: {
      title: "附加格式",
      hint: "只能选一种 —— 支持 AVIF 的浏览器必然也支持 WebP，两个都存等于为同一个回退付两次空间。",
      webpDesc: "比原图小 25–35%，浏览器全面支持",
      avifDesc: "比 WebP 再小 20–30%，编码慢，后台异步生成",
      noneLabel: "不生成",
      noneDesc: "只保留主图，最省空间",
    },
    thumbnailNote:
      "上传时只生成 200px 网格缩略图，两种模式下都会生成 —— 图库列表用的就是它，直接渲染原图会让页面变得无法使用。600px 与 1200px 改为在图片详情里点击对应尺寸时才生成，不点就不占空间。你复制到外部的链接始终指向主图。",
  },

  passwordField: {
    strength: {
      veryWeak: "很弱",
      weak: "较弱",
      fair: "一般",
      strong: "很强",
    },
    placeholder: "至少 8 位",
    generateTitle: "随机生成一个 20 位强密码",
    generateAria: "随机生成密码",
    hide: "隐藏密码",
    show: "显示密码",
    generatedCopied: "已生成并复制到剪贴板，请立即保存",
  },

  otpConfirm: {
    passkeyAction: "继续添加",
    purgeTitle: "清空图库",
    purgeAction: "确认清空",
    sending: "正在发送验证码…",
    sendFailed: "无法发送验证码。",
    codePlaceholder: "6 位验证码",
    resendCountdown: (seconds: number) => `重新发送 (${seconds}s)`,
  },

  deleteAccount: {
    button: "删除账户",
    done: "账号已删除",
    doneWithImages: (count: number) => `账号已删除，同时删除了 ${count} 张图片`,
    warningTitle: "删除账户是不可撤销的",
    warning: {
      images: "你上传的全部图片会被永久删除，所有外链立即失效",
      imagesEmphasis: "全部图片会被永久删除",
      rewards: "已获得的空间、签到记录、邀请奖励全部清空",
      tokens: "API Token 立即失效，绑定的自有存储配置会被移除（不会删除你自己桶里的文件）",
      reregister: "该邮箱可以重新注册，但数据无法找回",
    },
    confirmLabel: (email: string) => `请输入 ${email} 以确认`,
    confirmButton: "永久删除我的账户",
  },

  checkinWeek: {
    /** 按下标取一个字母，而不是按下标切字符串。 */
    weekdayInitial: (i: number) => "一二三四五六日"[i],
    milestoneReward: (days: number, amount: string) => `满 ${days} 天 +${amount}`,
    milestoneProgress: (current: number, total: number) => `${current} / ${total} 天`,
  },

  checkinCalendar: {
    cappedNoGrant: "已达上限，未发放",
    streak: (days: number) => `连续 ${days} 天`,
    empty: "未签到",
    summary: (days: number, amount: string) => `${days} 天签到 · 累计 +${amount}`,
  },

  activityCalendar: {
    tooltipUploads: (count: number) => `上传 ${count} 张`,
    tooltipBytes: (size: string) => `占用 ${size}`,
    empty: "没有上传",
    summaryDays: (days: number) => `${days} 天有上传`,
    summaryCount: (count: number) => `共 ${count} 张`,
  },

  heatCalendar: {
    noData: "无数据",
    legendLess: "少",
    legendMore: "多",
  },

  toast: {
    dismiss: "点击关闭",
  },
};
