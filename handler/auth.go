package handler

import (
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"hackathon-backend/db"
	mw "hackathon-backend/middleware"
	"hackathon-backend/model"
)

type registerRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Username string `json:"username"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// Register は新規ユーザーを登録しJWTを返す
func Register(c echo.Context) error {
	var req registerRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエスト形式が不正です")
	}
	if req.Email == "" || req.Password == "" || req.Username == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "必須項目を入力してください")
	}

	// パスワードをbcryptでハッシュ化
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "パスワード処理エラー")
	}

	user := model.User{
		Email:        req.Email,
		PasswordHash: string(hash),
		Username:     req.Username,
	}
	if result := db.DB.Create(&user); result.Error != nil {
		return echo.NewHTTPError(http.StatusConflict, "メールアドレスまたはユーザー名が既に使用されています")
	}

	token, err := generateJWT(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "トークン生成エラー")
	}

	return c.JSON(http.StatusCreated, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Login はメール・パスワードを検証しJWTを返す
func Login(c echo.Context) error {
	var req loginRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "リクエスト形式が不正です")
	}

	var user model.User
	if result := db.DB.Where("email = ?", req.Email).First(&user); result.Error != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "メールアドレスまたはパスワードが正しくありません")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "メールアドレスまたはパスワードが正しくありません")
	}

	token, err := generateJWT(user.ID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "トークン生成エラー")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"token": token,
		"user":  user,
	})
}

// Me はログイン中のユーザー情報を返す
func Me(c echo.Context) error {
	userID := c.Get("user_id").(uint)
	var user model.User
	if result := db.DB.First(&user, userID); result.Error != nil {
		return echo.NewHTTPError(http.StatusNotFound, "ユーザーが見つかりません")
	}
	return c.JSON(http.StatusOK, user)
}

// generateJWT はユーザーIDを含む署名済みJWTを生成する
func generateJWT(userID uint) (string, error) {
	claims := mw.JWTClaims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(os.Getenv("JWT_SECRET")))
}
