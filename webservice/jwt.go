package webservice

import (
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

type CustomClaims struct {
	Role        string `json:"role"`
	LoginMethod string `json:"login_method,omitempty"`
	Username    string `json:"username,omitempty"`
	jwt.RegisteredClaims
}

func (wm *WebMaster) GenerateToken() (string, error) {
	// 设置 Token 有效期
	expirationTime := time.Now().Add(2 * time.Hour)

	claims := &CustomClaims{
		Role:        "admin",
		Username:    "PIN",
		LoginMethod: "pin",
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "webscreen",
		},
	}

	// 使用 HS256 算法进行签名
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(wm.jwtSecret)
	return tokenString, err
}

func (wm *WebMaster) parseToken(tokenString string) (*CustomClaims, error) {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return wm.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("webscreen"))

	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid token: %v", err)
	}

	return claims, nil
}

// ---------------------------------------------------------
// 中间件：JWT 认证拦截器
// ---------------------------------------------------------
func (wm *WebMaster) HybridAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if wm.isPublicAuthPath(c.Request.URL.Path) {
			c.Next()
			return
		}

		var tokenString string

		// 1. 优先尝试从 Cookie 取
		tokenString, _ = c.Cookie("auth_token")

		// 2. 如果 Cookie 没有，尝试从 Header 取
		if tokenString == "" {
			authHeader := c.GetHeader("Authorization")
			if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
				tokenString = authHeader[7:]
			}
		}

		// 3. 都没有，或者验证失败
		if tokenString == "" || !wm.validateToken(tokenString) {
			accept := c.GetHeader("Accept")
			if strings.Contains(accept, "text/html") {
				if wm.pin != 0 && wm.isTrustedIP(c.ClientIP()) {
					c.Redirect(http.StatusFound, "/unlock")
				} else {
					c.Redirect(http.StatusFound, "/login")
				}
			} else {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
			}
			return
		}
		claims, err := wm.parseToken(tokenString)
		if err != nil {
			// 未授权处理
			return
		}

		c.Set("userid", claims.ID)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)
		c.Set("login_method", claims.LoginMethod)

		c.Next()
	}
}
func (wm *WebMaster) validateToken(tokenString string) bool {
	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return wm.jwtSecret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithIssuer("webscreen"))

	if err != nil || !token.Valid {
		return false
	}

	return claims.Role == "admin"
}

func (wm *WebMaster) isPublicAuthPath(path string) bool {
	switch path {
	case "/unlock", "/api/unlock", "/login", "/api/login", "/setup", "/api/setup":
		return true
	}
	return strings.HasPrefix(path, "/static/")
}

func (wm *WebMaster) isTrustedIP(ip string) bool {
	host := net.ParseIP(ip)
	if host == nil {
		return false
	}
	for _, source := range wm.trustedIPs {
		source = strings.TrimSpace(source)
		if source == "" {
			continue
		}
		if _, network, err := net.ParseCIDR(source); err == nil {
			if network.Contains(host) {
				return true
			}
			continue
		}
		if trusted := net.ParseIP(source); trusted != nil && trusted.Equal(host) {
			return true
		}
	}
	return false
}
