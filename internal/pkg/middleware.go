package pkg

import (
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"net/http"
	"strings"
)

func JWTAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["role"] != "admin" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		c.Next()
	}
}

func CompanyAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["role"] != "company" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Access denied"})
			return
		}

		companyID, err := uuid.Parse(claims["sub"].(string))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid company ID"})
			return
		}

		c.Set("companyID", companyID.String())

		if resetRequired, exists := claims["reset_required"]; exists && resetRequired.(bool) {
			if c.Request.URL.Path != "/api/v1/company/change-password" {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "Password reset required",
					"code":  "PASSWORD_RESET_REQUIRED",
				})
				return
			}
		}

		c.Next()
	}
}

func TeamAuthMiddleware(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Требуется авторизация"})
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		})

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Неверный токен"})
			return
		}

		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok || claims["role"] != "team" {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Доступ запрещен"})
			return
		}

		teamID, err := uuid.Parse(claims["sub"].(string))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Неверный ID команды"})
			return
		}

		c.Set("teamID", teamID.String())
		c.Next()
	}
}
