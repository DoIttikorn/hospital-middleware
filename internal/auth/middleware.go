package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/DoIttikorn/hospital-middleware/internal/httpx"
)

const identityKey = "auth.identity"

// Middleware rejects requests without a valid "Authorization: Bearer <jwt>"
// header with 401, and otherwise stores the caller's Identity for
// FromContext.
func (m *Manager) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			unauthorized(c, "missing bearer token")
			return
		}
		id, err := m.Parse(token)
		if err != nil {
			unauthorized(c, "invalid or expired token")
			return
		}
		c.Set(identityKey, id)
		c.Next()
	}
}

// FromContext returns the caller's identity; ok is false on routes the
// middleware does not protect.
func FromContext(c *gin.Context) (Identity, bool) {
	v, ok := c.Get(identityKey)
	if !ok {
		return Identity{}, false
	}
	id, ok := v.(Identity)
	return id, ok
}

func bearerToken(header string) (string, bool) {
	scheme, token, found := strings.Cut(header, " ")
	token = strings.TrimSpace(token)
	if !found || !strings.EqualFold(scheme, "Bearer") || token == "" {
		return "", false
	}
	return token, true
}

func unauthorized(c *gin.Context, detail string) {
	c.Header("WWW-Authenticate", "Bearer")
	httpx.WriteProblemCode(c.Writer, http.StatusUnauthorized, "unauthorized", detail)
	c.Abort()
}
