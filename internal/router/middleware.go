package router

import (
	"net/http"
	"strings"

	"gaia/internal/tenant"

	"github.com/gin-gonic/gin"
)

// TenantAuth valida el token Bearer y el slug de la URL
func TenantAuth(store *tenant.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Obtener slug de la URL (/v1/:tenant/...)
		urlTenant := c.Param("tenant")
		if urlTenant == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "tenant_missing",
				"message": "Falta el tenant en la URL",
			})
			c.Abort()
			return
		}

		// 2. Obtener el Token del Header
		authHeader := c.GetHeader("Authorization")
		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Falta el token Bearer en el header Authorization",
			})
			c.Abort()
			return
		}
		token := strings.TrimPrefix(authHeader, "Bearer ")

		// 3. Hashear el token y buscarlo en la memoria (Store)
		hash := tenant.HashToken(token)
		t, ok := store.GetByTokenHash(hash)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "Token inválido o no existe",
			})
			c.Abort()
			return
		}

		// 4. Verificar que el token pertenezca al tenant de la URL
		// (Para evitar que un cliente use su token en la URL de otro cliente)
		if t.Slug != urlTenant {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "El token no pertenece a este tenant",
			})
			c.Abort()
			return
		}

		// 5. Verificar que el tenant esté activo
		if !t.IsActive() {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   "forbidden",
				"message": "El tenant está inactivo",
			})
			c.Abort()
			return
		}

		// 6. Inyectar el tenant en el contexto para que los handlers lo usen
		// (Ej: en el handler de analyze, lo sacamos con c.MustGet("tenant"))
		c.Set("tenant", t)
		
		// 7. Dejar pasar la petición al siguiente handler
		c.Next()
	}
}