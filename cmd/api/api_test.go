package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"security-service/internal/audit"
	"security-service/internal/auth"
	"security-service/internal/middleware"
	"security-service/internal/rbac"
	"security-service/internal/security"
	"security-service/internal/store"
	"security-service/internal/user"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// setupRouter builds a full Gin router backed by in-memory SQLite + miniredis.
func setupRouter(t *testing.T) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)

	// ---- in-memory Redis ----
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})

	// ---- in-memory SQLite ----
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, db.AutoMigrate(
		&user.User{},
		&rbac.Role{},
		&rbac.Permission{},
		&rbac.UserRole{},
		&audit.Log{},
	))
	rbac.Seed(db)

	// ---- services ----
	jwtManager := security.NewJWTManager()
	blacklist := store.NewTokenBlacklist(rdb)
	userRepo := user.NewRepository(db)
	authService := auth.NewService(userRepo, jwtManager, blacklist)
	rbacService := rbac.NewService(db, rdb)

	authHandler := auth.NewHandler(authService)

	// ---- router ----
	r := gin.New()
	api := r.Group("/api/v1")

	// public auth
	authGroup := api.Group("/auth")
	authGroup.POST("/register", authHandler.Register)
	authGroup.POST("/login",
		middleware.RateLimitByIP(rdb, 5, time.Minute),
		authHandler.Login)
	authGroup.POST("/logout", authHandler.Logout)

	// protected
	protected := api.Group("")
	protected.Use(middleware.JWTAuth(jwtManager, blacklist))

	protected.GET("/me", func(c *gin.Context) {
		c.JSON(200, gin.H{"user_id": c.GetString("user_id")})
	})
	protected.GET("/admin-only",
		middleware.RequirePermission(rbacService, "user:create"),
		func(c *gin.Context) {
			c.JSON(200, gin.H{"ok": true})
		})

	return r
}

// --------------- helpers ---------------

func postJSON(r *gin.Engine, path, body string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

func getWithToken(r *gin.Engine, path, token string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(w, req)
	return w
}

type tokenBody struct {
	Data struct {
		AccessToken string `json:"access_token"`
	} `json:"data"`
}

func registerAndLogin(t *testing.T, r *gin.Engine, username, email, password string) string {
	t.Helper()
	body := `{"username":"` + username + `","email":"` + email + `","password":"` + password + `"}`
	w := postJSON(r, "/api/v1/auth/register", body)
	require.Equal(t, 201, w.Code, "register should succeed")

	loginBody := `{"email":"` + email + `","password":"` + password + `"}`
	w = postJSON(r, "/api/v1/auth/login", loginBody)
	require.Equal(t, 200, w.Code, "login should succeed")

	var resp tokenBody
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp.Data.AccessToken)
	return resp.Data.AccessToken
}

// --------------- test cases ---------------

func TestRegisterDuplicateUsername_Returns400(t *testing.T) {
	r := setupRouter(t)

	body1 := `{"username":"dupuser","email":"a@example.com","password":"Test1234"}`
	w := postJSON(r, "/api/v1/auth/register", body1)
	assert.Equal(t, 201, w.Code)

	// same username, different email
	body2 := `{"username":"dupuser","email":"b@example.com","password":"Test1234"}`
	w = postJSON(r, "/api/v1/auth/register", body2)
	assert.Equal(t, 400, w.Code)
	assert.Contains(t, w.Body.String(), "username already taken")
}

func TestLogoutInvalidatesToken(t *testing.T) {
	r := setupRouter(t)
	token := registerAndLogin(t, r, "logoutuser", "lo@example.com", "Test1234")

	// token works before logout
	w := getWithToken(r, "/api/v1/me", token)
	assert.Equal(t, 200, w.Code)

	// logout
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	r.ServeHTTP(rec, req)
	assert.Equal(t, 200, rec.Code)

	// same token should now be rejected
	w = getWithToken(r, "/api/v1/me", token)
	assert.Equal(t, 401, w.Code)
	assert.Contains(t, w.Body.String(), "revoked")
}

func TestNoPermission_Returns403(t *testing.T) {
	r := setupRouter(t)
	// regular user — no roles in user_roles table → no permissions
	token := registerAndLogin(t, r, "normie", "normie@example.com", "Test1234")

	w := getWithToken(r, "/api/v1/admin-only", token)
	assert.Equal(t, 403, w.Code)
	assert.Contains(t, w.Body.String(), "access denied")
}

func TestLoginRateLimit_Returns429(t *testing.T) {
	r := setupRouter(t)

	// register a user so the endpoint exists meaningfully
	regBody := `{"username":"rateuser","email":"rate@example.com","password":"Test1234"}`
	postJSON(r, "/api/v1/auth/register", regBody)

	loginBody := `{"email":"rate@example.com","password":"Wrong1234"}`

	// first 5 attempts allowed (will get 401 — wrong password, but not 429)
	for i := 0; i < 5; i++ {
		w := postJSON(r, "/api/v1/auth/login", loginBody)
		assert.NotEqual(t, 429, w.Code, "attempt %d should not be rate-limited", i+1)
	}

	// 6th attempt must be rate-limited
	w := postJSON(r, "/api/v1/auth/login", loginBody)
	assert.Equal(t, 429, w.Code)
}
