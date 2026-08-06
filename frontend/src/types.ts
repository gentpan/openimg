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
