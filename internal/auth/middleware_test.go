package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func protectedRouter(m *Manager) *gin.Engine {
	r := gin.New()
	r.GET("/me", m.Middleware(), func(c *gin.Context) {
		id, ok := FromContext(c)
		if !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, gin.H{"staff": id.StaffID, "hospital": id.HospitalID})
	})
	return r
}

func TestMiddleware(t *testing.T) {
	m := newManager(t)
	good, _ := m.Issue("staff-1", "hosp-1")
	expired := newManager(t)
	expired.now = func() time.Time { return time.Now().Add(-2 * time.Hour) }
	expiredTok, _ := expired.Issue("staff-1", "hosp-1")

	tests := []struct {
		name, header string
		want         int
	}{
		{"valid", "Bearer " + good, http.StatusOK},
		{"scheme is case-insensitive", "bearer " + good, http.StatusOK},
		{"no header", "", http.StatusUnauthorized},
		{"wrong scheme", "Basic " + good, http.StatusUnauthorized},
		{"no token", "Bearer ", http.StatusUnauthorized},
		{"token without scheme", good, http.StatusUnauthorized},
		{"garbage", "Bearer garbage", http.StatusUnauthorized},
		{"expired", "Bearer " + expiredTok, http.StatusUnauthorized},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/me", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			protectedRouter(m).ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d: %s", rec.Code, tt.want, rec.Body)
			}
			if tt.want == http.StatusUnauthorized && rec.Header().Get("WWW-Authenticate") != "Bearer" {
				t.Errorf("WWW-Authenticate = %q", rec.Header().Get("WWW-Authenticate"))
			}
		})
	}
}

func TestFromContextWithoutMiddleware(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	if _, ok := FromContext(c); ok {
		t.Error("FromContext ok on an unprotected route")
	}
}
