package passkey

import (
	"fmt"
	"sync"
	"time"

	"github.com/gentpan/openimg/backend/internal/models"
	"github.com/go-webauthn/webauthn/protocol"
	"github.com/go-webauthn/webauthn/webauthn"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Service wraps the WebAuthn instance and an in-memory map of in-flight
// challenge sessions. One process is fine for our scale; if we ever scale
// out, move sessions to Postgres or Redis.
type Service struct {
	wa    *webauthn.WebAuthn
	db    *gorm.DB
	mu    sync.Mutex
	flows map[string]*flowSession
}

type flowSession struct {
	UserID uuid.UUID
	Data   *webauthn.SessionData
	Expire time.Time
}

func New(db *gorm.DB, rpID, displayName, origin string) (*Service, error) {
	wa, err := webauthn.New(&webauthn.Config{
		RPDisplayName: displayName,
		RPID:          rpID,
		RPOrigins:     []string{origin},
	})
	if err != nil {
		return nil, err
	}
	s := &Service{wa: wa, db: db, flows: map[string]*flowSession{}}
	go s.gc()
	return s, nil
}

func (s *Service) gc() {
	t := time.NewTicker(2 * time.Minute)
	for range t.C {
		s.mu.Lock()
		now := time.Now()
		for k, v := range s.flows {
			if v.Expire.Before(now) {
				delete(s.flows, k)
			}
		}
		s.mu.Unlock()
	}
}

func (s *Service) saveFlow(userID uuid.UUID, data *webauthn.SessionData) string {
	id := uuid.New().String()
	s.mu.Lock()
	s.flows[id] = &flowSession{UserID: userID, Data: data, Expire: time.Now().Add(5 * time.Minute)}
	s.mu.Unlock()
	return id
}

func (s *Service) loadFlow(id string) *flowSession {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, ok := s.flows[id]
	if !ok || f.Expire.Before(time.Now()) {
		delete(s.flows, id)
		return nil
	}
	delete(s.flows, id) // single-use
	return f
}

// ----- begin enroll: user wants to add a new passkey -----

func (s *Service) BeginEnroll(user *models.User) (*protocol.CredentialCreation, string, error) {
	wu := wrap(user, s.loadCredentials(user.ID))
	options, sessionData, err := s.wa.BeginRegistration(wu)
	if err != nil {
		return nil, "", err
	}
	flowID := s.saveFlow(user.ID, sessionData)
	return options, flowID, nil
}

func (s *Service) FinishEnroll(user *models.User, flowID, name string, raw *protocol.ParsedCredentialCreationData) (*models.PasskeyCredential, error) {
	flow := s.loadFlow(flowID)
	if flow == nil {
		return nil, fmt.Errorf("注册会话已过期，请重新尝试")
	}
	if flow.UserID != user.ID {
		return nil, fmt.Errorf("flow owner mismatch")
	}
	wu := wrap(user, s.loadCredentials(user.ID))
	cred, err := s.wa.CreateCredential(wu, *flow.Data, raw)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	pkc := models.PasskeyCredential{
		ID:              uuid.New(),
		UserID:          user.ID,
		CredentialID:    cred.ID,
		PublicKey:       cred.PublicKey,
		AttestationType: cred.AttestationType,
		AAGUID:          cred.Authenticator.AAGUID,
		SignCount:       cred.Authenticator.SignCount,
		CloneWarning:    cred.Authenticator.CloneWarning,
		Flags:           cred.Flags.MsgpByte(),
		Name:            name,
	}
	if pkc.Name == "" {
		pkc.Name = "Passkey " + time.Now().Format("01/02 15:04")
	}
	if err := s.db.Create(&pkc).Error; err != nil {
		return nil, err
	}
	return &pkc, nil
}

// ----- begin login: user wants to sign in via passkey -----

func (s *Service) BeginLogin(user *models.User) (*protocol.CredentialAssertion, string, error) {
	wu := wrap(user, s.loadCredentials(user.ID))
	if len(wu.creds) == 0 {
		return nil, "", fmt.Errorf("此账号未绑定 Passkey")
	}
	options, sessionData, err := s.wa.BeginLogin(wu)
	if err != nil {
		return nil, "", err
	}
	flowID := s.saveFlow(user.ID, sessionData)
	return options, flowID, nil
}

func (s *Service) FinishLogin(user *models.User, flowID string, raw *protocol.ParsedCredentialAssertionData) error {
	flow := s.loadFlow(flowID)
	if flow == nil {
		return fmt.Errorf("登录会话已过期，请重新尝试")
	}
	if flow.UserID != user.ID {
		return fmt.Errorf("flow owner mismatch")
	}
	creds := s.loadCredentials(user.ID)
	syncFlagsFromAssertion(s.db, creds, raw)
	wu := wrap(user, creds)
	cred, err := s.wa.ValidateLogin(wu, *flow.Data, raw)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	// update sign_count + last_used_at + flags (post-validation, authoritative)
	now := time.Now()
	s.db.Model(&models.PasskeyCredential{}).
		Where("credential_id = ?", cred.ID).
		Updates(map[string]any{
			"sign_count":   cred.Authenticator.SignCount,
			"last_used_at": now,
			"flags":        cred.Flags.MsgpByte(),
		})
	return nil
}

// ----- discoverable (email-less) login -----
//
// Modern passkey UX: user clicks "Sign in with passkey", the browser shows the
// platform passkey picker, no email required. The credential the user picks
// tells us who they are. Backed by webauthn.BeginDiscoverableLogin (no
// allowCredentials filter) and ValidateDiscoverableLogin (lookup by handle).

func (s *Service) BeginDiscoverableLogin() (*protocol.CredentialAssertion, string, error) {
	options, sessionData, err := s.wa.BeginDiscoverableLogin()
	if err != nil {
		return nil, "", err
	}
	// userID=zero — the flow isn't bound to a user yet; the credential reveals it.
	flowID := s.saveFlow(uuid.Nil, sessionData)
	return options, flowID, nil
}

// FinishDiscoverableLogin verifies the assertion and returns the user it
// belongs to. The handler then issues a session for that user.
func (s *Service) FinishDiscoverableLogin(flowID string, raw *protocol.ParsedCredentialAssertionData) (*models.User, error) {
	flow := s.loadFlow(flowID)
	if flow == nil {
		return nil, fmt.Errorf("登录会话已过期，请重新尝试")
	}
	// Resolver maps a userHandle (the user.id we registered) → webauthn.User.
	// We also self-heal stored flags from the incoming assertion here, so a
	// passkey whose BE bit flipped (e.g. iCloud sync turned on) can still log
	// in without manual re-enrollment.
	handler := func(rawID, userHandle []byte) (webauthn.User, error) {
		uid, err := uuid.FromBytes(userHandle)
		if err != nil {
			return nil, fmt.Errorf("bad user handle: %w", err)
		}
		var u models.User
		if err := s.db.Where("id = ?", uid).First(&u).Error; err != nil {
			return nil, err
		}
		creds := s.loadCredentials(u.ID)
		syncFlagsFromAssertion(s.db, creds, raw)
		return wrap(&u, creds), nil
	}
	cred, err := s.wa.ValidateDiscoverableLogin(handler, *flow.Data, raw)
	if err != nil {
		return nil, fmt.Errorf("verify: %w", err)
	}
	// Bump sign_count + last_used_at, then return the owning user.
	var pkc models.PasskeyCredential
	if err := s.db.Where("credential_id = ?", cred.ID).First(&pkc).Error; err != nil {
		return nil, fmt.Errorf("credential owner not found: %w", err)
	}
	now := time.Now()
	s.db.Model(&pkc).Updates(map[string]any{
		"sign_count":   cred.Authenticator.SignCount,
		"last_used_at": now,
	})
	var u models.User
	if err := s.db.Where("id = ?", pkc.UserID).First(&u).Error; err != nil {
		return nil, err
	}
	return &u, nil
}

// syncFlagsFromAssertion patches both the in-memory creds slice and the DB
// row matching the assertion's credential id with the *current* flag byte
// reported by the authenticator. Required because the BE bit can flip (e.g.
// when iCloud Keychain sync is toggled on a previously local passkey) and
// webauthn.ValidateLogin treats any BE inconsistency as a hard fail. We
// trust the latest authenticator state — that's what every reference
// implementation does in practice.
func syncFlagsFromAssertion(db *gorm.DB, creds []models.PasskeyCredential, raw *protocol.ParsedCredentialAssertionData) {
	if raw == nil {
		return
	}
	incoming := uint8(raw.Response.AuthenticatorData.Flags)
	for i, c := range creds {
		if !bytesEqual(c.CredentialID, raw.RawID) {
			continue
		}
		if c.Flags == incoming {
			return
		}
		creds[i].Flags = incoming
		_ = db.Model(&models.PasskeyCredential{}).
			Where("id = ?", c.ID).
			Update("flags", incoming).Error
		return
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (s *Service) loadCredentials(userID uuid.UUID) []models.PasskeyCredential {
	var creds []models.PasskeyCredential
	s.db.Where("user_id = ?", userID).Find(&creds)
	return creds
}

// ----- webauthn.User adapter -----

type webauthnUser struct {
	id          uuid.UUID
	email, name string
	creds       []models.PasskeyCredential
}

func wrap(u *models.User, creds []models.PasskeyCredential) *webauthnUser {
	displayName := u.Name
	if displayName == "" {
		displayName = u.Email
	}
	return &webauthnUser{id: u.ID, email: u.Email, name: displayName, creds: creds}
}

func (u *webauthnUser) WebAuthnID() []byte          { return u.id[:] }
func (u *webauthnUser) WebAuthnName() string        { return u.email }
func (u *webauthnUser) WebAuthnDisplayName() string { return u.name }
func (u *webauthnUser) WebAuthnIcon() string        { return "" }
func (u *webauthnUser) WebAuthnCredentials() []webauthn.Credential {
	out := make([]webauthn.Credential, 0, len(u.creds))
	for _, c := range u.creds {
		out = append(out, webauthn.Credential{
			ID:              c.CredentialID,
			PublicKey:       c.PublicKey,
			AttestationType: c.AttestationType,
			Flags:           webauthn.CredentialFlagsFromMsgpByte(c.Flags),
			Authenticator: webauthn.Authenticator{
				AAGUID:       c.AAGUID,
				SignCount:    c.SignCount,
				CloneWarning: c.CloneWarning,
			},
		})
	}
	return out
}
