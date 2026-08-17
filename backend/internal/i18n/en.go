package i18n

// The English catalogue.
//
// Kept in the same order as zh.go so the two can be read side by side. The
// tests enforce that the key sets match and that the format verbs line up —
// one extra %s here renders "%!s(MISSING)" into a real response.
var en = map[string]string{
	// Upload
	"upload.email_unverified": "Verify your email before uploading",
	"upload.suspended":        "This account is suspended",
	"upload.daily_limit":      "Daily upload limit reached",
	"upload.too_large":        "File exceeds the %.1f MB limit",
	"upload.missing_field":    "Missing upload field: file",
	"upload.read_failed":      "Could not read the upload: %s",
	"upload.format_denied":    "Your tier cannot upload %s files",
	"upload.rate_limited":     "Too many uploads, try again shortly",
	"upload.key_failed":       "Could not generate a storage path, try again",

	// Storage
	"storage.unavailable":           "Storage unavailable: %s",
	"storage.unavailable_bare":      "Storage unavailable",
	"storage.quota":                 "Not enough space: %s needed, %s left",
	"storage.write_failed":          "Storage write failed: %s",
	"storage.write_variant_failed":  "Could not write the %s derivative format: %s",
	"storage.local_write_failed":    "Could not write to local storage: %s",
	"storage.local_verify_mismatch": "Local storage read back different bytes than were written",
	"storage.platform_pool":         "Platform storage pool",
	"storage.s3.unreachable":        "Cannot access bucket %q: %s",
	"storage.s3.test_write_failed":  "Test write failed (the credentials may be read-only): %s",
	"storage.s3.test_read_failed":   "Could not read back the test object: %s",
	"storage.s3.test_body_failed":   "Could not read the test object contents: %s",
	"storage.s3.test_mismatch":      "Test object contents did not match; another service may be using this bucket",

	// Auth
	"auth.not_authenticated": "Not signed in",
	"auth.bad_params":        "Invalid parameters",
	"auth.email_invalid":     "Enter a valid email address",
	"auth.email_taken":       "That email is already registered, sign in instead",
	"auth.credentials_short": "Enter an email and a password of at least 8 characters",
	"auth.name_required":     "Enter a display name",
	"auth.name_too_long":     "Display name can be at most 32 characters",
	"auth.otp_required":      "Enter the 6-digit email code",
	"auth.otp_needed":        "A 6-digit email code is required",
	"auth.password_short":    "New password must be at least 8 characters",
	"auth.password_hash":     "Could not process the password",
	"auth.native_code":       "Login code is invalid or expired, sign in again",

	// Account
	"account.not_found":         "Account not found",
	"account.delete.mismatch":   "The confirmation text does not match your account email",
	"account.delete.last_admin": "This is the last admin account and cannot be deleted",
	"account.delete.failed":     "Delete failed: %s",

	// Email codes
	"otp.not_configured":     "Email is not configured, codes cannot be sent",
	"otp.rate_limited":       "Try again shortly, the last code was sent less than a minute ago",
	"otp.invalid_or_expired": "The code is invalid or has expired",
	"otp.too_many_attempts":  "Too many attempts, request a new code",
	"otp.incorrect":          "Incorrect code",
	"otp.purpose_unknown":    "Unsupported verification purpose",

	// Purpose labels, spliced into the email subject line.
	"otp.purpose.password": "change password",
	"otp.purpose.passkey":  "add passkey",
	"otp.purpose.purge":    "clear gallery",
	"otp.purpose.storage":  "change storage settings", "otp.purpose.reset": "reset password",
	"ai.disabled":          "AI generation is not enabled on this site",
	"ai.prompt_required":   "Describe the image you want first",
	"ai.prompt_too_long":   "That description is too long — keep it under 1000 characters",
	"ai.daily_limit":       "You've used today's generations — try again tomorrow",
	"ai.no_credits":        "You're out of generations this month; daily check-in grants more",
	"ai.submit_failed":     "Could not submit the generation: %s",
	"otp.purpose.register": "sign up",
	"otp.purpose.login":    "sign in",

	// Email bodies
	"email.otp.subject":    "Openimg %s code: %s",
	"email.otp.heading":    "Openimg verification code",
	"email.otp.body":       "Enter the code below on the sign-in page. It is valid for %d minutes.",
	"email.otp.warning":    "If this wasn't you, ignore this email. No one, including Openimg staff, will ever ask you for this code.",
	"email.footer.tagline": "Openimg · free image host",

	// Passkeys
	"passkey.otp_required":     "An email code is required",
	"passkey.none_registered":  "This account has no passkey",
	"passkey.login_failed":     "Sign-in failed",
	"passkey.register_expired": "Registration session expired, try again",
	"passkey.login_expired":    "Sign-in session expired, try again",

	// OAuth
	"oauth.no_user_id":       "Provider %s is not supported, or returned no user id",
	"oauth.linked_elsewhere": "This %s account is already linked to another user (%s)",
	"oauth.unlink_last":      "Cannot unlink: this is your only sign-in method. Set a password or link another account first",

	// Image processing
	"image.not_found":          "Image not found",
	"image.too_large":          "Image is %dx%d, over the %dx%d limit",
	"image.format_unsupported": "Unsupported image format: %s",
	"imageproc.decode_failed":  "Decoding failed: %s",
	"imageproc.rotate_failed":  "Rotation failed: %s",
	"imageproc.resize_failed":  "Resizing failed: %s",
	"imageproc.strip_failed":   "Could not strip metadata: %s",
	"imageproc.crop_failed":    "Could not crop the avatar: %s",
	"imageproc.encode_failed":  "Could not encode the avatar: %s",

	// Derivative sizes
	"variant.size_unsupported": "Unsupported size",
	"variant.read_failed":      "Could not read the original: %s",
	"variant.generate_failed":  "Generation failed: %s",
	"variant.no_space":         "Not enough storage to generate this size (%s needed)",
	"variant.not_worth_it":     "The requested size is not smaller than the original, nothing to generate",

	// Avatars
	"avatar.choose_image": "Choose an image",
	"avatar.too_large":    "Avatar must be 8 MB or smaller",
	"avatar.read_failed":  "Read failed: %s",
	"avatar.bad_format":   "Unsupported image format",

	// Gallery
	"gallery.purge_otp_required": "Clearing your gallery requires an email code",
	"gallery.bad_image_id":       "Invalid image ID",
	"gallery.no_selection":       "No images selected",

	// Share page
	"share.link_not_found": "This link does not exist or has expired",
	"share.image_blocked":  "This image has been blocked",
	"share.bad_reaction":   "Unsupported reaction",

	// Reports
	"report.received":           "Report received, we will look into it shortly",
	"report.bad_image_id":       "Invalid image_id",
	"report.reason_required":    "Pick a report category or add a description",
	"report.rate_limited":       "Too many reports, try again later",
	"report.category.porn":      "Sexual content",
	"report.category.copyright": "Copyright infringement",
	"report.category.violence":  "Violence or terrorism",
	"report.category.privacy":   "Personal privacy",
	"report.category.fraud":     "Scam or malicious link",
	"report.category.other":     "Other violation",

	// API tokens
	"token.limit_reached": "API token limit reached, delete one you no longer use",
	"token.shown_once":    "This token is shown only once, save it now",
	"token.bad_id":        "Invalid token id",
	"token.not_found":     "Token not found",

	// Upload preferences
	"prefs.bad_upload_mode":    "upload_mode must be optimized or original",
	"prefs.bad_variant_format": "variant_format must be none, webp, or avif",
	"prefs.bad_width_preset":   "Unsupported width preset",
	"prefs.bad_thumb_width":    "That thumbnail width is not one of the presets",
	"prefs.bad_thumb_format":   "Thumbnail format must be WebP, AVIF or JPEG",
	"prefs.serialize_failed":   "Could not serialize the settings",
	"prefs.save_failed":        "Save failed",

	// Daily check-in
	"checkin.already_done": "You already checked in today",
	"checkin.no_reward":    "No check-in reward is configured for your tier",
	"checkin.capped":       "Checked in, but your storage is at its cap so nothing was granted",

	// Ledger reasons — every user sees these in their own storage ledger.
	"quota.reason.upload":             "Upload %s",
	"quota.reason.upload_dedup":       "Upload %s (deduplicated)",
	"quota.reason.upload_refund":      "Upload failed, refunded",
	"quota.reason.dedup_refund":       "Deduplicated upload failed, refunded",
	"quota.reason.delete_image":       "Delete %s",
	"quota.reason.bulk_delete":        "Bulk delete %d images",
	"quota.reason.generate_size":      "Generate %s size",
	"quota.reason.generate_refund":    "Size generation failed, refunded",
	"quota.reason.signup_bonus":       "Signup storage bonus",
	"quota.reason.initial_backfill":   "Initial storage (ledger backfill)",
	"quota.reason.referral_invitee":   "Referral signup bonus (from %s)",
	"quota.reason.referral_referrer":  "Referral bonus for inviting %s",
	"quota.reason.admin_adjust":       "Manual admin adjustment",
	"quota.reason.checkin":            "Daily check-in (%d-day streak)",
	"quota.reason.checkin_week":       "Daily check-in (%d-day streak · full-week bonus)",
	"quota.reason.checkin_month":      "Daily check-in (%d-day streak · full-month bonus)",
	"quota.reason.checkin_week_month": "Daily check-in (%d-day streak · full-week + full-month bonus)",

	// Short links
	"shortlink.failed": "Could not generate a short link",

	// Rate limits
	"ratelimit.too_frequent": "Too many requests",

	// Endpoint validation
	"endpoint.required":          "Endpoint is required",
	"endpoint.malformed":         "Invalid endpoint format: %s",
	"endpoint.scheme_required":   "Endpoint must start with http:// or https://",
	"endpoint.https_required":    "Endpoint must use https (plain http would leak your keys in transit)",
	"endpoint.host_missing":      "Endpoint has no host name",
	"endpoint.host_unresolvable": "Cannot resolve host %q: %s",
	"endpoint.loopback":          "Endpoint points to a loopback address (%s), which is not allowed",
	"endpoint.private":           "Endpoint points to a private network address (%s), which is not allowed",
	"endpoint.link_local":        "Endpoint points to a link-local address (%s), which is not allowed",
	"endpoint.unspecified":       "Endpoint points to an unspecified address (%s), which is not allowed",
	"endpoint.multicast":         "Endpoint points to a multicast address (%s), which is not allowed",
	"endpoint.cgnat":             "Endpoint points to a carrier-grade NAT address (%s), which is not allowed",
	"endpoint.public_required":   "Public URL is required",
	"endpoint.public_malformed":  "Invalid public URL format: %s",
	"endpoint.public_scheme":     "Public URL must start with http:// or https://",
	"endpoint.public_domain":     "Public URL has no domain",

	// Storage locations
	"profile.byos_denied":       "Your tier does not allow connecting your own bucket",
	"profile.no_master_key":     "The server has no storage encryption key, so credentials cannot be stored securely — contact an admin",
	"profile.limit_reached":     "Storage location limit reached",
	"profile.test_failed":       "Connection test failed: %s",
	"profile.not_ready":         "This storage location is not available, test the connection first",
	"profile.backup_as_default": "A backup location cannot be the default upload target",
	"profile.platform_backup":   "Backups for platform storage are configured by an admin",
	"profile.bad_backup_of_id":  "Invalid backup_of_id",
	"profile.backup_self":       "A storage location cannot back up itself",
	"profile.source_not_found":  "Source storage location not found",
	"profile.source_is_backup":  "That location is already a backup, so it cannot be a backup source",
	"profile.backup_exists":     "This storage location already has a backup target",
	"profile.platform_locked":   "Platform storage cannot be deleted",
	"profile.has_images":        "Images still live in this location; deleting it will make them unreachable",
	"profile.bad_id":            "Invalid storage id",
	"profile.not_found":         "Storage location not found",
	"profile.platform_readonly": "You cannot modify platform storage",
	"profile.bucket_required":   "Bucket is required",
	"profile.keys_required":     "Access key and secret key are required",
	"profile.removed_location":  "Removed storage location",
	"profile.platform":          "Platform storage",

	// Tier descriptions
	"tier.admin":   "Admin (no limits)",
	"tier.trusted": "Trusted user (long-term active)",
	"tier.free":    "Registered free user",

	// Admin console and the report mail sent to operators.
	"admin.report.bad_id":        "Invalid report id",
	"admin.report.not_found":     "Report not found",
	"admin.report.bad_action":    "action must be dismiss, block, or block_and_ban",
	"admin.image.bad_id":         "Invalid image id",
	"admin.user.bad_id":          "Invalid user id",
	"admin.user.not_found":       "User not found",
	"admin.user.is_admin":        "Admin accounts cannot be banned",
	"admin.reconcile_failed":     "Reconciliation failed: %s",
	"admin.owner_deleted":        "(account deleted)",
	"admin.storage.missing":      "Platform storage is not configured yet",
	"admin.storage.thumb_domain": "Thumbnail domain: %s",
	"admin.oauth.no_master_key":  "The server has no STORAGE_MASTER_KEY, so OAuth secrets cannot be stored encrypted; configure them with environment variables instead",
	"admin.oauth.bad_provider":   "provider must be google or github",
	"admin.oauth.secret_needed":  "A client secret is required whenever a client ID is set",
	"admin.oauth.from_console":   "Configured (admin console)",
	"admin.oauth.from_env":       "Configured (environment variable)",

	"admin.mail.subject":     "[Openimg] New report: %s",
	"admin.mail.no_note":     "(the reporter left no additional note)",
	"admin.mail.no_contact":  "not provided",
	"admin.mail.anonymous":   "Anonymous visitor",
	"admin.mail.signed_in":   "Signed-in user",
	"admin.mail.heading":     "New report received",
	"admin.mail.reason":      "Report reason",
	"admin.mail.image":       "Reported image",
	"admin.mail.spec":        "Specs",
	"admin.mail.reporter":    "Reporter",
	"admin.mail.contact":     "Contact",
	"admin.mail.direct_link": "Direct link · open at your own discretion",
	"admin.mail.cta":         "Open the admin console",
	"admin.mail.footer":      "Openimg · report notification",
}
