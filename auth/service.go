package auth

import (
	"errors"
	"scrmux/consts"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	jwtSecret []byte
}

type jwtClaims struct {
	ID          int    `json:"id"`
	Role        string `json:"role"`
	LoginMethod string `json:"login_method,omitempty"`
	Username    string `json:"username,omitempty"`
	jwt.RegisteredClaims
}

var ErrAlreadyInitialized = errors.New("System already initialized")

func NewService(secret string) *Service {
	return &Service{jwtSecret: []byte(secret)}
}

// HasAdmin 检查系统是否已完成首次初始化
func (s *Service) HasAdmin() (bool, error) {
	var count int
	err := consts.DB.QueryRow("SELECT COUNT(*) FROM users WHERE role = 'admin'").Scan(&count)
	return count > 0, err
}

// SetupAdmin 首次安装时创建唯一管理员
func (s *Service) SetupAdmin(username, password string) error {
	exists, err := s.HasAdmin()
	if err != nil {
		return err
	}
	if exists {
		return ErrAlreadyInitialized
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = consts.DB.Exec(
		"INSERT INTO users (username, password_hash, role) VALUES (?, ?, 'admin')",
		username,
		string(hash),
	)
	return err
}

// Login 验证密码并签发 Admin JWT
func (s *Service) Login(username, password string) (string, error) {
	var hash string
	var id int
	err := consts.DB.QueryRow("SELECT password_hash, id FROM users WHERE username = ?", username).Scan(&hash, &id)
	if err != nil {
		return "", errors.New("Invalid username or password")
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		return "", errors.New("Invalid username or password")
	}
	return s.signJWT(id, "admin", username, "password", 7*24*time.Hour)
}

// CreateShareLink 生成临时观看链接 Token
// func (s *Service) CreateShareLink(duration time.Duration) (string, error) {
// 	b := make([]byte, 16)
// 	rand.Read(b)
// 	token := hex.EncodeToString(b)
// 	expiresAt := time.Now().Add(duration)
// 	_, err := s.db.Exec("INSERT INTO share_links (token, expires_at) VALUES (?, ?)", token, expiresAt)
// 	return token, err
// }

func (s *Service) signJWT(ID int, role, username, loginMethod string, duration time.Duration) (string, error) {
	now := time.Now()

	claims := jwtClaims{
		ID:          ID,
		Role:        role,
		Username:    username,
		LoginMethod: loginMethod,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "scrmux",
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(duration)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.jwtSecret)
}
