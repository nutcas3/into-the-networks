package main

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

var logger = logrus.New()

func init() {
	logger.SetFormatter(&logrus.JSONFormatter{})
}

// Dispatcher handlers
func addDestinationHandler(kamailio *KamailioClient, db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SetID       int    `json:"setid" binding:"required"`
			Destination string `json:"destination" binding:"required"`
			Priority    int    `json:"priority"`
			Description string `json:"description"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		dest := &Destination{
			SetID:       req.SetID,
			Destination: req.Destination,
			Flags:       0,
			Priority:    req.Priority,
			Description: req.Description,
		}

		if err := db.AddDestination(dest); err != nil {
			logger.WithError(err).Error("Failed to add destination to database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add destination"})
			return
		}

		// Also add to Kamailio dispatcher
		if err := kamailio.DispatcherAdd(req.SetID, req.Destination); err != nil {
			logger.WithError(err).Error("Failed to add destination to Kamailio")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add destination to Kamailio"})
			return
		}

		c.JSON(http.StatusOK, dest)
	}
}

func removeDestinationHandler(kamailio *KamailioClient, db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			SetID       int    `json:"setid" binding:"required"`
			Destination string `json:"destination" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.RemoveDestination(req.SetID, req.Destination); err != nil {
			logger.WithError(err).Error("Failed to remove destination from database")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove destination"})
			return
		}

		if err := kamailio.DispatcherRemove(req.SetID, req.Destination); err != nil {
			logger.WithError(err).Error("Failed to remove destination from Kamailio")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove destination from Kamailio"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Destination removed successfully"})
	}
}

func listDestinationsHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		setidInt := 1
		if setidStr := c.Query("setid"); setidStr != "" {
			if _, err := fmt.Sscanf(setidStr, "%d", &setidInt); err == nil {
				if setidInt == 0 {
					setidInt = 1
				}
			}
		}

		destinations, err := db.ListDestinations(setidInt)
		if err != nil {
			logger.WithError(err).Error("Failed to list destinations")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list destinations"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"destinations": destinations})
	}
}

func reloadDispatcherHandler(kamailio *KamailioClient) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := kamailio.DispatcherReload(); err != nil {
			logger.WithError(err).Error("Failed to reload dispatcher")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to reload dispatcher"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Dispatcher reloaded successfully"})
	}
}

// Carrier handlers
func addCarrierHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CarrierID   int    `json:"carrierid" binding:"required"`
			CarrierName string `json:"carrier_name" binding:"required"`
			GWList      string `json:"gwlist" binding:"required"`
			Description string `json:"description"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		carrier := &Carrier{
			CarrierID:   req.CarrierID,
			CarrierName: req.CarrierName,
			GWList:      req.GWList,
			Description: req.Description,
		}

		if err := db.AddCarrier(carrier); err != nil {
			logger.WithError(err).Error("Failed to add carrier")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add carrier"})
			return
		}

		c.JSON(http.StatusOK, carrier)
	}
}

func removeCarrierHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			CarrierID int `json:"carrierid" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := db.RemoveCarrier(req.CarrierID); err != nil {
			logger.WithError(err).Error("Failed to remove carrier")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove carrier"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Carrier removed successfully"})
	}
}

func listCarriersHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		carriers, err := db.ListCarriers()
		if err != nil {
			logger.WithError(err).Error("Failed to list carriers")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list carriers"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"carriers": carriers})
	}
}

func addRouteHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			GroupID  int    `json:"groupid" binding:"required"`
			Prefix   string `json:"prefix" binding:"required"`
			Priority int    `json:"priority"`
			RouteID  int    `json:"routeid" binding:"required"`
			GWList   string `json:"gwlist" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		route := &Route{
			GroupID:  req.GroupID,
			Prefix:   req.Prefix,
			Priority: req.Priority,
			RouteID:  req.RouteID,
			GWList:   req.GWList,
		}

		if err := db.AddRoute(route); err != nil {
			logger.WithError(err).Error("Failed to add route")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to add route"})
			return
		}

		c.JSON(http.StatusOK, route)
	}
}

func getRouteHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		prefix := c.Param("prefix")

		route, err := db.GetRoute(prefix)
		if err != nil {
			logger.WithError(err).Error("Failed to get route")
			c.JSON(http.StatusNotFound, gin.H{"error": "Route not found"})
			return
		}

		c.JSON(http.StatusOK, route)
	}
}

// Health check handlers
func healthStatusHandler(db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		checks, err := db.GetHealthStatus()
		if err != nil {
			logger.WithError(err).Error("Failed to get health status")
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get health status"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"health_checks": checks})
	}
}

func performHealthCheckHandler(kamailio *KamailioClient, db *DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Destination string `json:"destination" binding:"required"`
		}

		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		isHealthy, duration, err := kamailio.HealthCheck(req.Destination)
		if err != nil {
			logger.WithError(err).Error("Health check failed")
			status := "unhealthy"
			responseTime := int(duration.Milliseconds())
			db.UpdateHealthCheck(req.Destination, status, responseTime)
			c.JSON(http.StatusOK, gin.H{
				"destination":   req.Destination,
				"status":        status,
				"response_time": responseTime,
				"error":         err.Error(),
			})
			return
		}

		status := "healthy"
		if !isHealthy {
			status = "unhealthy"
		}
		responseTime := int(duration.Milliseconds())

		if err := db.UpdateHealthCheck(req.Destination, status, responseTime); err != nil {
			logger.WithError(err).Error("Failed to update health check in database")
		}

		c.JSON(http.StatusOK, gin.H{
			"destination":   req.Destination,
			"status":        status,
			"response_time": responseTime,
		})
	}
}
