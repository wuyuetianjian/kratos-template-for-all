package data

import (
	"context"
	"fmt"
	"time"

	"github.com/wuyuetianjian/kratos-template-for-all/internal/biz"
	"github.com/wuyuetianjian/kratos-template-for-all/internal/data/ent"
	entauditlog "github.com/wuyuetianjian/kratos-template-for-all/internal/data/ent/auditlog"
	entsystemsetting "github.com/wuyuetianjian/kratos-template-for-all/internal/data/ent/systemsetting"
	entusersession "github.com/wuyuetianjian/kratos-template-for-all/internal/data/ent/usersession"

	"entgo.io/ent/dialect/sql"
	"github.com/go-kratos/kratos/v3/errors"
)

// ── Session repo ──────────────────────────────────────────────────────────────

type sessionRepo struct {
	data *Data
}

func NewSessionRepo(data *Data) biz.SessionRepo {
	return &sessionRepo{data: data}
}

func (r *sessionRepo) GetSession(ctx context.Context, sessionID int64) (*biz.UserSession, error) {
	s, err := r.data.WriteEnt.UserSession.Query().
		Where(entusersession.ID(int(sessionID))).
		WithUser().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "session not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizSession(s), nil
}

func (r *sessionRepo) CreateSession(ctx context.Context, s *biz.UserSession) error {
	u, err := r.data.WriteEnt.User.Get(ctx, int(s.UserID))
	if err != nil {
		return err
	}
	now := time.Now()
	_, err = r.data.WriteEnt.UserSession.Create().
		SetTokenHash(s.TokenHash).
		SetIP(s.IP).
		SetBrowser(s.Browser).
		SetOs(s.OS).
		SetStatus(biz.SessionStatusActive).
		SetLoginAt(now).
		SetLastAccessAt(now).
		SetUser(u).
		Save(ctx)
	return err
}

func (r *sessionRepo) FindSessionByTokenHash(ctx context.Context, tokenHash string) (*biz.UserSession, error) {
	s, err := r.data.WriteEnt.UserSession.Query().
		Where(entusersession.TokenHash(tokenHash)).
		WithUser().
		Only(ctx)
	if ent.IsNotFound(err) {
		return nil, errors.NotFound(bizReasonNotFound, "session not found")
	}
	if err != nil {
		return nil, err
	}
	return toBizSession(s), nil
}

func (r *sessionRepo) UpdateSessionLastAccess(ctx context.Context, tokenHash string) error {
	return r.data.WriteEnt.UserSession.Update().
		Where(entusersession.TokenHash(tokenHash)).
		SetLastAccessAt(time.Now()).
		Exec(ctx)
}

func (r *sessionRepo) ListSessions(ctx context.Context, page biz.Page) ([]biz.UserSession, int, error) {
	query := r.data.WriteEnt.UserSession.Query().
		Where(entusersession.StatusIn(biz.SessionStatusActive, biz.SessionStatusKicked)).
		WithUser()
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	sessions, err := query.
		Order(entusersession.ByID(sql.OrderDesc())).
		Limit(page.Size).
		Offset(page.Token).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]biz.UserSession, 0, len(sessions))
	for _, s := range sessions {
		result = append(result, *toBizSession(s))
	}
	return result, total, nil
}

func (r *sessionRepo) KickSession(ctx context.Context, sessionID int64, kickedBy string) error {
	s, err := r.data.WriteEnt.UserSession.Get(ctx, int(sessionID))
	if ent.IsNotFound(err) {
		return errors.NotFound(bizReasonNotFound, "session not found")
	}
	if err != nil {
		return err
	}
	if s.Status != biz.SessionStatusActive {
		return errors.BadRequest(bizReasonAlreadyExists, "session is not active")
	}
	return r.data.WriteEnt.UserSession.UpdateOneID(int(sessionID)).
		SetStatus(biz.SessionStatusKicked).
		SetKickedBy(kickedBy).
		Exec(ctx)
}

func (r *sessionRepo) ExpireSession(ctx context.Context, tokenHash string) error {
	return r.data.WriteEnt.UserSession.Update().
		Where(entusersession.TokenHash(tokenHash), entusersession.Status(biz.SessionStatusActive)).
		SetStatus(biz.SessionStatusExpired).
		SetLastAccessAt(time.Now()).
		Exec(ctx)
}

func (r *sessionRepo) ExpireInactiveSessions(ctx context.Context, idleSince time.Time) ([]int64, error) {
	sessions, err := r.data.WriteEnt.UserSession.Query().
		Where(
			entusersession.Status(biz.SessionStatusActive),
			entusersession.LastAccessAtLT(idleSince),
		).
		Select(entusersession.FieldID).
		All(ctx)
	if err != nil {
		return nil, err
	}
	if len(sessions) == 0 {
		return nil, nil
	}
	ids := make([]int, 0, len(sessions))
	result := make([]int64, 0, len(sessions))
	for _, s := range sessions {
		ids = append(ids, s.ID)
		result = append(result, int64(s.ID))
	}
	_, err = r.data.WriteEnt.UserSession.Update().
		Where(entusersession.IDIn(ids...)).
		SetStatus(biz.SessionStatusExpired).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (r *sessionRepo) DeleteSessionsBefore(ctx context.Context, before time.Time) error {
	_, err := r.data.WriteEnt.UserSession.Delete().
		Where(entusersession.LoginAtLT(before)).
		Exec(ctx)
	return err
}

func toBizSession(s *ent.UserSession) *biz.UserSession {
	session := &biz.UserSession{
		ID:           int64(s.ID),
		TokenHash:    s.TokenHash,
		IP:           s.IP,
		Browser:      s.Browser,
		OS:           s.Os,
		Status:       s.Status,
		KickedBy:     s.KickedBy,
		LoginAt:      s.LoginAt,
		LastAccessAt: s.LastAccessAt,
	}
	if s.Edges.User != nil {
		session.UserID = int64(s.Edges.User.ID)
		session.Username = s.Edges.User.Username
	}
	return session
}

// ── Audit log repo ────────────────────────────────────────────────────────────

type auditLogRepo struct {
	data *Data
}

func NewAuditLogRepo(data *Data) biz.AuditLogRepo {
	return &auditLogRepo{data: data}
}

func (r *auditLogRepo) CreateAuditLog(ctx context.Context, entry *biz.AuditLogEntry) error {
	_, err := r.data.WriteEnt.AuditLog.Create().
		SetUserID(entry.UserID).
		SetUsername(entry.Username).
		SetAction(entry.Action).
		SetResourceType(entry.ResourceType).
		SetResourceName(entry.ResourceName).
		SetIP(entry.IP).
		SetDetail(entry.Detail).
		Save(ctx)
	return err
}

func (r *auditLogRepo) ListAuditLogs(ctx context.Context, filter biz.AuditLogFilter, page biz.Page) ([]biz.AuditLogEntry, int, error) {
	query := r.data.ReadEnt.AuditLog.Query()
	if filter.Action != "" {
		query = query.Where(entauditlog.Action(filter.Action))
	}
	if filter.Username != "" {
		query = query.Where(entauditlog.UsernameContainsFold(filter.Username))
	}
	if filter.ResourceType != "" {
		query = query.Where(entauditlog.ResourceType(filter.ResourceType))
	}
	if !filter.StartTime.IsZero() {
		query = query.Where(entauditlog.CreatedAtGTE(filter.StartTime))
	}
	if !filter.EndTime.IsZero() {
		query = query.Where(entauditlog.CreatedAtLTE(filter.EndTime))
	}
	total, err := query.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	logs, err := query.
		Order(entauditlog.ByID(sql.OrderDesc())).
		Limit(page.Size).
		Offset(page.Token).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	result := make([]biz.AuditLogEntry, 0, len(logs))
	for _, l := range logs {
		result = append(result, biz.AuditLogEntry{
			ID:           int64(l.ID),
			UserID:       l.UserID,
			Username:     l.Username,
			Action:       l.Action,
			ResourceType: l.ResourceType,
			ResourceName: l.ResourceName,
			IP:           l.IP,
			Detail:       l.Detail,
			CreatedAt:    l.CreatedAt,
		})
	}
	return result, total, nil
}

func (r *auditLogRepo) DeleteAuditLogsBefore(ctx context.Context, before time.Time) error {
	_, err := r.data.WriteEnt.AuditLog.Delete().
		Where(entauditlog.CreatedAtLT(before)).
		Exec(ctx)
	return err
}

// ── Settings repo ─────────────────────────────────────────────────────────────

type settingsRepo struct {
	data *Data
}

func NewSettingsRepo(data *Data) biz.SettingsRepo {
	return &settingsRepo{data: data}
}

func (r *settingsRepo) GetSettings(ctx context.Context) (*biz.SystemSettings, error) {
	settings := &biz.SystemSettings{
		AuditLogRetentionDays:   90,
		SessionLogRetentionDays: 30,
		ServiceName:             biz.DefaultServiceName,
		SiteIcon:                biz.DefaultServiceName,
		CornerIcon:              biz.DefaultServiceName,
	}
	rows, err := r.data.ReadEnt.SystemSetting.Query().All(ctx)
	if err != nil {
		return settings, nil
	}
	seen := make(map[string]bool, len(rows))
	for _, row := range rows {
		seen[row.Key] = true
		switch row.Key {
		case "audit_log_retention_days":
			var v int32
			if _, err := countSScan(row.Value, &v); err == nil && v > 0 {
				settings.AuditLogRetentionDays = v
			}
		case "session_log_retention_days":
			var v int32
			if _, err := countSScan(row.Value, &v); err == nil && v > 0 {
				settings.SessionLogRetentionDays = v
			}
		case "service_name":
			if row.Value != "" {
				settings.ServiceName = row.Value
			}
		case "site_icon":
			if row.Value != "" {
				settings.SiteIcon = row.Value
			}
		case "corner_icon":
			if row.Value != "" {
				settings.CornerIcon = row.Value
			}
		case "totp_enabled":
			settings.TOTPEnabled = row.Value == "true"
		}
	}
	_ = r.upsertSettings(ctx, defaultSettingPairs(settings, seen))
	return settings, nil
}

func (r *settingsRepo) UpdateSettings(ctx context.Context, settings *biz.SystemSettings) error {
	if settings.ServiceName == "" {
		settings.ServiceName = biz.DefaultServiceName
	}
	if settings.SiteIcon == "" {
		settings.SiteIcon = biz.DefaultServiceName
	}
	if settings.CornerIcon == "" {
		settings.CornerIcon = biz.DefaultServiceName
	}
	totpVal := "false"
	if settings.TOTPEnabled {
		totpVal = "true"
	}
	return r.upsertSettings(ctx, map[string]string{
		"audit_log_retention_days":   intToStr(settings.AuditLogRetentionDays),
		"session_log_retention_days": intToStr(settings.SessionLogRetentionDays),
		"service_name":               settings.ServiceName,
		"site_icon":                  settings.SiteIcon,
		"corner_icon":                settings.CornerIcon,
		"totp_enabled":               totpVal,
	})
}

func defaultSettingPairs(settings *biz.SystemSettings, seen map[string]bool) map[string]string {
	defaults := map[string]string{
		"audit_log_retention_days":   intToStr(settings.AuditLogRetentionDays),
		"session_log_retention_days": intToStr(settings.SessionLogRetentionDays),
		"service_name":               settings.ServiceName,
		"site_icon":                  settings.SiteIcon,
		"corner_icon":                settings.CornerIcon,
	}
	for key := range seen {
		delete(defaults, key)
	}
	return defaults
}

func (r *settingsRepo) upsertSettings(ctx context.Context, pairs map[string]string) error {
	for k, v := range pairs {
		existing, err := r.data.WriteEnt.SystemSetting.Query().Where(entsystemsetting.Key(k)).Only(ctx)
		if ent.IsNotFound(err) {
			if _, err := r.data.WriteEnt.SystemSetting.Create().SetKey(k).SetValue(v).Save(ctx); err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			if err := existing.Update().SetValue(v).Exec(ctx); err != nil {
				return err
			}
		}
	}
	return nil
}

func countSScan(s string, dst *int32) (int, error) {
	var v int
	n, err := fmt.Sscan(s, &v)
	if err != nil {
		return n, err
	}
	*dst = int32(v)
	return n, nil
}

func intToStr(v int32) string {
	return fmt.Sprintf("%d", v)
}
