package api

import (
	"net/http"

	service "aetherlink-iot/backend/internal/service"

	"github.com/gin-gonic/gin"
)

func deploymentHealthStatusCode(report service.DeploymentHealthReport) int {
	if report.Status == "ok" {
		return http.StatusOK
	}
	return http.StatusServiceUnavailable
}

// Readiness reports whether all required local/core dependencies are ready.
// Optional external capabilities remain visible in the report but do not
// change the HTTP status unless they are explicitly modeled as required checks.
func (*SystemApi) Readiness(c *gin.Context) {
	report := service.RunDeploymentHealthCheck()
	c.JSON(deploymentHealthStatusCode(report), report)
}

func (*SystemApi) DeploymentHealth(c *gin.Context) {
	report := service.RunDeploymentHealthCheck()
	c.JSON(deploymentHealthStatusCode(report), report)
}
