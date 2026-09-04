package handler

import (
	"errors"
	"net/http"

	"marrow/internal/handler/dto"
	model "marrow/internal/model"
	services "marrow/internal/service"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	Auth *services.AuthService
}

func NewAuthHandler(auth *services.AuthService) *AuthHandler {
	return &AuthHandler{Auth: auth}
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, access, refresh, err := h.Auth.Register(c.Request.Context(), req.Email, req.Password, req.DisplayName)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrEmailTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		}
		return
	}

	h.writeTokenPair(c, http.StatusCreated, user, access, refresh)
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req dto.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, access, refresh, err := h.Auth.Login(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		if errors.Is(err, services.ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		internalError(c, err)
		return
	}

	h.writeTokenPair(c, http.StatusOK, user, access, refresh)
}

func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, access, refresh, err := h.Auth.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, services.ErrInvalidRefreshToken) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
			return
		}
		internalError(c, err)
		return
	}

	h.writeTokenPair(c, http.StatusOK, user, access, refresh)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	var req dto.LogoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.Auth.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		internalError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Google(c *gin.Context) {
	var req dto.GoogleLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, access, refresh, err := h.Auth.GoogleLogin(c.Request.Context(), req.IDToken)
	if err != nil {
		if errors.Is(err, services.ErrOAuthProviderUnavailable) {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	h.writeTokenPair(c, http.StatusOK, user, access, refresh)
}

func (h *AuthHandler) Me(c *gin.Context) {
	user, ok := model.UserFromContext(c.Request.Context())
	if !ok || user.ID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
		return
	}
	c.JSON(http.StatusOK, dto.FromUser(user))
}
func (h *AuthHandler) AuthRequired() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing authorization token"})
			return
		}

		claims, err := h.Auth.App.Auth.JWTManager.Parse(raw)
		if err != nil || claims == nil || claims.Subject == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
			return
		}

		user := claims.ToUser()
		c.Request = c.Request.WithContext(model.WithUser(c.Request.Context(), user))
		c.Next()
	}
}

// bearerToken extracts the token from an Authorization header of the form
// "Bearer <token>" (case-insensitive scheme). Returns ok=false for anything
// else — no header, wrong scheme, or no token.
func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) > len(prefix) && header[:len(prefix)] == prefix {
		return header[len(prefix):], true
	}
	return "", false
}

// writeTokenPair is the shared response shape for register/login/refresh:
// the access token, its lifetime in seconds, the refresh token, and the user.
func (h *AuthHandler) writeTokenPair(c *gin.Context, status int, user model.User, access, refresh string) {
	c.JSON(status, dto.TokenPairResponse{
		AccessToken:  access,
		RefreshToken: refresh,
		ExpiresIn:    int64(h.Auth.App.Auth.JWTManager.AccessTTL().Seconds()),
		User:         dto.FromUser(user),
	})
}
