package api

import (
	"testing"

	"github.com/gentpan/openimg/backend/internal/models"
)

// A code is bound to the action it was mailed for. Without this, a code the
// user was talked into reading out of a "confirm your password change" email
// would also open the login endpoint — the standard OTP-phishing shape.
func TestOTPHashIsPurposeScoped(t *testing.T) {
	const email, code = "user@example.com", "123456"

	purposes := []string{
		models.OTPPurposeLogin,
		models.OTPPurposePassword,
		models.OTPPurposePasskey,
		models.OTPPurposePurge,
		models.OTPPurposeReset,
		models.OTPPurposeRegister,
	}
	seen := map[string]string{}
	for _, p := range purposes {
		h := otpHash(email, p, code)
		if prev, dup := seen[h]; dup {
			t.Fatalf("purposes %q and %q hash the same code identically", prev, p)
		}
		seen[h] = p
	}

	// And to the address, so a code cannot be replayed against another account.
	if otpHash(email, models.OTPPurposeLogin, code) ==
		otpHash("other@example.com", models.OTPPurposeLogin, code) {
		t.Error("same code hashes identically for two different addresses")
	}
}

func TestValidOTPPurposeRejectsUnknown(t *testing.T) {
	for _, bad := range []string{"", "admin", "delete-account", "LOGIN"} {
		if models.ValidOTPPurpose(bad) {
			t.Errorf("ValidOTPPurpose(%q) = true, want false", bad)
		}
	}
	for _, ok := range []string{"login", "password", "passkey", "purge", "reset", "register"} {
		if !models.ValidOTPPurpose(ok) {
			t.Errorf("ValidOTPPurpose(%q) = false, want true", ok)
		}
	}
}

func TestMaskEmailKeepsDomainHidesLocal(t *testing.T) {
	cases := map[string]string{
		"123456@qq.com":     "12***@qq.com",
		"a@b.com":           "a***@b.com",
		"verylong@mail.org": "ve***@mail.org",
		"notanemail":        "notanemail",
	}
	for in, want := range cases {
		if got := maskEmail(in); got != want {
			t.Errorf("maskEmail(%q) = %q, want %q", in, got, want)
		}
	}
}
