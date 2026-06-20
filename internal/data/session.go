package data

import (
	"context"
	"fmt"
	"time"

	"temperate/internal/biz"
	"temperate/internal/data/ent"
	entauditlog "temperate/internal/data/ent/auditlog"
	entsystemsetting "temperate/internal/data/ent/systemsetting"
	entusersession "temperate/internal/data/ent/usersession"

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
	query := r.data.WriteEnt.UserSession.Query().WithUser()
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

func (r *auditLogRepo) ListAuditLogs(ctx context.Context, action string, page biz.Page) ([]biz.AuditLogEntry, int, error) {
	query := r.data.ReadEnt.AuditLog.Query()
	if action != "" {
		query = query.Where(entauditlog.Action(action))
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
	}
	rows, err := r.data.ReadEnt.SystemSetting.Query().All(ctx)
	if err != nil {
		return settings, nil
	}
	for _, row := range rows {
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
		}
	}
	return settings, nil
}

func (r *settingsRepo) UpdateSettings(ctx context.Context, settings *biz.SystemSettings) error {
	pairs := map[string]string{
		"audit_log_retention_days":   intToStr(settings.AuditLogRetentionDays),
		"session_log_retention_days": intToStr(settings.SessionLogRetentionDays),
	}
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
