import type { Dict } from "./index";

/**
 * English catalogue. Typed as `Dict`, so anything missing — or a function whose
 * arguments drifted from the Chinese one — is a build error, not a page that
 * renders `undefined`.
 *
 * `p` is the local plural helper. `index.ts` exports the same function for call
 * sites, but importing it here would close an import cycle (index → en → index)
 * for the sake of three lines, so it is duplicated deliberately.
 */
const p = (n: number, one: string, many: string): string => (n === 1 ? one : many);

export const en: Dict = {
  common: {
    loading: "Loading…",
    noData: "No data yet",

    save: "Save",
    saved: "Saved",
    saveFailed: "Couldn't save",
    cancel: "Cancel",
    cancelled: "Cancelled",
    close: "Close",
    confirmChange: "Confirm change",
    delete: "Delete",
    deleteFailed: "Delete failed",
    remove: "Remove",
    edit: "Edit",
    copy: "Copy",
    copied: "Copied",
    download: "Download",
    optional: "(optional)",
    verified: "Verified",
    documentTitle: "Openimg — A free image host, forever | Optimize, convert, deliver",
    adminRequired: "Admin access required",
    backToHome: "Back to home",

    signIn: "Sign in",
    signUp: "Sign up",
    signOut: "Sign out",
    authRequired: (link: string) => `Please ${link} first`,
    authRequiredLink: "sign in",

    email: "Email",
    displayName: "Display name",
    emailPassword: "Email + password",
    newPassword: "New password",
    enterAgain: "Enter it again",
    passwordMismatch: "The two passwords do not match",
    setPassword: "Set password",
    changePassword: "Change password",
    addPasskey: "Add passkey",

    emailCode: "Email code",
    otpPlaceholder: "6 digits",
    sendCode: "Send code",
    sendingCode: "Sending…",
    resend: "Resend",
    otpSentTo: (email: string) => `Code sent to ${email}, valid for 5 minutes.`,

    upload: "Upload",
    recentUploads: "Recent uploads",
    noUploadsYet: "No uploads yet",
    uploadFirst: "Upload your first image →",
    availableStorage: "Available storage",
    shortLink: "Short link",

    imageCount: (n: number) => `${n} ${p(n, "image", "images")}`,
    countAndSize: (n: number, size: string) => `${n} ${p(n, "image", "images")} · ${size}`,
    days: (n: number) => `${n} ${p(n, "day", "days")}`,
    lastUsed: (date: string) => `last used ${date}`,

    pager: {
      prev: "Previous page",
      next: "Next page",
    },
  },

  home: {
    highlight: {
      optimizeOrOriginal: "Optimized or keep original",
      webpAvif: "WebP / AVIF",
      globalCdn: "Global CDN",
      stripExif: "EXIF stripped automatically",
      ownBucket: "Bring your own R2 / S3 bucket",
      freeForever: "Free forever",
    },
    hero: {
      badge: "A nonprofit image host, free forever",
      titleLead: "Image hosting,",
      titleAccent: "handled for you",
      subtitle: "Just drag them in. We handle the optimizing, converting and delivery.",
    },
    features: {
      sectionTitle: "Why Openimg",
      sectionSubtitle: "One job, done well: small images, delivered fast",
      optimize: {
        title: "Automatic optimizing and conversion",
        desc: "Every upload gets WebP / AVIF derivative formats and thumbnails at several sizes. Usually about a third of the original size, with no visible quality loss.",
      },
      cdn: {
        title: "Cloudflare CDN",
        desc: "Content-addressed immutable URLs, cached at the edge for a year. Every request is served from the nearest node.",
      },
      privacy: {
        title: "Privacy first",
        desc: "EXIF and GPS location data are stripped automatically, and re-encoding removes anything else embedded in the file. Where you took the photo stays private.",
      },
      ownStorage: {
        title: "Connect your own storage",
        desc: "Store images straight into your own bucket on Cloudflare R2 or S3. No storage limit from us, and you can move away at any time.",
      },
      checkin: {
        title: "Earn storage by checking in",
        desc: "1 GB when you sign up, a random 1–20 MB for each daily check-in, a bonus for a 7-day check-in streak, and 200 MB each for you and anyone you refer. Storage adds up and never expires.",
      },
      tools: {
        title: "Works with your tools",
        desc: "Create an API token and use it straight from PicGo, Typora or curl. No switching pages while you write.",
      },
    },
    steps: {
      sectionTitle: "Three steps to start",
      sectionSubtitle: "Under a minute",
      signUp: {
        title: "Create an account",
        desc: "Email, Google, GitHub or passkey. Verify your email and you can upload.",
      },
      drop: {
        title: "Drag in your images",
        desc: "Drag, click to browse, or paste a screenshot with Ctrl+V. Batches work too.",
      },
      copy: {
        title: "Copy the link",
        desc: "Copy the URL, Markdown, HTML or BBCode with one click and paste it anywhere.",
      },
    },
    integration: {
      sectionTitle: "Fit it into your workflow",
      sectionSubtitle: "Create an API token in settings to get started",
      curlCardTitle: "curl",
      picgoCardTitle: "PicGo custom image host",
      picgoConfigLabel: {
        apiUrl: "API URL",
        postField: "POST field name",
        customHeader: "Custom header",
        jsonPath: "JSON path",
      },
    },
    cta: {
      title: "Start now, it costs nothing",
      subtitle: "This is a nonprofit project. No paid tiers, no ads, and we never sell your data.",
      button: "Sign up free and get 1 GB",
    },
  },

  auth: {
    shell: {
      signIn: "Sign in to",
      signUp: "Sign up for",
      reset: "Reset password for",
    },
    modePasskey: "Passkey",
    password: "Password",
    forgotPassword: "Forgot password?",
    signingIn: "Signing in…",
    noAccountPrompt: "No account?",
    haveAccountPrompt: "Already have an account?",
    resetCodeSentNeutral:
      "If that email is registered, a code has been sent. We answer the same either way, so this page cannot be used to find out whether an account exists.",
    nativeHandoffFailed: "Could not hand the sign-in back to the app. Try again.",

    register: {
      emailRequiredFirst: "Enter your email first",
      displayNamePlaceholder: "The name others see, does not need to be unique",
      password: "Password (at least 8 characters)",
      confirmPassword: "Confirm password",
      submitBusy: "Creating account…",
      submit: "Create account",
    },

    reset: {
      emailRequired: "Enter your email",
      codeSentNotice:
        "If that email is registered, a code has been sent. We treat both cases the same so no one can tell whether an account exists.",
      submitBusy: "Saving…",
      submit: "Set new password and sign in",
      backToSignIn: "← Back to sign in",
    },

    passkey: {
      cancelledOrFailed: "Cancelled or failed",
      email: "Email (optional — leave blank to let the browser pick a passkey)",
      emailPlaceholder: "Leave blank to sign in with a passkey on this device",
      submitBusy: "Waiting for passkey…",
      submit: "Sign in with a passkey",
      hint: "Uses Touch ID, Face ID or a security key on this device",
    },

    oauth: {
      divider: "Or use",
      google: "Sign in with Google",
      github: "Sign in with GitHub",
      passkey: "Sign in with a passkey or fingerprint",
    },
  },

  nav: {
    overview: "Overview",
    gallery: "Gallery",
    refer: "Referrals",
    admin: "Admin",
    checkin: "Check in",
    checkinHint: "Check in for a random amount of storage",
    checkedInTodayStreak: (days: number) =>
      `Checked in today · ${days} ${p(days, "day", "days")} streak`,
    storagePillTitle: (used: string, total: string) => `${used} used of ${total}`,
    switchToViolet: "Switch to violet",
    switchToGreen: "Switch to green",
    checkinCappedTitle: "Storage limit reached",
    checkinCappedDetail: "Today's check-in added no storage",
    checkinSuccessTitle: (amount: string) => `Checked in, +${amount}`,
    checkinSuccessDetail: "Added to your total storage permanently",
    checkinFailed: "Check-in failed",
  },

  footer: {
    docs: "API docs",
    statImages: (n: number) => p(n, "image", "images"),
    statUsers: (n: number) => p(n, "user", "users"),
    statStored: "stored",
    statToday: "uploaded today",
    rightsReserved: "All rights reserved",
  },

  dashboard: {
    greeting: (_name: string) => `Hi, ${name}`,
    welcomeBack: "Welcome back",
    streakBadge: (days: number) => `${days} ${p(days, "day", "days")} check-in streak`,
    uploadCta: "Upload images",
    kpi: {
      availableStorageSub: (used: string, percent: number, total: string) =>
        `${used} used · ${percent}% · ${total} total`,
      totalImages: "Total images",
      uploadsTodaySub: (count: number, limit: number) => `${count} / ${limit} uploaded today`,
      checkinStreak: "Check-in streak",
      checkedInToday: "Checked in today",
      notCheckedInToday: "Not checked in yet today",
      savedByOptimizing: "Saved by optimizing",
      origToStored: (orig: string, stored: string) => `${orig} original → ${stored} stored`,
      acrossAllImages: "Across all images",
    },
    earnTrend: {
      title: "Storage earned",
      subtitle: "Last 30 days · check-in / referral / grants",
    },
    optimization: {
      title: "Optimization results",
      subtitle: (count: number) => `${count} ${p(count, "image", "images")} total`,
      calculating: "Calculating…",
      actuallyStored: "Actually stored",
      saved: "Saved",
      breakdown: (primary: string, variants: string, thumbs: string) =>
        `Storage breakdown · ${primary} primary · ${variants} full-size derivatives · ${thumbs} thumbnails`,
      unclassified: (size: string) => `${size} unclassified (legacy)`,
    },
    storageSplit: {
      title: "Storage distribution",
      subtitle: "Platform storage and your own buckets",
      byFormat: "By format",
    },
    activity: {
      title: "Upload activity",
      subtitle: "Last 30 days · hover for that day's uploads and storage",
    },
    recent: {
      subtitle: "Click to manage in the gallery",
      viewAll: "View all →",
    },
  },

  space: {
    title: "My storage",
    tx: {
      signupGrant: "Signup grant",
      checkin: "Daily check-in",
      referral: "Referral reward",
      adminGrant: "Admin adjustment",
      upload: "Upload",
      deleteRefund: "Deletion refund",
    },
    quota: {
      title: "Storage used",
      totalAndPercent: (total: string, percent: number) => `${total} total · ${percent}% used`,
      total: "Total quota",
      available: "Available",
      images: "Images",
    },
    checkin: {
      cappedMsg: "Checked in, but your storage is at its cap — nothing granted this time",
      successMsg: (granted: string, days: number) =>
        `Checked in, +${granted} · ${days} ${p(days, "day", "days")} streak`,
      streakUnit: (n: number) => p(n, "day streak", "days streak"),
      randomRange: (min: string, max: string) => `${min} – ${max} at random each day`,
      neverExpires: "Adds up permanently, never expires",
      doneToday: "Checked in",
      button: (min: string, max: string) => `Check in for ${min}–${max}`,
      monthProgress: "This month's progress",
    },
    ledger: {
      title: "Storage ledger",
      total: (count: number) => `${count} ${p(count, "entry", "entries")}`,
      perPage: "Per page",
      perPageSuffix: "",
      empty: "No records yet",
    },
  },

  refer: {
    copyFailed: (error: string) => `Copy failed: ${error}`,
    title: "Refer and earn",
    heroHeadline: (bonus: string) => `Invite a friend, you both get ${bonus}`,
    heroSub:
      "Share your link with a friend. When they sign up, the storage lands in both accounts right away.",
    yourCode: "Your referral code",
    theyGet: "They get",
    theyGetBonus: (bonus: string) =>
      `${bonus} one-time — credited as soon as they sign up through your link`,
    theyGetTier: "Full free tier quota unlocked immediately",
    youGet: "You get",
    youGetBonus: (bonus: string) => `${bonus} one-time — for every friend who signs up`,
    youGetMore: "More referrals, more storage (up to your tier cap)",
    statsHeading: "Stats",
    stat: {
      invited: "Friends referred",
      totalEarned: "Storage earned",
      perReferral: "Reward per referral",
    },
    historyHeading: "Referral history",
    table: {
      user: "User",
      signedUp: "Signed up",
      empty: "No one has signed up through your link yet — copy it and share it with a friend",
    },
  },

  upload: {
    emailUnverified: "Verify your email before you can upload images.",
    goVerify: "Verify now →",
    storageDetails: "Storage details →",
    checkInForStorage: "Check in for storage →",
    currentLimits: (tier: string) => `Current limits (${tier})`,
    maxFileSize: "Max file size",
    uploadedToday: "Uploaded today",
    uploadedTodayValue: (used: number, limit: number) =>
      `${used} / ${limit} ${p(limit, "image", "images")}`,
    supportedFormats: "Supported formats",
    recentHint: "Click for details and links in every format",
    viewGallery: "Full gallery →",
    networkError: "Network error, upload failed",
  },

  uploader: {
    dropHere: "Drop to upload",
    dragOrClick: "Drag images here, or click to choose",
    pasteHint: "Paste screenshots with Ctrl+V · JPEG / PNG / WebP / GIF / AVIF",
    signInToUpload: "Sign in to upload images",
    signUpPerk: "Get 1 GB when you sign up, plus a random 1–20 MB for each daily check-in",
    signUpFree: "Sign up free",
  },

  uploadPanel: {
    finishedWithErrors: (done: number, failed: number) => `${done} uploaded, ${failed} failed`,
    finished: (n: number) => `${n} ${p(n, "image", "images")} uploaded`,
    uploading: (done: number, total: number) => `Uploading ${done}/${total}`,
    copyAll: (n: number) => `Copy ${n} ${p(n, "link", "links")}`,
    clearFinished: "Clear finished",
    clearQueueTitle: "Clear the queue",
    clear: "Clear",
    dedupTitle: "An identical file is already on the server, so it uploaded instantly",
    dedupBadge: "Deduplicated",
    saved: (saved: string) => `${saved} saved`,
    copyLink: "Copy link",
    retry: "Retry",
    remove: "Remove",
  },

  gallery: {
    title: "My gallery",
    searchPlaceholder: "Search file names",
    clearSelection: "Clear selection",
    selectAllOnPage: "Select all on this page",
    selectedCount: (n: number) => `${n} selected`,
    loadedCount: (loaded: number, total: number) => `${loaded} of ${total} loaded`,
    deleteSelected: (n: number) => `Delete ${n} selected`,
    deleteSelectedConfirm: (n: number) =>
      `Delete the ${n} selected ${p(n, "image", "images")}? This frees the storage ${p(n, "it uses", "they use")} and cannot be undone.`,
    deletedToast: (n: number) => `Deleted ${n} ${p(n, "image", "images")}`,
    deletedToastDetail: "The storage has been freed",
    wipeSearchTitle: "Delete every image matching the current search",
    wipeAllTitle: "Delete every image in the gallery",
    wipeSearch: "Delete all search results",
    wipeAll: "Clear gallery",
    wipeProgress: (n: number) => `${n} deleted…`,
    wipeConfirmSearch: (n: number, query: string) =>
      `This will delete the ${n} ${p(n, "image", "images")} matching “${query}” and free the storage ${p(n, "it uses", "they use")}. This cannot be undone.`,
    wipeConfirmAll: (n: number) =>
      `This will delete all ${n} ${p(n, "image", "images")} and free the storage ${p(n, "it uses", "they use")}. This cannot be undone.`,
    wipeDoneToast: (n: number) => `Gallery cleared, ${n} ${p(n, "image", "images")} deleted`,
    wipeDoneToastDetail: "All storage has been freed",
    wipeFailed: "Clearing did not finish",
    wipeFailedDetail: (n: number) => `Stopped after deleting ${n} ${p(n, "image", "images")}`,
    emptySearch: "No images match",
    loadMore: "Load more",
    pageSizeTitle: "Images per page",
    pageSizeUnit: "per page",
    select: "Select",
    deselect: "Deselect",
    blocked: "Blocked",
    sort: {
      newest: "Newest first",
      oldest: "Oldest first",
      largest: "Largest",
      smallest: "Smallest",
      widest: "Highest resolution",
      name: "File name",
    },
  },

  imageDetail: {
    deleteConfirm: "Delete this image? This frees the storage it uses and cannot be undone.",
    dimensions: "Dimensions",
    originalSize: "Original size",
    storedSize: "Storage used",
    format: "Format",
    uploadedAt: "Uploaded",
    generated: "Generated",
    originalVariant: "Original",
    otherSizes: "Other sizes",
    otherSizesHint: "· generated on demand, and only counts against storage once generated",
    generateSize: (width: number) => `Generate ${width}px`,
    originalTitle: "The file you uploaded, uncompressed and with EXIF intact",
    originalLink: (ext: string) => `Original ${ext}`,
  },

  share: {
    goHome: "Go to the Openimg home page →",
    formatDirectLink: "Direct link",
    formatThumbnail: "Thumbnail",
    uploadedBy: (name: string) => `Uploaded by ${name}`,
    views: (count: number) => `${count} ${p(count, "view", "views")}`,
    copyrightNotice:
      "This site only stores and serves images; it holds no copyright over the content shown. All images remain the copyright of their original authors or uploaders. If you believe something here infringes your rights, use the report button in the top right to file a report — we will verify and remove it as soon as we are notified.",
    shortLinkCopied: "Short link copied",
    copyShortLink: "Copy short link",
    targetWeibo: "Weibo",
    targetEmail: "Email",
    reaction: {
      like: "Like",
      clap: "Clap",
      party: "Celebrate",
      sparkle: "Wow",
      smile: "Smile",
      undo: (label: string) => `Undo ${label.toLowerCase()}`,
      failed: "Action failed",
    },
  },

  report: {
    imageTitle: "Report this image",
    dialogTitle: "Report image",
    categoryLabel: "Report category",
    detailsLabel: "Additional details",
    detailsPlaceholder: "Describe the problem…",
    detailsOptionalPlaceholder: "More details (optional)",
    reasonPlaceholder: "Describe the problem, at least 4 characters",
    contactPlaceholder: "Contact (optional, so we can reply with the outcome)",
    notice: "Moderators are notified and will review it. Abusing reports may get your account restricted.",
    noLogin: "No account needed",
    submit: "Submit report",
    submitted: "Report submitted",
    submittedDetail: "An admin will be notified and will review it shortly",
    category: {
      porn: "Sexual content",
      copyright: "Copyright infringement",
      violence: "Violence or terrorism",
      privacy: "Personal privacy",
      fraud: "Scam or malicious link",
      other: "Other violation",
    },
  },

  settings: {
    title: "Account settings",
    profile: {
      title: "Profile",
      subtitle:
        "Your avatar and display name appear at the top of the page. Display names don't have to be unique.",
      avatarUpdated: "Avatar updated",
      avatarUploadFailed: "Avatar upload failed",
      avatarRemoved: "Avatar removed",
      removeFailed: "Couldn't remove",
      avatarButtonTitle: "Click to change your avatar",
      avatarChangeOverlay: "Change",
      avatarHint:
        "Click the avatar to change it · JPG / PNG / WebP / HEIC, cropped to a 256px square and converted to AVIF",
      displayNameChanged: (name: string) => `Display name changed to "${name}"`,
      displayNameCleared: "Display name cleared",
      displayNameToastDetail: "Your email is still your account identifier",
      displayNamePlaceholder: "Not set, can be left blank",
      displayNameHint: "Display names can repeat; your email is the account identifier",
    },
    storage: {
      title: "Storage location",
      subtitle:
        "Images go to platform storage by default. You can also connect your own R2 / S3 bucket, with no platform quota.",
    },
    convert: {
      title: "Optimization and conversion",
      subtitle: "Applies to future uploads only; images already uploaded are unaffected",
    },
    apiTokens: {
      subtitle: "For uploading from external tools like PicGo, Typora and curl",
      docsLink: "How to set it up",
    },
    accountInfo: {
      title: "Account info",
      role: "Role",
      tier: "Tier",
    },
    loginMethods: {
      title: "Sign-in methods",
      subtitle: "You can connect several sign-in methods; any one of them signs you in",
      passwordSet: "Set",
      passwordNotSet: "Not set (you sign in another way)",
      changePasswordAction: "Change password",
      emailOtpNote: "Once verified, you can sign in with an emailed code",
      verifyEmail: "Verify email",
      verifyEmailComingSoon: "Email verification is coming soon",
      connected: "Connected",
    },
    oauth: {
      link: "Link",
      unlink: "Unlink",
      linked: (provider: string) => `${provider} linked`,
      unlinked: (provider: string) => `${provider} unlinked`,
      unlinkConfirm: (provider: string) =>
        `Unlink ${provider}? This ${provider} account will no longer be able to sign in.`,
    },
    password: {
      updated: "Password updated",
      updatedDetail: "Use the new password next time you sign in",
      dialogIntro: "Changing it requires an emailed code.",
      tooShort: "Password must be at least 8 characters",
    },
    passkey: {
      subtitle: "Sign in without a password using Touch ID / Face ID / a security key",
      namePrompt: "Name this passkey (e.g. MacBook Touch ID, iPhone)",
      added: "Passkey added",
      addedDetail: "You can now sign in with your fingerprint or face",
      deleteConfirm: (name: string) =>
        `Delete passkey "${name}"? This device will no longer be able to sign in with a passkey.`,
      otpDetail: "After verification your browser will ask for your fingerprint or face.",
      addedOn: (date: string) => `Added ${date}`,
      empty: "No passkeys yet. Add your first one below.",
      waiting: "Waiting for authorization…",
      defaultDeviceName: "My device",
    },
    danger: {
      title: "Danger zone",
      subtitle: "Nothing here can be undone",
    },
  },

  storageProfiles: {
    endpoint: {
      r2: "Detected Cloudflare R2 · virtual-hosted addressing",
      s3: "Detected AWS S3 · virtual-hosted addressing",
      b2: "Detected Backblaze B2 · virtual-hosted addressing",
      spaces: "Detected DigitalOcean Spaces · virtual-hosted addressing",
      oss: "Detected Alibaba Cloud OSS · virtual-hosted addressing",
      cos: "Detected Tencent Cloud COS · virtual-hosted addressing",
      custom: "Treated as self-hosted S3 (MinIO and similar) · path-style addressing",
    },
    testPassed: "Connection test passed, you can save now",
    badge: {
      default: "Default",
      platform: "Platform",
      backup: "Backup",
      unavailable: "Unavailable",
    },
    platformHosted: "Platform hosted",
    setDefault: "Set as default",
    setDefaultDone: "Set as the default upload target",
    connectionOk: "Connection is fine",
    test: "Test",
    testConnection: "Test connection",
    deleteConfirm: (name: string) => `Delete storage location "${name}"?`,
    deleted: "Deleted",
    addTitle: "Add your own bucket",
    editTitle: (name: string) => `Edit ${name}`,
    field: {
      name: "Name",
      namePlaceholder: "My storage",
      endpointPlaceholder:
        "https://<account>.r2.cloudflarestorage.com or https://s3.example.com",
      keyPrefix: "Key prefix (optional)",
      unchangedPlaceholder: "Leave blank to keep the current value",
      publicBaseUrl: "Public base URL",
    },
    securityNote:
      "Create a token scoped to this bucket only, with object read/write permission only. Secrets are stored encrypted with AES-256-GCM and are never shown on this page again.",
    securityNoteEmphasis: "scoped to this bucket only, with object read/write permission only",
  },

  apiTokens: {
    deleteConfirm: (name: string) =>
      `Delete token "${name}"? Any tool using it stops working immediately.`,
    shownOnce: "This token is shown only once — save it now",
    gotIt: "Got it",
    revoked: "Revoked",
    neverUsed: "never used",
    expiresAt: (date: string) => `expires ${date}`,
    namePlaceholder: "Token name, e.g. PicGo",
    expiry: {
      never: "Never expires",
      days30: "30 days",
      days90: "90 days",
      year1: "1 year",
    },
    generate: "Generate",
    scopeNote:
      "Tokens can only upload and manage images. They can't change account settings, storage credentials or passkeys.",
  },

  convertSettings: {
    thumbnailNote:
      "Only the 200px grid thumbnail is generated at upload time, in both modes — the gallery grid uses it, and rendering originals there would make the page unusable. The 600px and 1200px sizes are generated when you ask for them on an image's page, and cost nothing until you do. Links you copy out always point at the main image.",
    savedDetail: "Applies to future uploads; images already uploaded are unaffected",
    mode: {
      title: "Upload mode",
      optimized: "Optimized",
      optimizedLine1: "Re-encoded, EXIF and GPS stripped",
      optimizedLine2: "Max width can be capped",
      optimizedLine3: "Usually 10–30% of the original size",
      original: "Keep original",
      originalLine1: "Bytes untouched, all EXIF and location data kept",
      originalLine2: "No optimizing, no resizing",
      originalLine3: "Uses noticeably more storage",
    },
    badgeRecommended: "Recommended",
    original: {
      warning:
        "Originals are published with their EXIF and GPS location — anyone with the link can read where and when the photo was taken and on what device. Make sure that's what you want before sharing photos from your phone.",
      warningSecurity:
        "Uploads are still checked against the real file format (SVG and non-images are refused) and always served with an image content type.",
      warningEmphasis: "EXIF and GPS location",
      formatNote:
        "For safety, uploads are still checked for their real format (SVG and non-images are rejected) and always served with an image content type.",
    },
    width: {
      title: "Max width",
      keepOriginal: "Keep original size",
      disabledHint: "No resizing when keeping originals",
      hint: "Wider images are scaled down proportionally before storing; smaller ones are never enlarged",
    },
    variant: {
      title: "Derivative format",
      hint: "Pick one — every browser that supports AVIF also supports WebP, so storing both pays for the same fallback twice.",
      webpDesc: "25–35% smaller than the original, supported by every browser",
      avifDesc: "Another 20–30% smaller than WebP, slow to encode, generated in the background",
      noneLabel: "None",
      noneDesc: "Keep the main image only, uses the least storage",
    },
  },

  passwordField: {
    strength: {
      veryWeak: "Very weak",
      weak: "Weak",
      fair: "Fair",
      strong: "Strong",
    },
    placeholder: "At least 8 characters",
    generateTitle: "Generate a strong random 20-character password",
    generateAria: "Generate a random password",
    hide: "Hide password",
    show: "Show password",
    generatedCopied: "Generated and copied to your clipboard — save it now",
  },

  otpConfirm: {
    passkeyAction: "Continue",
    purgeTitle: "Clear gallery",
    purgeAction: "Confirm clear",
    sending: "Sending code…",
    sendFailed: "Couldn't send the code.",
    codePlaceholder: "6-digit code",
    resendCountdown: (seconds: number) => `Resend (${seconds}s)`,
  },

  deleteAccount: {
    button: "Delete account",
    done: "Account deleted",
    doneWithImages: (count: number) =>
      `Account deleted, along with ${count} ${p(count, "image", "images")}`,
    warningTitle: "Deleting your account can't be undone",
    warning: {
      images:
        "Every image you uploaded is permanently deleted and all external links break immediately",
      imagesEmphasis: "every image is permanently deleted",
      rewards: "Storage you've earned, your check-in record and referral rewards are all wiped",
      tokens:
        "API tokens stop working immediately and connected storage locations are removed (files in your own bucket are not deleted)",
      reregister: "The email can be used to sign up again, but the data can't be recovered",
    },
    confirmLabel: (email: string) => `Type ${email} to confirm`,
    typeEmailToConfirm: (email: string) => `Type ${email} to confirm`,
    confirmButton: "Permanently delete my account",
  },

  checkinWeek: {
    weekdayInitial: (i: number) => ["M", "T", "W", "T", "F", "S", "S"][i],
    milestoneReward: (days: number, amount: string) => `+${amount} at ${days} days`,
    milestoneProgress: (current: number, total: number) => `${current} / ${total} days`,
  },

  checkinCalendar: {
    cappedNoGrant: "Quota capped, nothing granted",
    streak: (days: number) => `${days} ${p(days, "day", "days")} streak`,
    empty: "No check-in",
    summary: (days: number, amount: string) =>
      `${days} ${p(days, "day", "days")} checked in · +${amount} total`,
  },

  activityCalendar: {
    tooltipUploads: (count: number) => `${count} ${p(count, "upload", "uploads")}`,
    tooltipBytes: (size: string) => `${size} used`,
    empty: "No uploads",
    summaryDays: (days: number) => `${days} ${p(days, "day", "days")} with uploads`,
    summaryCount: (count: number) => `${count} ${p(count, "image", "images")} total`,
  },

  heatCalendar: {
    noData: "No data",
    legendLess: "Less",
    legendMore: "More",
  },

  toast: {
    dismiss: "Click to dismiss",
  },
  docs: {
    title: "Using Openimg",
    subtitle: "One API token puts the image host into PicGo, Typora, a script, or CI.",
    toc: {
      token: "Get a token",
      curl: "Upload in one command",
      picgo: "PicGo",
      typora: "Typora",
      errors: "When it fails",
      limits: "Limits",
      more: "Other endpoints",
    },
    token: {
      heading: "Get a token",
      settingsLink: "Settings → API tokens",
      createIn: (link: string, once: string) =>
        `Create one in ${link}. The plaintext is ${once} — close the dialog and it is gone. The server keeps only a hash, so a lost token cannot be recovered, only replaced.`,
      onceEmphasis: "shown exactly once",
      headerForms: (prefix: string) =>
        `Tokens start with ${prefix}. Three header forms are accepted; the server takes the first non-empty one in this order:`,
      commentCommon: "the usual one",
      commentBare: "bare, must start with oimg_",
      commentNoBearer: "for clients that will not add a Bearer prefix",
      note: (bearer: string, warn: string, apiKey: string) =>
        `The third form exists for tools like PicGo and Typora: they can set a custom header but will not necessarily add the ${bearer} prefix. ${warn}, though — CORS allows only Origin, Content-Type, Accept and Authorization, so ${apiKey} is rejected at the preflight.`,
      noteWarn: "Do not use it from a browser script",
    },
    curl: {
      heading: "Upload in one command",
      constraints: (multipart: string, field: string) =>
        `Two hard constraints: the request must be ${multipart}, and the field must be named ${field}. The server reads that field and nothing else — quality, format and width are ignored, because conversion follows your account settings rather than the request.`,
      success: (code: string) => `Success is ${code}. The response looks like this (common fields only):`,
      directLinkOnly: "If all you want is the direct link:",
      note: (imageKey: string, dedupKey: string, path: string, wrong: string) =>
        `The top level has exactly two keys, ${imageKey} and ${dedupKey}. Wherever a JSON path is asked for, write ${path}, not ${wrong} — this is the single most common mistake when wiring it up.`,
      noteEmphasis: (imageKey: string) => `The image object lives under ${imageKey}`,
      dedup: (flag: string, emphasis: string) =>
        `${flag} means the server already had these exact bytes: nothing is re-encoded and no object is written, but ${emphasis}.`,
      dedupEmphasis: "the quota is still charged in full",
      variants: (variants: string, isString: string, urls: string) =>
        `Note also that ${variants} is a comma-separated ${isString} while ${urls} is the object — both fields are present, and they are easy to confuse.`,
      variantsEmphasis: "string",
    },
    picgo: {
      heading: "PicGo",
      install: (plugin: string) => `Install the ${plugin} plugin, then fill in:`,
      fieldApiUrl: "API URL",
      fieldPostName: "POST field name",
      fieldJsonPath: "JSON path",
      fieldHeaders: "Custom headers",
      fieldBody: "Custom body",
      valueEmpty: "leave empty",
      shortLink: (path: string) =>
        `To have it paste short links instead, change the JSON path to ${path}.`,
      note: 'Deleting from the PicGo album only removes the local record; the image stays on the server, because a custom web uploader has no delete callback. See section 7 for deleting for real.',
    },
    typora: {
      heading: "Typora",
      intro: (emphasis: string) =>
        `${emphasis} Its uploader list has only PicGo, iPic, uPic and “custom command” — there is no form for an API URL and headers. Two ways round it:`,
      introEmphasis: "Typora cannot talk to this API directly.",
      viaPicgo: "Through PicGo (recommended)",
      viaPicgoSteps: (picgoApp: string) =>
        `Set PicGo up as in the previous section, then Typora → Preferences → Image → Upload Service → ${picgoApp}, point it at the executable, and click “Test Uploader”.`,
      viaScript: "Without PicGo: a custom command",
      viaScriptIntro: (path: string, chmod: string) =>
        `Typora passes the file paths as arguments and reads the last lines of output as URLs. Save this as ${path} and ${chmod} it:`,
      viaScriptTail: (custom: string, curl: string, jq: string) =>
        `Then set the upload service to ${custom} with this script as the command. It needs ${curl} and ${jq} on the machine.`,
    },
    errors: {
      heading: "When it fails",
      shape: (json: string) => `Errors are always ${json}; a few carry extra fields.`,
      colStatus: "Status",
      colFix: "What to do",
      note: (emphasis: string, retryAfter: string, used: string, limit: string) =>
        `${emphasis} The status code alone is not enough: with ${retryAfter} it is the rate limit (60 per IP per minute) and backing off works; with ${used} / ${limit} the daily quota is spent, and backing off will not help — that one waits for tomorrow.`,
      noteEmphasis: "The two 429s need different handling.",
    },
    limits: {
      heading: "Limits",
      intro: (emphasis: string) =>
        `File size, daily count, allowed formats and pixel dimensions all follow your ${emphasis}, and an admin may have changed them. Read them rather than hardcoding them:`,
      introEmphasis: "tier",
      fields: (a: string, b: string, c: string, d: string) =>
        `The response carries ${a}, ${b} and ${c}, along with ${d} for the space left.`,
      note: (emphasis: string) =>
        `The size limit applies to ${emphasis}, not to the file alone — multipart boundaries and headers count toward it. A file whose byte count exactly equals the limit is therefore still rejected; leaving 1% of headroom is the safe move.`,
      noteEmphasis: "the whole request body",
    },
    more: {
      heading: "Other endpoints",
      intro: "These take the same token:",
      commentList: "list (paging, search, sort)",
      commentDetail: "one image",
      commentDelete: "delete one",
      commentBulk: "delete several",
      commentCheckin: "daily check-in for storage",
      note: (emphasis: string) =>
        `Changing the password, managing tokens and connecting your own storage ${emphasis} — those are web-only. A leaked token should not be able to take over the account: it can upload and delete images, but it cannot change the password, mint another token, or read your S3 credentials.`,
      noteEmphasis: "cannot be done with a token",
    },
    errorRows: [
      /* The `error` column is the API's own wording, taken from the backend
         catalogue, so this table and the responses cannot drift apart. */
      ["401", "auth: invalid token format", "The token does not start with oimg_ — usually a character lost while pasting"],
      ["401", "auth: invalid token", "No such token: deleted, or copied wrong"],
      ["401", "auth: token expired", "Expired. Create another (leave the expiry blank for one that never expires)"],
      ["401", "Not signed in", "No authentication header at all"],
      ["403", "Verify your email before uploading", "Carries code: email_unverified. Verify on the site"],
      ["413", "File exceeds the X MB limit", "Compress and retry"],
      ["400", "Missing upload field: file", "The field is not named file, or the request is not valid multipart"],
      ["415", "Unsupported image format: X", "Not on the allow list. SVG and PDF are refused outright"],
      ["415", "Your tier cannot upload X files", "The allowed array in the response lists what you can send"],
      ["415", "Image is AxB, over the CxD limit", "Too many pixels. Note this is 415, not 413"],
      ["429", "Too many uploads, try again shortly", "Carries retry_after in seconds. Back off and retry"],
      ["429", "Daily upload limit reached", "Carries used / limit. Wait for tomorrow"],
      ["507", "Not enough space: X needed, Y left", "Delete images, or check in for more"],
      ["503", "Storage unavailable: …", "The storage backend is having trouble. Retry later"],
    ] as [string, string, string][],
    feedback: (link: string) =>
      `Found something here that disagrees with the API, or a step that is not clear? ${link}.`,
    feedbackLink: "Open an issue",
  },
};
