package ctx

import (
	"context"
)

type key int

const (
	userIDKey key = iota
	usernameKey
	roleKey
	siteIDKey
	ipKey
	userAgentKey
)

func WithUserID(c context.Context, v string) context.Context {
	return context.WithValue(c, userIDKey, v)
}

func WithUsername(c context.Context, v string) context.Context {
	return context.WithValue(c, usernameKey, v)
}

func WithRole(c context.Context, v string) context.Context {
	return context.WithValue(c, roleKey, v)
}

// WithSiteID guarda la sede asignada al usuario (vacío = todas las sedes).
func WithSiteID(c context.Context, v string) context.Context {
	return context.WithValue(c, siteIDKey, v)
}

// WithIP guarda la IP del cliente para auditoría.
func WithIP(c context.Context, v string) context.Context {
	return context.WithValue(c, ipKey, v)
}

// WithUserAgent guarda el User-Agent del cliente para auditoría.
func WithUserAgent(c context.Context, v string) context.Context {
	return context.WithValue(c, userAgentKey, v)
}

func UserIDFromCtx(c context.Context) (string, bool) {
	v, ok := c.Value(userIDKey).(string)
	return v, ok
}

func UsernameFromCtx(c context.Context) (string, bool) {
	v, ok := c.Value(usernameKey).(string)
	return v, ok
}

func RoleFromCtx(c context.Context) (string, bool) {
	v, ok := c.Value(roleKey).(string)
	return v, ok
}

func SiteIDFromCtx(c context.Context) (string, bool) {
	v, ok := c.Value(siteIDKey).(string)
	return v, ok
}

func IPFromCtx(c context.Context) (string, bool) {
	v, ok := c.Value(ipKey).(string)
	return v, ok
}

func UserAgentFromCtx(c context.Context) (string, bool) {
	v, ok := c.Value(userAgentKey).(string)
	return v, ok
}
