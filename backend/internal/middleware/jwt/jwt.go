package jwt

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"

	"my_feed_system/internal/account"
	"my_feed_system/internal/response"
)

const OpsCookieName = "feed_ops"

func JWTAuth(db *gorm.DB, secret string) gin.HandlerFunc {
	return JWTAuthWithTokenCache(db, nil, secret)
}

func BearerToken(c *gin.Context) string {
	authHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(authHeader, "Bearer ")
}

func CookieToken(c *gin.Context, name string) string {
	value, err := c.Cookie(name)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

// JWTAuthWithTokenCache 优先从 Redis 校验 token，未命中时再回源 MySQL。
func JWTAuthWithTokenCache(db *gorm.DB, tokenCache *account.TokenCache, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		tokenString := BearerToken(c)
		if tokenString == "" {
			response.Abort(c, http.StatusUnauthorized, response.LoginRequired, nil)
			return
		}
		if !BindToken(c, db, tokenCache, secret, tokenString) {
			return
		}
		c.Next()
	}
}

// BindToken 校验 JWT 并写入账号上下文。失败时已经中止请求。
func BindToken(c *gin.Context, db *gorm.DB, tokenCache *account.TokenCache, secret string, tokenString string) bool {
	token, err := jwtv5.Parse(tokenString, func(token *jwtv5.Token) (interface{}, error) {
		if token.Method != jwtv5.SigningMethodHS256 {
			return nil, jwtv5.ErrTokenSignatureInvalid
		}
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		response.Abort(c, http.StatusUnauthorized, response.LoginExpired, err)
		return false
	}

	claims, ok := token.Claims.(jwtv5.MapClaims)
	if !ok {
		response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
		return false
	}

	accountIDValue, ok := claims["account_id"].(float64)
	if !ok {
		response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
		return false
	}

	accountID := uint64(accountIDValue)
	username, _ := claims["username"].(string)
	repo := account.NewRepo(db)

	if tokenCache != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		cachedToken, ok, err := tokenCache.Get(ctx, accountID)
		cancel()
		if err != nil {
			// 缓存不可用时降级回源 MySQL，不影响本次鉴权结果，故记 warn 而非 error。
			slog.WarnContext(c.Request.Context(), "read token cache failed, falling back to database",
				slog.Uint64("account_id", accountID), slog.String("error", err.Error()))
		} else if ok {
			if cachedToken != tokenString {
				response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
				return false
			}

			if username == "" {
				currentAccount, err := repo.FindByID(accountID)
				if err != nil || currentAccount == nil {
					response.Abort(c, http.StatusUnauthorized, response.AccountNotFound, err)
					return false
				}
				username = currentAccount.Username
			}

			c.Set("account_id", accountID)
			c.Set("account_username", username)
			return true
		}
	}

	currentAccount, err := repo.FindByID(accountID)
	if err != nil || currentAccount == nil {
		response.Abort(c, http.StatusUnauthorized, response.AccountNotFound, err)
		return false
	}

	if currentAccount.Token != tokenString {
		response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
		return false
	}

	if tokenCache != nil {
		ctx, cancel := context.WithTimeout(c.Request.Context(), time.Second)
		if err := tokenCache.Set(ctx, accountID, tokenString); err != nil {
			slog.WarnContext(c.Request.Context(), "refill token cache failed",
				slog.Uint64("account_id", accountID), slog.String("error", err.Error()))
		}
		cancel()
	}

	c.Set("account_id", currentAccount.ID)
	c.Set("account_username", currentAccount.Username)
	return true
}
