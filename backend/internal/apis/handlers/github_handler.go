package handlers

import (
	"neobase-ai/internal/apis/dtos"
	"neobase-ai/internal/services"
	"net/http"

	"github.com/gin-gonic/gin"
)

type GitHubHandler struct {
	githubService services.GitHubService
}

func NewGitHubHandler(githubService services.GitHubService) *GitHubHandler {
	return &GitHubHandler{
		githubService: githubService,
	}
}

// @Summary Get GitHub Stats
// @Description Get GitHub stats
// @Accept json
// @Produce json
// @Success 200 {object} dtos.Response

func (h *GitHubHandler) GetGitHubStats(c *gin.Context) {
	stats, err := h.githubService.GetStats(c.Request.Context())
	if err != nil {
		errorMsg := err.Error()
		c.JSON(http.StatusBadRequest, dtos.Response{
			Success: false,
			Error:   &errorMsg,
		})
		return
	}

	c.JSON(http.StatusOK, dtos.Response{
		Success: true,
		Data:    stats,
	})
}
