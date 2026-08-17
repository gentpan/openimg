export interface User {
  id: string;
  email: string;
  name: string;
  role: "user" | "admin";
  group?: string;
  quota_bytes: number;
  used_bytes: number;
  available_bytes: number;
  checkin_streak: number;
  checked_in_today: boolean;
  has_password: boolean;
  email_verified: boolean;
  google_connected: boolean;
  github_connected: boolean;
  avatar_url?: string;
  referral_code?: string;
  default_profile_id?: string;
  upload_mode: "optimized" | "original";
  variant_format: "none" | "webp" | "avif";
  max_image_width: number;
  thumb_width: number;
  thumb_format: "webp" | "avif" | "jpg";
}

export interface Passkey {
  id: string;
  name: string;
  created_at: string;
  last_used_at?: string;
}

// ----- Images -----

export type ImageStatus = "active" | "blocked" | "deleted";
export type BackupState = "none" | "pending" | "done" | "failed";

export interface Image {
  short_code?: string;
  short_url?: string;
  id: string;
  user_id: string;
  profile_id: string;
  sha256: string;
  object_key: string;
  orig_name: string;
  mime: string;
  ext: string;
  width: number;
  height: number;
  size_orig: number;
  size_stored: number;
  size_primary: number;
  size_variants: number;
  size_thumbs: number;
  variants: string;
  backup_state: BackupState;
  status: ImageStatus;
  view_count: number;
  created_at: string;

  // Assembled server-side from the owning storage profile.
  url: string;
  variant_urls: Record<string, string>;
  thumb_url: string;
  markdown: string;
  html: string;
  bbcode: string;
}

// ----- Quota & check-in -----

export interface QuotaTransaction {
  id: string;
  type:
    | "signup_grant"
    | "checkin"
    | "referral"
    | "admin_grant"
    | "upload"
    | "delete_refund";
  bytes: number;
  quota_after: number;
  used_after: number;
  reason: string;
  image_id?: string;
  created_at: string;
}

export interface CheckinRecord {
  id: string;
  date: string;
  bytes: number;
  streak: number;
  created_at: string;
}

export interface Tier {
  name: string;
  description: string;
  max_file_size: number;
  daily_upload_count: number;
  allowed_formats: string[];
  max_total_space: number;
  allow_byos: boolean;
  max_profiles: number;
}

export interface QuotaInfo {
  quota_bytes: number;
  used_bytes: number;
  available_bytes: number;
  image_count: number;
  uploads_today: number;
  tier: Tier;
  checkin: {
    checked_in_today: boolean;
    /** Lifetime bytes earned from check-ins, weekly/monthly bonuses included. */
    total_earned: number;
    streak: number;
    last_date: string;
    next_min_bytes: number;
    next_max_bytes: number;
    streak_bonus_days: number;
    min_bytes: number;
    max_bytes: number;
    streak_bonus: number;
    /** Milestone rewards. Paid on the day the streak closes a whole week or
     *  month — not every day after, which is what streak_bonus used to do. */
    week_bonus: number;
    month_bonus: number;
    days_per_week: number;
    days_per_month: number;
  };
}

export interface CheckinResult {
  granted_bytes: number;
  bonus_bytes: number;
  streak: number;
  date: string;
  quota_bytes: number;
  capped: boolean;
  /** AI generations this check-in threw in — 0 where the group grants none.
   *  Absent on the capped reply, which the server builds field by field. */
  ai_credits?: number;
}

// ----- Storage profiles -----

export type ProfileKind = "platform" | "user_r2" | "user_s3";

/** 管理端图片列表的行：比用户自己的多了上传者身份。 */
export interface AdminImage extends Image {
  owner_id: string;
  owner_email: string;
  owner_name?: string;
}

export interface StorageProfile {
  id: string;
  kind: ProfileKind;
  name: string;
  endpoint: string;
  region: string;
  bucket: string;
  key_prefix: string;
  path_style: boolean;
  access_key_mask: string;
  public_base_url: string;
  is_default: boolean;
  is_platform: boolean;
  backup_of_id?: string;
  status: "active" | "invalid" | "testing";
  last_error?: string;
  last_checked_at?: string;
  image_count: number;
  stored_bytes: number;
}

/**
 * What the user actually fills in. `kind` and `path_style` are deliberately
 * absent — the server infers both from the endpoint, since R2, AWS, MinIO and
 * the rest all speak the same S3 API and the distinction is ours to make.
 */
export interface ProfileInput {
  name: string;
  endpoint: string;
  region: string;
  bucket: string;
  key_prefix: string;
  access_key: string;
  secret_key: string;
  public_base_url: string;
  test_only?: boolean;
}

// ----- API tokens -----

export interface ApiToken {
  id: string;
  name: string;
  prefix: string;
  revoked: boolean;
  last_used_at?: string;
  expires_at?: string;
  created_at: string;
}

// ----- Public / admin stats -----

export interface PublicStats {
  total_images: number;
  total_users: number;
  stored_bytes: number;
  images_today: number;
}


export interface DaySeries {
  date: string;
  uploads: number;
  bytes: number;
}

export interface FormatBreakdown {
  ext: string;
  count: number;
  bytes: number;
}

export interface ProfileBreakdown {
  name: string;
  kind: string;
  count: number;
  bytes: number;
}

export interface RecentLogin {
  email: string;
  ip: string;
  user_agent: string;
  login_at: string;
}

export interface BackupFailure {
  id: string;
  orig_name: string;
  error: string;
  created_at: string;
}

export interface TopUser {
  email: string;
  used_bytes: number;
  quota_bytes: number;
  image_count: number;
}

export interface Dashboard {
  stats: {
    total_users: number;
    total_images: number;
    images_today: number;
    checkins_today: number;
    blocked_images: number;
    queue_depth: number;
    pending_backup: number;
    stored_bytes: number;
    dedup_saved_bytes: number;
    original_bytes: number;
    granted_bytes: number;
  };
  uploads_by_day: DaySeries[];
  by_format: FormatBreakdown[];
  by_profile: ProfileBreakdown[];
  recent_logins: RecentLogin[];
  backup_failures: BackupFailure[];
  active_sessions: number;
  top_users_by_space: TopUser[];
}

export interface AdminUser {
  id: string;
  email: string;
  name: string;
  role: string;
  status: string;
  group_id?: string;
  group_name?: string;
  created_at: string;
  last_login_at?: string;
  signup_ip?: string;
  last_login_ip?: string;
  image_count: number;
  quota_bytes: number;
  used_bytes: number;
}

export interface UserGroup {
  id: string;
  name: string;
  description: string;
  max_file_size: number;
  daily_upload_count: number;
  allowed_formats: string;
  max_width: number;
  max_height: number;
  signup_space: number;
  checkin_min_space: number;
  checkin_max_space: number;
  streak_bonus_space: number;
  streak_bonus_days: number;
  referral_space: number;
  max_total_space: number;
  allow_byos: boolean;
  max_profiles: number;
}

export interface AdminQuotaTx {
  id: string;
  user_email: string;
  user_name: string;
  type: string;
  bytes: number;
  quota_after: number;
  used_after: number;
  reason: string;
  /** Filename joined from the image, when the row has one. Check-ins, referral
   *  bonuses and admin adjustments have no image and leave this null. */
  image_name: string | null;
  created_at: string;
}

export interface Report {
  id: string;
  image_id: string;
  category?: string;
  reason: string;
  contact: string;
  status: "open" | "resolved";
  created_at: string;
  object_key: string;
  profile_id: string;
  owner_email: string;
  owner_id: string;
  image_status: ImageStatus;
  orig_name: string;
  short_code?: string;
  width: number;
  height: number;
  size_stored: number;
  anonymous: boolean;
  reports_on_image: number;
}

export interface OAuthProviderStatus {
  client_id: string;
  source?: "env" | "admin" | "";
  editable?: boolean;
  secret_state: string;
  enabled: boolean;
  redirect_uri: string;
  console_url: string;
}

export interface OAuthStatus {
  can_store: boolean;
  google: OAuthProviderStatus;
  github: OAuthProviderStatus;
  passkey: { enabled: boolean };
  email_otp: { enabled: boolean };
  base_url: string;
}

// ----- AI image generation -----

/**
 * A generation is asynchronous: the POST returns the moment the upstream task
 * is accepted, and everything after that is observed by polling. `pending` and
 * `running` are both "still working" — the split exists because the queue only
 * flips to running once a worker has actually picked the job up.
 */
export type AIGenStatus = "pending" | "running" | "completed" | "failed";

export interface AIGeneration {
  id: string;
  prompt: string;
  model: string;
  size: string;
  resolution: string;
  status: AIGenStatus;
  /** Only on `failed`. The upstream message, shown to the user verbatim. */
  error?: string;
  /** Only on `completed`. Keys into the `images` map of the same response. */
  image_id?: string;
  /** How many generations this record consumed. Refunded whole on failure. */
  credits: number;
  created_at: string;
  done_at?: string;
}

/**
 * Two shapes, discriminated on `enabled`.
 *
 * A deployment without an upstream key answers `{enabled: false}` and nothing
 * else — the feature does not exist there, so the union refuses to hand out
 * quota numbers that were never sent.
 */
export interface AIStatusOff {
  enabled: false;
}

export interface AIStatusOn {
  enabled: true;
  /** Generations left this month. Reset monthly, not accumulated. */
  credits: number;
  used_today: number;
  daily_limit: number;
  /** The monthly allowance, for reading `credits` as a fraction. */
  monthly: number;
  /** min(credits, daily_limit − used_today) — what is left right now. */
  remaining: number;
  /** Aspect ratios the server accepts, e.g. "16:9". */
  sizes: string[];
  /** Billing tiers, not pixel sizes: "1k" / "2k". */
  resolutions: string[];
}

export type AIStatus = AIStatusOff | AIStatusOn;
