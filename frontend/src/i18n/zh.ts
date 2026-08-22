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
    ok: "确定",
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
    documentTitle: "Openimg — 免费图床 | 图片外链 · WebP/AVIF 压缩 · CDN 加速",
    adminRequired: "需要管理员权限",
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

  /** Mac 客户端的下载板块与下载页。 */
  mac: {
    sectionTitle: "Mac 客户端",
    sectionSubtitle: "拖进来就传，链接直接进剪贴板",
    tagline: "把图床放进菜单栏",
    intro:
      "拖放上传、批量并发、本地压缩与去除定位信息，都在传出去之前完成。支持 Passkey 登录与应用内更新。",
    download: "下载 for macOS",
    downloadDmg: "下载安装盘（.dmg）",
    downloadZip: "下载 .zip",
    dmgHint: "推荐 .dmg：打开后拖进「应用程序」。zip 就地双击会让应用内更新失效。",
    version: "版本",
    updated: "更新于",
    downloads: "累计下载",
    size: "体积",
    requires: (v: string) => `需要 macOS ${v} 或更新版本`,
    times: (n: number) => `${n} 次`,
    viewOnGithub: "在 GitHub 上查看",
    changelog: "更新记录",
    changelogSubtitle: "每个版本改了什么",
    fullChangelog: "完整改动",
    loading: "正在取版本信息…",
    unavailable: "版本信息暂时取不到，可以直接去 GitHub 下载",
    features: [
      { icon: "cloud-arrow-up", title: "拖进来就传", desc: "拖放或粘贴，传完链接自动进剪贴板" },
      { icon: "bolt", title: "批量并发", desc: "最多 20 张同时传，不用一张一张等" },
      { icon: "shield-halved", title: "先去定位再上传", desc: "GPS 与设备信息在本机剥掉，剥不干净就不传" },
      { icon: "fingerprint", title: "Passkey 登录", desc: "指纹解锁，不用记密码" },
    ],
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
      badge: "开源公益项目 · 永久免费",
      titleLead: "稳定快速的",
      titleAccent: "免费图床",
      subtitle:
        "上传自动压缩并转换 WebP / AVIF，全球 CDN 分发，图片外链持久有效。\n支持 API 上传与自有 R2 / S3 存储。",
    },
    features: {
      sectionTitle: "为什么用 Openimg",
      sectionSubtitle: "更小的体积，更快的加载",
      optimize: {
        title: "自动压缩转换",
        desc: "上传即压缩并按设置直接转成 WebP / AVIF 存储，附带网格缩略图。体积通常只剩三分之一，画质肉眼无差。",
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
      title: "开始使用 Openimg",
      subtitle: "公益项目：无付费墙、无广告、不出售数据。",
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
    download: "下载",
    overview: "概览",
    gallery: "图库",
    generate: "AI 生成",
    retouch: "AI 修图",
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
      totalEarned: (amount: string) => `累计签到已获得 ${amount}`,
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
    /** 升级与扩容的提示。都是顺带发生的好事，用完就走，不需要用户确认。 */
    promote: {
      trusted: (space: string) => `已升级为受信任用户，空间 +${space}`,
      loyalty: (space: string) => `长期活跃奖励，空间 +${space}`,
    },
    emailUnverified: "你的邮箱还没有验证，验证后才能上传图片。",
    goVerify: "去验证 →",
    storageDetails: "空间明细 →",
    checkInForStorage: "去签到领空间 →",
    currentLimitsTitle: "当前限制",
    currentLimits: (tier: string) => `当前限制（${tier}）`,
    maxFileSize: "单文件上限",
    uploadedToday: "今日已传",
    uploadedTodayValue: (used: number, limit: number) => `${used} / ${limit} 张`,
    supportedFormats: "支持格式",
    recentHint: "点击查看详情与各格式链接",
    viewGallery: "全部图库 →",
    networkError: "网络错误，上传失败",
  },

  generate: {
    title: "AI 生成",
    subtitle: "写一句描述，生成的图直接进你的图库，和手动上传的完全一样",
    promptLabel: "画面描述",
    promptPlaceholder: "描述你想要的画面：主体、场景、风格、光线、镜头……写得越具体越接近",
    /** 字数计数器紧跟输入框，两个数字都由这条渲染。 */
    promptCounter: (used: number, max: number) => `${used} / ${max}`,
    sizeLabel: "画面比例",
    resolutionLabel: "清晰度",
    submit: "生成图片",
    submitBusy: "提交中…",
    submitted: "已提交",
    submittedDetail: "通常几十秒，完成后会出现在下面的记录里",
    submitFailed: "提交失败",
    costHint: (n: number) => `本次消耗 ${n} 次`,
    /** 放了参考图之后，这一次其实是「照着这几张改」，按钮下面得说一声。 */
    withRefHint: (n: number) => `参考 ${n} 张图 · 消耗 1 次`,
    refsUnresolved: "这条记录的参考图不在本次会话中，请重新选择",
    submitWithRef: "按参考图生成",

    /**
     * 额度。原先是输入区旁边一整张卡（标题、大号数字、两行标签、一根进度条），
     * 现在只剩两块：`inline` 是塞进输入区底排的那一行，`*Exhausted*` 是用完了
     * 才出现的那一条。卡片上那些只为撑起版面的字（标题、量词、「每月重置」）
     * 跟着卡片一起去掉了——没有地方再渲染它们。
     */
    quota: {
      inline: {
        times: (n: number) => `${n} 次`,
        today: (used: number, limit: number) => `今日 ${used}/${limit}`,
        /**
         * 配给和签到分开报。从前这里是 `本月 ${credits}/${monthly}`,而 credits
         * 含签到攒的那部分,于是配 50 却显示 56/50——看着就像数据错了,真有人
         * 因此来问过两次。
         */
        monthly: (left: number, total: number) => `配给 ${left}/${total}`,
        granted: (n: number) => `签到 ${n}`,
        expiring: (n: number, date: string) => `${n} 次将于 ${date} 过期`,
        /** 关联 pic.bi 之后多出来的那一份余额，本站的用完了就接着扣它。 */
        picbi: (n: number) => `pic.bi ${n} 次`,
      },
      /** 两种「用完了」的解法不同：一个等明天，一个靠签到。 */
      dailyExhausted: "今天的次数用完了",
      dailyExhaustedHint: (limit: number) => `每天最多 ${limit} 次，明天恢复`,
      monthlyExhausted: "这个月的次数用完了",
      monthlyExhaustedHint: "每天签到送 1 次，连签 10/20/30 天再送 3/5/10 次",
      checkinLink: "去签到领次数 →",
      /** 签到反馈里的一句：不说的话，用户不知道签到到底有没有把次数领到手。 */
      checkinGranted: (n: number) => `另外领到 ${n} 次 AI 生成`,
    },

    history: {
      title: "生成记录",
      count: (n: number) => `共 ${n} 条`,
      empty: "还没有生成过图片",
      emptyHint: "在上面写一句描述，试试第一张",
      status: {
        charging: "提交中",
        pending: "排队中",
        running: "生成中",
        completed: "已完成",
        failed: "失败",
      },
      working: "正在生成，通常几十秒",
      refunded: "已退还次数",
      copyLink: "复制链接",
      openDetail: "查看详情与各格式链接",
      reusePrompt: "把描述填回输入框",
      failedLabel: "失败原因",
      remove: "删除",
      removeTitle: "删除这条记录？",
      removeBody: "记录会从列表里消失。已经用掉的次数不会退回来。",
      removeKeepImage: "只删记录，图片留在图库",
      removeWithImage: "记录和图片一起删",
      removed: "已删除",
    },
  },

  /**
   * 选图那一行，两个 AI 页共用（components/ai/SourceImages）。修图页管它叫
   * 「源图」、生成页管它叫「参考图」，除了这两个称呼和空态的分量，其余完全
   * 是同一件事，所以文案也只有这一份。
   */
  aiSource: {
    sourceLabel: "源图",
    refLabel: "参考图",
    optionalTag: "可选",
    count: (n: number, max: number) => `${n} / ${max}`,
    fromGallery: "从图库选",
    uploadNew: "上传图片",
    /** 两条提示都要说清「上传的图会进图库」，因为它确实会占用空间。 */
    sourceHint: (max: number) => `最多 ${max} 张 · 从图库选，或把图片拖进来上传`,
    refHint: (max: number) => `不填也能生成 · 最多 ${max} 张，可拖入上传`,
    dropHere: "松开即可上传",
    recentLabel: "最近上传",
    retry: "重试",
    uploadFailedTitle: "上传失败",
    notImage: "只能上传图片文件，其余的已跳过",
    full: (max: number) => `最多 ${max} 张，多出来的没有上传`,

    /** 选图弹层。两页共用，所以标题说「图片」而不是「源图」。 */
    picker: {
      title: "从图库选图",
      subtitle: (max: number) => `最多 ${max} 张 · 按选中的先后顺序交给模型`,
      selected: (n: number, max: number) => `已选 ${n} / ${max}`,
      limitReached: (max: number) => `已选满 ${max} 张，取消一张才能换`,
      confirm: "就用这些",
    },
  },

  /**
   * AI 修图。额度、状态、失败原因这些跟文生图共用一套（`generate.*`），
   * 选图那一行走 `aiSource.*`，这里只剩这条路上独有的文案。
   */
  retouch: {
    title: "AI 修图",
    subtitle: "挑 1–4 张图（图库里选或直接上传），写一句要改什么，改完的图直接进你的图库",

    presetsLabel: "常用改法",
    /**
     * 按钮上的字。给模型的那句提示词不在这里——它是指令不是文案，只有一份英文，
     * 写死在 pages/Retouch.tsx 里，两端逐字相同。
     */
    presets: {
      watermark: "去水印",
      declutter: "去除路人 / 杂物",
      background: "换背景",
      restore: "修复老照片",
      sharpen: "提升清晰度",
      text: "去文字",
      whitebg: "白底商品图",
      colorize: "黑白上色",
      portrait: "人像精修",
      cartoon: "卡通风格",
      expand: "扩展画面",
    },
    /** 点中的那一条再点一次就取消，按钮上挂个说明，省得用户去输入框里手删。 */
    presetClear: "再点一次取消",

    promptLabel: "要改成什么样",
    promptPlaceholder: "说清楚改哪里、改成什么样，以及哪些地方不要动",
    /** 不传比例时上游按原图输出，所以这是个真选项，不是「默认值」。 */
    matchSource: "跟随原图",
    matchSourceHint: "不指定时，输出保持原图的比例与清晰度",

    needSource: "先选一张源图",
    submit: "开始修图",
    submitted: "已提交",
    submittedDetail: "通常几十秒，完成后会出现在下面的记录里",
    submitFailed: "提交失败",

    history: {
      title: "修图记录",
      empty: "还没有修过图",
      emptyHint: "在上面选张图，写一句要改什么",
      kindBadge: "修图",
      sourceLabel: "源图",
      /** 记录比图片活得久，源图可能已经删了；这时至少要说清有几张。 */
      sourceUnavailable: (n: number) => `${n} 张（已不在图库）`,
      sourceMissing: (n: number) => `另有 ${n} 张已删除`,
      reuse: "把描述和源图填回去",
    },
  },

  uploader: {
    dropHere: "松开即可上传",
    dragOrClick: "拖拽图片到这里，或点击选择",
    pasteHint: "支持 Ctrl+V 粘贴截图 · JPEG / PNG / WebP / GIF / AVIF",
    signInToUpload: "登录后即可上传图片",
    signUpPerk: "注册送 1 GB，每天签到还能随机领 1–20 MB",
    signUpFree: "免费注册",
    urlPlaceholder: "或粘贴一条图片网址",
    urlFetch: "取图",
    urlHint: "由服务器代取，只支持 http / https 的公网地址",
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
      emailNotify: "邮件提醒",
      emailNotifyOn: "接收",
      emailNotifyOff: "已关闭",
      emailNotifyHint: "很久没来时提醒你图还在。验证码这类不受影响。",
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
    // 关联不是登录：pic.bi 账号打通的是积分，不能拿来登录本站，所以
    // 它自己一张卡片，措辞也和上面那组「登录方式」分开。
    picbi: {
      title: "账号关联",
      subtitle: "两边账号各自独立，关联只是把额度打通",
      link: "关联",
      unlink: "取消关联",
      connected: "已关联",
      note: "关联 pic.bi 后，AI 生成会多出 4K 清晰度",
      connectedNote: "AI 生成先用本站的免费次数，用完才扣 pic.bi 的积分",
      linkedTip: "已关联 pic.bi",
      unlinkedTip: "已取消关联 pic.bi",
      unlinkConfirm: "取消关联 pic.bi？之后 AI 生成不再能使用 pic.bi 的积分，4K 清晰度也会收起。",
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
    thumb: {
      title: "缩略图策略",
      hint: "图库网格加载的那张小图按这里的设置生成。宽度越小越省空间；AVIF 体积最小，JPEG 兼容性最好。",
      widthTitle: "宽度",
      formatTitle: "格式",
      formatWebp: "均衡，站内显示的默认选择",
      formatAvif: "体积再省约三成，极老的浏览器不支持",
      formatJpg: "任何软件都认识，体积最大",
    },
    thumbnailNote:
      "网格缩略图按上面的策略在上传时生成，两种模式下都会生成——图库列表用的就是它，直接渲染原图会让页面变得无法使用。600px 与 1200px 在图片详情里点击对应尺寸时才生成，不点就不占空间。你复制到外部的链接始终指向主图。",
    savedDetail: "对之后上传的图片生效，已上传的不受影响",
    mode: {
      title: "上传模式",
      optimized: "优化模式",
      optimizedLine1: "重新编码，剥离 EXIF 与 GPS",
      optimizedLine2: "可限制最大宽度",
      optimizedLine3: "体积通常只剩 10–30%",
      original: "原图模式",
      originalLine1: "字节原样保存，保留 EXIF（仅 JPEG 抹除 GPS 定位）",
      originalLine2: "不压缩、不缩放",
      originalLine3: "占用空间显著更大",
    },
    badgeRecommended: "推荐",
    original: {
      warning:
        "原图连同 EXIF 一起公开 —— 任何拿到链接的人都能读出拍摄时间和设备。JPEG 的 GPS 定位会被抹除；PNG / WebP 等其他格式的定位信息会原样公开，分享手机照片前请确认这是你想要的。",
      warningSecurity: "为安全起见，上传仍会校验真实格式（拒绝 SVG 与非图片），并强制以图片类型返回。",
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
      title: "上传自动转换",
      hint: "选定后上传的图片直接转成该格式存储，不保留原格式——外链扩展名就是目标格式。动图保持原格式。",
      disabledHint: "原图模式下不转换",
      webpDesc: "比原格式小 25–35%，浏览器全面支持",
      avifDesc: "比 WebP 再小 20–30%，编码更慢，上传耗时会增加",
      noneLabel: "不转换",
      noneDesc: "保持上传时的格式存储",
    },
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
    typeEmailToConfirm: (email: string) => `请输入 ${email} 以确认`,
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
  groupBadge: {
    admin: "管理员",
    trusted: "受信任",
    free: "免费用户",
  },
  docs: {
    title: "接入 Openimg",
    subtitle: "用一枚 API Token，把图床接进 PicGo、Typora、脚本或 CI。",
    toc: {
      token: "先拿一枚 Token",
      curl: "一条命令上传",
      picgo: "PicGo",
      typora: "Typora",
      errors: "出错了怎么办",
      limits: "限制",
      more: "其余接口",
    },
    token: {
      heading: "先拿一枚 Token",
      settingsLink: "设置 → API Token",
      createIn: (link: string, once: string) =>
        `到 ${link} 创建。明文${once}，关掉就再也看不到——服务端只存哈希，找回不了，丢了只能重建。`,
      onceEmphasis: "只显示这一次",
      headerForms: (prefix: string) =>
        `Token 以 ${prefix} 开头。三种请求头写法都收，服务端按这个顺序取第一个非空的：`,
      commentCommon: "最常见",
      commentBare: "裸写，必须以 oimg_ 开头",
      commentNoBearer: "给不会加 Bearer 前缀的客户端",
      note: (bearer: string, warn: string, apiKey: string) =>
        `第三种是为 PicGo、Typora 这类工具留的——它们能设自定义请求头，但不一定会处理 ${bearer} 前缀。不过${warn}：服务端 CORS 放行的请求头只有 Origin、Content-Type、Accept、Authorization，${apiKey} 会被预检挡掉。`,
      noteWarn: "浏览器里的脚本别用它",
    },
    curl: {
      heading: "一条命令上传",
      constraints: (multipart: string, field: string) =>
        `两条硬约束：必须是 ${multipart}，字段名必须是 ${field}。服务端只读这一个字段，质量、格式、宽度这些一律不看——转码行为由你账号的设置决定，不由请求决定。`,
      success: (code: string) => `成功返回 ${code}，结构是这样（只列常用字段）：`,
      directLinkOnly: "只要直链的话：",
      note: (imageKey: string, dedupKey: string, path: string, wrong: string) =>
        `顶层只有 ${imageKey} 和 ${dedupKey} 两个键。填 JSON 路径的地方要写 ${path}，不是 ${wrong}——这是接入时最常踩的一脚。`,
      noteEmphasis: (imageKey: string) => `图片对象在 ${imageKey} 下面`,
      dedup: (flag: string, emphasis: string) =>
        `${flag} 表示服务端已经存过一模一样的字节，走秒传：不重新转码、不重新写对象，但${emphasis}。`,
      dedupEmphasis: "配额照样全额扣",
      variants: (variants: string, isString: string, urls: string) =>
        `另外 ${variants} 是逗号分隔的${isString}，${urls} 才是对象——两个字段同时存在，容易看混。`,
      variantsEmphasis: "字符串",
    },
    picgo: {
      heading: "PicGo",
      install: (plugin: string) => `装插件 ${plugin}，然后逐项填：`,
      fieldApiUrl: "API 地址",
      fieldPostName: "POST 参数名",
      fieldJsonPath: "JSON 路径",
      fieldHeaders: "自定义请求头",
      fieldBody: "自定义 Body",
      valueEmpty: "留空",
      shortLink: (path: string) => `想让粘贴出来的是短链，把 JSON 路径改成 ${path} 就行。`,
      note: "PicGo 相册里的「删除」只删本地记录，不会真的删服务器上的图——自定义 Web 图床没有删除回调。要真删见下面第 7 节。",
    },
    typora: {
      heading: "Typora",
      intro: (emphasis: string) =>
        `${emphasis}它的上传服务只有 PicGo、iPic、uPic 和「自定义命令」几个选项，没有可以填 API 地址和请求头的表单。两条路：`,
      introEmphasis: "Typora 不能直连。",
      viaPicgo: "走 PicGo（推荐）",
      viaPicgoSteps: (picgoApp: string) =>
        `按上一节配好 PicGo，然后 Typora → 偏好设置 → 图像 → 上传服务选 ${picgoApp}，指定可执行文件路径，点「验证图片上传选项」。`,
      viaScript: "不想装 PicGo：自定义命令",
      viaScriptIntro: (path: string, chmod: string) =>
        `Typora 会把待上传的文件路径作为参数传给命令，并读取输出的最后几行当 URL。存成 ${path} 并 ${chmod}：`,
      viaScriptTail: (custom: string, curl: string, jq: string) =>
        `然后上传服务选 ${custom}，命令填这个脚本的路径。需要本机有 ${curl} 和 ${jq}。`,
    },
    errors: {
      heading: "出错了怎么办",
      shape: (json: string) => `错误响应统一是 ${json}，个别会多带字段。`,
      colStatus: "状态码",
      colFix: "怎么办",
      note: (emphasis: string, retryAfter: string, used: string, limit: string) =>
        `${emphasis}只看状态码会误判：带 ${retryAfter} 的是频率限制（每 IP 每分钟 60 次），退避之后可以继续；带 ${used} / ${limit} 的是当天配额用完了，退避没有意义，要等第二天。`,
      noteEmphasis: "两种 429 要分开处理。",
    },
    limits: {
      heading: "限制",
      intro: (emphasis: string) =>
        `单文件大小、每日次数、允许格式、像素尺寸都按${emphasis}走，管理员可能调整过。别把数字写死在客户端里，现取：`,
      introEmphasis: "用户组",
      fields: (a: string, b: string, c: string, d: string) =>
        `返回里有 ${a}、${b}、${c}，以及 ${d} 剩余空间。`,
      note: (emphasis: string) =>
        `大小上限卡的是${emphasis}，不只是文件本身——multipart 的边界和头部也算进去。所以一个字节数刚好等于上限的文件仍然会被拒，留 1% 余量比较稳。`,
      noteEmphasis: "整个请求体",
    },
    more: {
      heading: "其余接口",
      intro: "这些接口同样用 Token 调：",
      commentList: "列表（分页、搜索、排序）",
      commentDetail: "单张详情",
      commentDelete: "删除一张",
      commentBulk: "批量删除",
      commentCheckin: "每日签到领空间",
      note: (emphasis: string) =>
        `改密码、管理 Token、绑定自有存储这些${emphasis}，只能在网页上操作。一枚泄漏的 Token 不该能接管账号——它能传图删图，但改不了密码、发不了新 Token、读不到你的 S3 密钥。`,
      noteEmphasis: "不能用 Token 调",
    },
    errorRows: [
      /* The `error` column is the API's own wording, taken from the backend
         catalogue, so this table and the responses cannot drift apart. */
      ["401", "auth: invalid token format", "Token 不以 oimg_ 开头，多半是粘贴时丢了字符"],
      ["401", "auth: invalid token", "查不到这枚 Token —— 已被删除，或者抄错了"],
      ["401", "auth: token expired", "过期了，重建一枚（创建时不填有效期即永不过期）"],
      ["401", "尚未登录", "一个认证头都没带"],
      ["403", "请先验证邮箱后再上传", "带 code: email_unverified，去网页验证邮箱"],
      ["413", "文件超过大小上限 X MB", "压缩后重传"],
      ["400", "缺少上传文件字段 file", "字段名不是 file，或请求不是合法 multipart"],
      ["415", "不支持的图片格式：X", "格式不在白名单。SVG、PDF 明确拒绝"],
      ["415", "当前用户组不允许上传 X 格式", "响应里的 allowed 数组是你能传的格式"],
      ["415", "图片尺寸 AxB 超出上限 CxD", "像素超限。注意是 415 不是 413"],
      ["429", "上传过于频繁，请稍后再试", "带 retry_after 秒数，退避后重试"],
      ["429", "今日上传数量已达上限", "带 used / limit，等第二天"],
      ["507", "空间不足：需要 X，剩余 Y", "删图或签到领空间"],
      ["503", "存储不可用：…", "后端存储出问题，稍后重试"],
    ] as [string, string, string][],
    feedback: (link: string) => `文档和代码对不上，或者哪里没写清楚？${link}。`,
    feedbackLink: "提个 issue",
  },
};
