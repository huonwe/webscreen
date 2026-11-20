package auth

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

var errUnauthorized = errors.New("未授权访问")

// RequireAdmin 保护鼠标键盘控制、生成分享链接等高危接口
func (s *Service) RequireAdmin() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, err := s.parseRoleFromCookie(c)
		if err != nil || role != "admin" {
			c.AbortWithStatusJSON(403, gin.H{"error": "需要管理员权限"})
			return
		}

		c.Set("role", role)
		c.Next()
	}
}

// RequireViewer 保护画面流，允许 admin 或有效的 viewer
func (s *Service) RequireViewer() gin.HandlerFunc {
	return func(c *gin.Context) {
		role, err := s.parseRoleFromCookie(c)
		if err != nil || (role != "admin" && role != "viewer") {
			c.AbortWithStatusJSON(401, gin.H{"error": "未授权访问画面"})
			return
		}

		c.Set("role", role)
		c.Next()
	}
}

func (s *Service) parseRoleFromCookie(c *gin.Context) (string, error) {
	// 管理员使用 auth_token Cookie 中的 JWT
	if tokenString, err := c.Cookie("auth_token"); err == nil && tokenString != "" {
		claims := &jwtClaims{}

		token, err := jwt.ParseWithClaims(
			tokenString,
			claims,
			func(token *jwt.Token) (any, error) {
				if token.Method != jwt.SigningMethodHS256 {
					return nil, errUnauthorized
				}
				return s.jwtSecret, nil
			},
			jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
			jwt.WithIssuer("webscreen"),
		)
		if err == nil && token.Valid {
			if claims.Role == "admin" || claims.Role == "viewer" {
				return claims.Role, nil
			}
			return "", errUnauthorized
		}
	}

	return "", errUnauthorized
	// 目前暂未实现分享连接，所以这里直接返回未授权

	// 分享链接使用 share_token Cookie；viewer 需经过数据库校验
	// shareToken, err := c.Cookie("share_token")
	// if err != nil || shareToken == "" {
	// 	return "", errUnauthorized
	// }

	// var exists int
	// err = s.db.QueryRow(`
	//     SELECT 1
	//     FROM share_links
	//     WHERE token = ?
	//       AND is_revoked = 0
	//       AND expires_at > ?
	//     LIMIT 1`,
	// 	shareToken,
	// 	time.Now(),
	// ).Scan(&exists)

	// if err != nil {
	// 	return "", errUnauthorized
	// }

	// return "viewer", nil
}
