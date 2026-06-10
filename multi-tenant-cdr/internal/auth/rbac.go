package auth

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Permission string

const (
	PermissionCDRRead      Permission = "cdr:read"
	PermissionCDRWrite     Permission = "cdr:write"
	PermissionCDRDelete    Permission = "cdr:delete"
	PermissionAnalytics    Permission = "analytics:view"
	PermissionReports      Permission = "reports:view"
	PermissionReportsCreate Permission = "reports:create"
	PermissionTenantRead   Permission = "tenant:read"
	PermissionTenantWrite  Permission = "tenant:write"
	PermissionUserRead     Permission = "user:read"
	PermissionUserWrite    Permission = "user:write"
)

type Role string

const (
	RoleSuperAdmin  Role = "super_admin"
	RoleTenantAdmin Role = "tenant_admin"
	RoleAnalyst    Role = "analyst"
	RoleBilling    Role = "billing"
	RoleSupport    Role = "support"
)

var rolePermissions = map[Role][]Permission{
	RoleSuperAdmin: {
		PermissionCDRRead, PermissionCDRWrite, PermissionCDRDelete,
		PermissionAnalytics, PermissionReports, PermissionReportsCreate,
		PermissionTenantRead, PermissionTenantWrite,
		PermissionUserRead, PermissionUserWrite,
	},
	RoleTenantAdmin: {
		PermissionCDRRead, PermissionCDRWrite,
		PermissionAnalytics, PermissionReports, PermissionReportsCreate,
		PermissionUserRead, PermissionUserWrite,
	},
	RoleAnalyst: {
		PermissionCDRRead,
		PermissionAnalytics,
		PermissionReports,
	},
	RoleBilling: {
		PermissionCDRRead,
		PermissionReports,
	},
	RoleSupport: {
		PermissionCDRRead,
		PermissionAnalytics,
	},
}

type RBACMiddleware struct {
	db     *sql.DB
	jwtMgr *JWTManager
}

func NewRBACMiddleware(db *sql.DB, jwtMgr *JWTManager) *RBACMiddleware {
	return &RBACMiddleware{
		db:     db,
		jwtMgr: jwtMgr,
	}
}

func (r *RBACMiddleware) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			c.Abort()
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header format"})
			c.Abort()
			return
		}

		tokenString := strings.TrimPrefix(authHeader, "Bearer ")
		claims, err := r.jwtMgr.ValidateToken(tokenString)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Store claims in context
		c.Set("user_id", claims.UserID)
		c.Set("tenant_id", claims.TenantID)
		c.Set("role", claims.Role)

		c.Next()
	}
}

func (r *RBACMiddleware) RequirePermission(permission Permission) gin.HandlerFunc {
	return func(c *gin.Context) {
		role := Role(c.GetString("role"))
		if role == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Role not found in context"})
			c.Abort()
			return
		}

		permissions, exists := rolePermissions[role]
		if !exists {
			c.JSON(http.StatusForbidden, gin.H{"error": "Invalid role"})
			c.Abort()
			return
		}

		hasPermission := false
		for _, p := range permissions {
			if p == permission {
				hasPermission = true
				break
			}
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, gin.H{"error": "Insufficient permissions"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (r *RBACMiddleware) RequireTenantAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID := c.GetString("tenant_id")
		if tenantID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Tenant ID not found in context"})
			c.Abort()
			return
		}

		// Check if tenant is active
		var status string
		err := r.db.QueryRowContext(c.Request.Context(), 
			"SELECT status FROM tenants WHERE id = $1", tenantID).Scan(&status)
		if err != nil {
			if err == sql.ErrNoRows {
				c.JSON(http.StatusNotFound, gin.H{"error": "Tenant not found"})
			} else {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			}
			c.Abort()
			return
		}

		if status != "active" {
			c.JSON(http.StatusForbidden, gin.H{"error": "Tenant is not active"})
			c.Abort()
			return
		}

		c.Next()
	}
}

func (r *RBACMiddleware) GetUserPermissions(ctx context.Context, userID, tenantID uuid.UUID) ([]Permission, error) {
	var role string
	var permissionsJSON string

	query := `
		SELECT role, permissions 
		FROM user_roles 
		WHERE user_id = $1 AND tenant_id = $2
	`

	err := r.db.QueryRowContext(ctx, query, userID, tenantID).Scan(&role, &permissionsJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}

	// Parse custom permissions if they exist
	var customPermissions []Permission
	if permissionsJSON != "" {
		if err := json.Unmarshal([]byte(permissionsJSON), &customPermissions); err != nil {
			return nil, err
		}
		return customPermissions, nil
	}

	// Return default role permissions
	return rolePermissions[Role(role)], nil
}

func (r *RBACMiddleware) HasPermission(ctx context.Context, userID, tenantID uuid.UUID, permission Permission) (bool, error) {
	permissions, err := r.GetUserPermissions(ctx, userID, tenantID)
	if err != nil {
		return false, err
	}

	for _, p := range permissions {
		if p == permission {
			return true, nil
		}
	}

	return false, nil
}

func (r *RBACMiddleware) AuditLog(c *gin.Context, statusCode int, responseTimeMs int64) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")
	requestID := c.GetString("request_id")

	if userID == "" {
		return
	}

	query := `
		INSERT INTO api_audit_log (
			tenant_id, user_id, request_id, method, path, 
			query_params, response_status, response_time_ms, 
			ip_address, user_agent
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := r.db.Exec(
		query,
		tenantID, userID, requestID, c.Request.Method, c.Request.URL.Path,
		c.Request.URL.RawQuery, statusCode, responseTimeMs,
		c.ClientIP(), c.Request.UserAgent(),
	)

	if err != nil {
		// Log error but don't fail the request
		return
	}
}
