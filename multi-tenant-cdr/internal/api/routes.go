package api

import (
	"github.com/gin-gonic/gin"
	"github.com/nutcas3/multi-tenant-cdr/internal/auth"
)

func SetupRoutes(r *gin.Engine, handler *Handler, rbac *auth.RBACMiddleware) {
	api := r.Group("/api/v1")
	{
		// CDR endpoints
		cdr := api.Group("/cdrs")
		cdr.Use(rbac.Authenticate())
		cdr.Use(rbac.RequireTenantAccess())
		{
			cdr.GET("", rbac.RequirePermission(auth.PermissionCDRRead), handler.GetCDRs)
			cdr.GET("/:id", rbac.RequirePermission(auth.PermissionCDRRead), handler.GetCDR)
		}

		// Analytics endpoints
		analytics := api.Group("/analytics")
		analytics.Use(rbac.Authenticate())
		analytics.Use(rbac.RequireTenantAccess())
		{
			analytics.GET("", rbac.RequirePermission(auth.PermissionAnalytics), handler.GetAnalytics)
			analytics.GET("/patterns", rbac.RequirePermission(auth.PermissionAnalytics), handler.GetCallPatterns)
			analytics.GET("/quality", rbac.RequirePermission(auth.PermissionAnalytics), handler.GetQualityMetrics)
			analytics.GET("/trends", rbac.RequirePermission(auth.PermissionAnalytics), handler.GetTrendAnalysis)
		}

		// Statistics endpoint
		api.GET("/statistics", rbac.Authenticate(), rbac.RequireTenantAccess(), rbac.RequirePermission(auth.PermissionAnalytics), handler.GetStatistics)
	}
}
