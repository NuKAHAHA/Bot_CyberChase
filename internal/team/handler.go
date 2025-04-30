package team

import (
	"Cyber-chase/internal/service"
	"github.com/gin-gonic/gin"
	"net/http"
)

type TeamHandler struct {
	service service.TeamService
}

func NewTeamHandler(service service.TeamService) *TeamHandler {
	return &TeamHandler{service: service}
}

func (h *TeamHandler) RegisterTeam(c *gin.Context) {
	var request struct {
		Email string `json:"email" binding:"required,email"`
		Name  string `json:"name" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.RegisterTeam(request.Email, request.Name); err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"status": "registration_success"})
}

func (h *TeamHandler) LoginTeam(c *gin.Context) {
	var request struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	team, err := h.service.AuthenticateTeam(request.Email, request.Password)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"team_id": team.ID,
		"status":  "authenticated",
	})
}
