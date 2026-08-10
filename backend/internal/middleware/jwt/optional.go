package jwt

import (
	"strings"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

// OptionalJWTAuth 用于匿名可访问、但登录后能看到更多内容的接口。
//
// 与 JWTAuthWithTokenCache 的区别：没有 token 或 token 无效时不拦截，
// 只是不设置身份，请求以匿名方式继续。
//
// 典型用途是视频详情与作者作品列表——作者本人需要能看到自己
// 尚未过审的内容，其他人则只应看到已过审的。
//
// 这里刻意不查 token 缓存做吊销校验：本中间件的结论只用于
// 「是不是作者本人」这类可见性判断，读到的也只是用户自己的内容，
// 为此多打一次 Redis 不划算。所有写操作仍走严格鉴权。
func OptionalJWTAuth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.Next()
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
			c.Next()
			return
		}

		claims, ok := token.Claims.(jwtv5.MapClaims)
		if !ok {
			c.Next()
			return
		}
		accountIDValue, ok := claims["account_id"].(float64)
		if !ok {
			c.Next()
			return
		}

		username, _ := claims["username"].(string)
		c.Set("account_id", uint64(accountIDValue))
		c.Set("account_username", username)
		c.Next()
	}
}
