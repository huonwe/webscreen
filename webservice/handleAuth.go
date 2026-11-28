package webservice

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func (wm *WebMaster) handleAuthStatus(c *gin.Context) {
	tokenString, err := c.Cookie("auth_token")
	if err != nil || tokenString == "" {
		authHeader := c.GetHeader("Authorization")
		if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
			tokenString = authHeader[7:]
		}
	}
	if tokenString == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "未登录"})
		return
	}

	claims := &CustomClaims{}
	token, err := jwt.ParseWithClaims(
		tokenString,
		claims,
		func(token *jwt.Token) (interface{}, error) {
			if token.Method != jwt.SigningMethodHS256 {
				return nil, jwt.ErrTokenMalformed
			}
			return wm.jwtSecret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer("scrmux"),
	)
	if err != nil || !token.Valid || claims.Role == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "登录已失效"})
		return
	}

	username := claims.Username
	if username == "" {
		username = "未知用户"
	}
	loginMethod := claims.LoginMethod
	if loginMethod == "" {
		loginMethod = "unknown"
	}

	c.JSON(http.StatusOK, gin.H{
		"username":     username,
		"role":         claims.Role,
		"login_method": loginMethod,
	})
}
