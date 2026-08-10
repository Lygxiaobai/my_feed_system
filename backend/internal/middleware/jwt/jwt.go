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

func JWTAuth(db *gorm.DB, secret string) gin.HandlerFunc {
	return JWTAuthWithTokenCache(db, nil, secret)
}

// JWTAuthWithTokenCache 优先从 Redis 校验 token，未命中时再回源 MySQL。
func JWTAuthWithTokenCache(db *gorm.DB, tokenCache *account.TokenCache, secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			response.Abort(c, http.StatusUnauthorized, response.LoginRequired, nil)
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwtv5.Parse(tokenString, func(token *jwtv5.Token) (interface{}, error) {
			if token.Method != jwtv5.SigningMethodHS256 {
				return nil, jwtv5.ErrTokenSignatureInvalid
			}
			return []byte(secret), nil
		})
		if err != nil || !token.Valid {
			response.Abort(c, http.StatusUnauthorized, response.LoginExpired, err)
			return
		}

		claims, ok := token.Claims.(jwtv5.MapClaims)
		if !ok {
			response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
			return
		}

		accountIDValue, ok := claims["account_id"].(float64)
		if !ok {
			response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
			return
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
					return
				}

				if username == "" {
					currentAccount, err := repo.FindByID(accountID)
					if err != nil || currentAccount == nil {
						response.Abort(c, http.StatusUnauthorized, response.AccountNotFound, err)
						return
					}
					username = currentAccount.Username
				}

				c.Set("account_id", accountID)
				c.Set("account_username", username)
				c.Next()
				return
			}
		}

		currentAccount, err := repo.FindByID(accountID)
		if err != nil || currentAccount == nil {
			response.Abort(c, http.StatusUnauthorized, response.AccountNotFound, err)
			return
		}

		if currentAccount.Token != tokenString {
			response.Abort(c, http.StatusUnauthorized, response.LoginExpired, nil)
			return
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
		c.Next()
	}
}
