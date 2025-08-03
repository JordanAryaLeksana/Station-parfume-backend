package authmiddlewares

import (
	httperror "backend/src/middlewares/Error"
	"backend/src/utils/jwt"
	utils "backend/src/utils/redisUtils"
	"strings"

	"github.com/gin-gonic/gin"
	jwtv5 "github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(secretKey string) gin.HandlerFunc{
	return func(c *gin.Context) {
		authorizationHeader := c.GetHeader("Authorization");
		if(authorizationHeader == ""){
		  httperror.UnauthorizedError(c, "Authorization header is missing") 
		  return 
		}

		parts := strings.Split(authorizationHeader, " ");
		if(len(parts) != 2 || parts[0] != "Bearer"){
			httperror.UnauthorizedError(c, "Invalid authorization header format")
			return
		}
		tokenStr := parts[1]
		token, err := jwtv5.ParseWithClaims(tokenStr, &jwt.JwtCustomClaims{}, func(token *jwtv5.Token) (interface{}, error) {
			if _, ok := token.Method.(*jwtv5.SigningMethodHMAC); !ok {
				httperror.UnauthorizedError(c, "Unexpected signing method")
				return nil, jwtv5.ErrSignatureInvalid
			}
			return []byte(secretKey), nil
		})
		if (err != nil || !token.Valid) {
			httperror.UnauthorizedError(c, "Invalid token")
		}
		claims, ok := token.Claims.(*jwt.JwtCustomClaims)
		if !ok {
			httperror.UnauthorizedError(c, "Invalid token claims")
			return
		}

		blacklisted,err := utils.IsTokenBlacklisted(tokenStr);
		if err != nil {
			httperror.InternalServerError(c, "Error checking token blacklist status")
			return
		}
		if blacklisted {
			httperror.UnauthorizedError(c, "Token is blacklisted")
			return
		}
		c.Set("userID", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("role", claims.Role)
		c.Next()	
	}
}