package web

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"strconv"
	"strings"
	"time"

	"myblog/internal/model"
	"myblog/internal/service"
	"myblog/internal/util"

	"github.com/gin-gonic/gin"
)

const (
	sessionCookie  = "BLOG_SESSION"
	userContextKey = "login_user"
	sessionMaxAge  = 12 * time.Hour
	rememberMaxAge = 30 * 24 * time.Hour
)

// SessionManager uses a signed, stateless cookie. It avoids a shared session
// store on the request hot path and works across multiple application replicas.
type SessionManager struct {
	service *service.Service
	key     []byte
}

func NewSessionManager(service *service.Service, secret string) *SessionManager {
	sum := sha256.Sum256([]byte(secret))
	return &SessionManager{service: service, key: sum[:]}
}

func (manager *SessionManager) Load() gin.HandlerFunc {
	return func(context *gin.Context) {
		user := manager.userFromSession(context.Request)
		if user != nil {
			context.Set(userContextKey, user)
		}
		context.Next()
	}
}

func (manager *SessionManager) User(context *gin.Context) *model.User {
	value, exists := context.Get(userContextKey)
	if !exists {
		return nil
	}
	user, _ := value.(*model.User)
	return user
}

func (manager *SessionManager) RequireAdmin() gin.HandlerFunc {
	return func(context *gin.Context) {
		if manager.User(context) == nil {
			context.Redirect(http.StatusFound, "/admin/login")
			context.Abort()
			return
		}
		context.Next()
	}
}

func (manager *SessionManager) Login(context *gin.Context, user *model.User, remember bool) {
	maxAge := sessionMaxAge
	if remember {
		maxAge = rememberMaxAge
	}
	expiry := time.Now().Add(maxAge)
	payload := strconv.Itoa(user.Uid) + "|" + strconv.FormatInt(expiry.Unix(), 10)
	value := base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + manager.sign(payload)))
	setCookie(context, sessionCookie, value, int(maxAge.Seconds()), true)
	context.Set(userContextKey, user)
}

func (manager *SessionManager) Logout(context *gin.Context) {
	setCookie(context, sessionCookie, "", -1, true)
	setCookie(context, model.UserInCookie, "", -1, true)
}

func (manager *SessionManager) NewCSRFToken(path string) string {
	expiry := strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10)
	payload := path + "|" + expiry + "|" + util.Token()
	return base64.RawURLEncoding.EncodeToString([]byte(payload + "|" + manager.sign(payload)))
}

func (manager *SessionManager) ValidateCSRFToken(token string) bool {
	raw, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return false
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 4 {
		return false
	}
	payload := strings.Join(parts[:3], "|")
	if !hmac.Equal([]byte(parts[3]), []byte(manager.sign(payload))) {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && expiry > time.Now().Unix()
}

func (manager *SessionManager) userFromSession(request *http.Request) *model.User {
	cookie, err := request.Cookie(sessionCookie)
	if err != nil {
		return nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(cookie.Value)
	if err != nil {
		return nil
	}
	parts := strings.Split(string(raw), "|")
	if len(parts) != 3 {
		return nil
	}
	payload := parts[0] + "|" + parts[1]
	if !hmac.Equal([]byte(parts[2]), []byte(manager.sign(payload))) {
		return nil
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || expiry <= time.Now().Unix() {
		return nil
	}
	uid, err := strconv.Atoi(parts[0])
	if err != nil {
		return nil
	}
	return manager.service.QueryUserByID(uid)
}

func (manager *SessionManager) sign(payload string) string {
	mac := hmac.New(sha256.New, manager.key)
	_, _ = mac.Write([]byte(payload))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func setCookie(context *gin.Context, name, value string, maxAge int, httpOnly bool) {
	http.SetCookie(context.Writer, &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: httpOnly,
		Secure:   context.Request.TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
