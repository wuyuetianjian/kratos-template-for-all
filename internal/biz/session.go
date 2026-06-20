package biz

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"
	"time"
)

const (
	SessionStatusActive  = "active"
	SessionStatusKicked  = "kicked"
	SessionStatusExpired = "expired"

	settingAuditLogRetentionDays   = "audit_log_retention_days"
	settingSessionLogRetentionDays = "session_log_retention_days"

	defaultAuditLogRetentionDays   = 90
	defaultSessionLogRetentionDays = 30
)

type UserSession struct {
	ID           int64
	UserID       int64
	Username     string
	TokenHash    string
	IP           string
	Browser      string
	OS           string
	Status       string
	KickedBy     string
	LoginAt      time.Time
	LastAccessAt time.Time
}

type AuditLogEntry struct {
	ID           int64
	UserID       int64
	Username     string
	Action       string
	ResourceType string
	ResourceName string
	IP           string
	Detail       string
	CreatedAt    time.Time
}

type SystemSettings struct {
	AuditLogRetentionDays   int32
	SessionLogRetentionDays int32
}

type SessionRepo interface {
	CreateSession(ctx context.Context, session *UserSession) error
	FindSessionByTokenHash(ctx context.Context, tokenHash string) (*UserSession, error)
	UpdateSessionLastAccess(ctx context.Context, tokenHash string) error
	ListSessions(ctx context.Context, page Page) ([]UserSession, int, error)
	KickSession(ctx context.Context, sessionID int64, kickedBy string) error
	DeleteSessionsBefore(ctx context.Context, before time.Time) error
}

type AuditLogRepo interface {
	CreateAuditLog(ctx context.Context, entry *AuditLogEntry) error
	ListAuditLogs(ctx context.Context, action string, page Page) ([]AuditLogEntry, int, error)
	DeleteAuditLogsBefore(ctx context.Context, before time.Time) error
}

type SettingsRepo interface {
	GetSettings(ctx context.Context) (*SystemSettings, error)
	UpdateSettings(ctx context.Context, settings *SystemSettings) error
}

func TokenHash(rawToken string) string {
	sum := sha256.Sum256([]byte(rawToken))
	return fmt.Sprintf("%x", sum)
}

func ParseUserAgent(ua string) (browser, os string) {
	ua = strings.ToLower(ua)
	switch {
	case strings.Contains(ua, "edg/"):
		browser = "Edge"
	case strings.Contains(ua, "opr/") || strings.Contains(ua, "opera"):
		browser = "Opera"
	case strings.Contains(ua, "chrome"):
		browser = "Chrome"
	case strings.Contains(ua, "safari") && !strings.Contains(ua, "chrome"):
		browser = "Safari"
	case strings.Contains(ua, "firefox"):
		browser = "Firefox"
	default:
		browser = "Unknown"
	}
	switch {
	case strings.Contains(ua, "windows"):
		os = "Windows"
	case strings.Contains(ua, "iphone") || strings.Contains(ua, "ipad"):
		os = "iOS"
	case strings.Contains(ua, "mac os"):
		os = "macOS"
	case strings.Contains(ua, "android"):
		os = "Android"
	case strings.Contains(ua, "linux"):
		os = "Linux"
	default:
		os = "Unknown"
	}
	return
}

func (uc *UseCase) CreateSession(ctx context.Context, tokenHash, ip, browser, os string, userID int64, username string) error {
	return uc.sessionRepo.CreateSession(ctx, &UserSession{
		UserID:    userID,
		Username:  username,
		TokenHash: tokenHash,
		IP:        ip,
		Browser:   browser,
		OS:        os,
		Status:    SessionStatusActive,
	})
}

func (uc *UseCase) CheckSession(ctx context.Context, tokenHash string) error {
	session, err := uc.sessionRepo.FindSessionByTokenHash(ctx, tokenHash)
	if err != nil {
		return nil
	}
	if session.Status == SessionStatusKicked {
		return ErrUnauthorized()
	}
	_ = uc.sessionRepo.UpdateSessionLastAccess(ctx, tokenHash)
	return nil
}

func (uc *UseCase) ListSessions(ctx context.Context, page Page) ([]UserSession, int, error) {
	return uc.sessionRepo.ListSessions(ctx, page.normalize())
}

func (uc *UseCase) KickSession(ctx context.Context, sessionID int64) error {
	auth, ok := AuthFromContext(ctx)
	if !ok {
		return ErrUnauthorized()
	}
	return uc.sessionRepo.KickSession(ctx, sessionID, auth.Username)
}

func (uc *UseCase) LogAuditEvent(ctx context.Context, action, resourceType, resourceName, ip, detail string) {
	auth, _ := AuthFromContext(ctx)
	var userID int64
	var username string
	if auth != nil {
		userID = auth.UserID
		username = auth.Username
	}
	_ = uc.auditRepo.CreateAuditLog(ctx, &AuditLogEntry{
		UserID:       userID,
		Username:     username,
		Action:       action,
		ResourceType: resourceType,
		ResourceName: resourceName,
		IP:           ip,
		Detail:       detail,
	})
}

func (uc *UseCase) ListAuditLogs(ctx context.Context, action string, page Page) ([]AuditLogEntry, int, error) {
	return uc.auditRepo.ListAuditLogs(ctx, action, page.normalize())
}

func (uc *UseCase) GetSettings(ctx context.Context) (*SystemSettings, error) {
	return uc.settingsRepo.GetSettings(ctx)
}

func (uc *UseCase) UpdateSettings(ctx context.Context, settings *SystemSettings) error {
	return uc.settingsRepo.UpdateSettings(ctx, settings)
}

func (uc *UseCase) scheduleCleanup() {
	_, _ = uc.cron.AddFunc("0 0 * * *", func() {
		ctx := context.Background()
		settings, err := uc.settingsRepo.GetSettings(ctx)
		if err != nil {
			uc.log.Warn("failed to load settings for cleanup", "error", err)
			return
		}
		auditDays := settings.AuditLogRetentionDays
		if auditDays <= 0 {
			auditDays = defaultAuditLogRetentionDays
		}
		sessionDays := settings.SessionLogRetentionDays
		if sessionDays <= 0 {
			sessionDays = defaultSessionLogRetentionDays
		}
		if err := uc.auditRepo.DeleteAuditLogsBefore(ctx, time.Now().AddDate(0, 0, -int(auditDays))); err != nil {
			uc.log.Warn("audit log cleanup failed", "error", err)
		}
		if err := uc.sessionRepo.DeleteSessionsBefore(ctx, time.Now().AddDate(0, 0, -int(sessionDays))); err != nil {
			uc.log.Warn("session cleanup failed", "error", err)
		}
	})
	uc.cron.Start()
}
