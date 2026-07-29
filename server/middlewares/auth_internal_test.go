package middlewares

import (
	"net/http"
	"net/http/httptest"
	"testing"

	gin "github.com/gin-gonic/gin"
	assert "github.com/stretchr/testify/assert"
	zap "go.uber.org/zap"
)

// TestOIDCMiddleware_Returns401WithChallenge covers the spec 7.4 SHOULD: 401 responses
// carry a WWW-Authenticate challenge. The no-header and malformed-header paths do not
// touch the token verifier, so they can be exercised without a live OIDC provider.
func TestOIDCMiddleware_Returns401WithChallenge(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name          string
		authHeader    string
		setHeader     bool
		wantChallenge string
	}{
		{
			name:          "missing header",
			setHeader:     false,
			wantChallenge: "Bearer",
		},
		{
			name:          "malformed header",
			setHeader:     true,
			authHeader:    "Token abc",
			wantChallenge: `Bearer error="invalid_request"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			auth := &OIDCAuthenticatorImpl{logger: zap.NewNop()}

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/a2a", nil)
			if tt.setHeader {
				c.Request.Header.Set("Authorization", tt.authHeader)
			}

			auth.Middleware()(c)

			assert.Equal(t, http.StatusUnauthorized, w.Code)
			assert.Equal(t, tt.wantChallenge, w.Header().Get("WWW-Authenticate"))
			assert.True(t, c.IsAborted())
		})
	}
}
