package auth

import (
	"errors"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const cookieToken = "token"
const key = "jkdfhbgjkhdbgtuydfbnvfujbynytfubn458976y4ghjui"

var ErrInvalidToken = errors.New("bad token")

type Claims struct {
	UserID string `json:"user_id"`
	jwt.RegisteredClaims
}

func genToken(userID string) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(key))
}

func parseToken(tokenString string) (string, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return []byte(key), nil
	})

	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}

	if claims.UserID == "" {
		return "", ErrInvalidToken
	}

	return claims.UserID, nil
}

func GetOrCreateUserID(w http.ResponseWriter, r *http.Request) (string, error) {
	cookie, err := r.Cookie(cookieToken)
	if err == nil {
		userID, err := parseToken(cookie.Value)
		if err == nil {
			return userID, nil
		}
	}

	userID := uuid.New().String()
	token, err := genToken(userID)
	if err != nil {
		return "", err
	}

	http.SetCookie(w, &http.Cookie{
		Name:     cookieToken,
		Value:    token,
		HttpOnly: true,
		Path:     "/",
		Expires:  time.Now().AddDate(100, 0, 0),
	})

	return userID, nil
}

func GetUserID(r *http.Request) string {
	cookie, err := r.Cookie(cookieToken)
	if err != nil {
		return ""
	}

	userID, err := parseToken(cookie.Value)
	if err != nil {
		return ""
	}

	return userID
}

func NewToken() (string, string, error) {
	userID := uuid.New().String()
	token, err := genToken(userID)
	if err != nil {
		return "", "", err
	}
	return userID, token, nil
}

func ParseToken(token string) (string, error) {
	return parseToken(token)
}
