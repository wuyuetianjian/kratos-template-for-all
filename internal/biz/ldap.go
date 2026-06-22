package biz

import (
	"context"
	"crypto/tls"
	"fmt"
	"strings"

	goldap "github.com/go-ldap/ldap/v3"
)

// tryLDAPLogin attempts authentication against the first enabled LDAP provider.
// On success it upserts the user in the local DB (source="ldap") and returns it.
func (uc *UseCase) tryLDAPLogin(ctx context.Context, username, password string) (*User, error) {
	providers, err := uc.ssoRepo.ListSSOProviders(ctx, false)
	if err != nil {
		return nil, err
	}
	for _, p := range providers {
		if p.Type != string(SSOTypeLDAP) {
			continue
		}
		displayName, err := ldapAuthenticate(&p, username, password)
		if err != nil {
			continue
		}
		user, err := uc.authRepo.UpsertLDAPUser(ctx, username, displayName)
		if err != nil {
			return nil, err
		}
		return user, nil
	}
	return nil, fmt.Errorf("no ldap provider matched")
}

func ldapAuthenticate(p *SSOProvider, username, password string) (displayName string, err error) {
	cfg := p.Config
	host := cfg["host"]
	if host == "" {
		return "", fmt.Errorf("ldap host not configured")
	}
	port := cfg["port"]
	if port == "" {
		port = "389"
	}
	addr := host + ":" + port

	tlsMode := cfg["tls"]
	skipVerify := cfg["skip_tls_verify"] == "true"

	var conn *goldap.Conn
	switch tlsMode {
	case "tls":
		conn, err = goldap.DialTLS("tcp", addr, &tls.Config{InsecureSkipVerify: skipVerify}) //nolint:gosec
	default:
		conn, err = goldap.Dial("tcp", addr)
		if err == nil && tlsMode == "starttls" {
			err = conn.StartTLS(&tls.Config{InsecureSkipVerify: skipVerify}) //nolint:gosec
		}
	}
	if err != nil {
		return "", fmt.Errorf("ldap connect: %w", err)
	}
	defer conn.Close()

	bindDN := cfg["bind_dn"]
	bindPw := cfg["bind_password"]
	if err := conn.Bind(bindDN, bindPw); err != nil {
		return "", fmt.Errorf("ldap service bind: %w", err)
	}

	baseDN := cfg["base_dn"]
	usernameAttr := cfg["attr_username"]
	if usernameAttr == "" {
		usernameAttr = "uid"
	}
	displayAttr := cfg["attr_display_name"]
	if displayAttr == "" {
		displayAttr = "cn"
	}

	userFilter := cfg["user_filter"]
	if userFilter == "" {
		userFilter = fmt.Sprintf("(%s={username})", usernameAttr)
	}
	userFilter = strings.ReplaceAll(userFilter, "{username}", goldap.EscapeFilter(username))

	req := goldap.NewSearchRequest(
		baseDN,
		goldap.ScopeWholeSubtree,
		goldap.NeverDerefAliases,
		1, 0, false,
		userFilter,
		[]string{"dn", displayAttr},
		nil,
	)
	result, err := conn.Search(req)
	if err != nil || len(result.Entries) == 0 {
		return "", fmt.Errorf("ldap user not found")
	}
	entry := result.Entries[0]

	if err := conn.Bind(entry.DN, password); err != nil {
		return "", fmt.Errorf("ldap user bind failed: %w", err)
	}

	return entry.GetAttributeValue(displayAttr), nil
}
