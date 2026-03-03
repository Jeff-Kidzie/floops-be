package handlers

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/jeff-kidzie/floops-be/internal/database"
	"github.com/jeff-kidzie/floops-be/internal/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	oauth2api "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var googleOauthConfig *oauth2.Config

func InitGoogleOAuth() {
	googleOauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func GoogleLogin(c *gin.Context) {
	state, err := generateStateToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate state token"})
		return
	}

	c.SetCookie("oauth_state", state, 300, "/", "", false, true)
	url := googleOauthConfig.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
}

func GoogleCallback(c *gin.Context) {
	savedState, err := c.Cookie("oauth_state")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing state cookie"})
		return
	}

	if c.Query("state") != savedState {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid state parameter"})
		return
	}

	code := c.Query("code")
	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing authorization code"})
		return
	}

	token, err := googleOauthConfig.Exchange(c.Request.Context(), code)
	if err != nil {
		log.Printf("google oauth exchange error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to exchange authorization code"})
		return
	}

	oauth2Service, err := oauth2api.NewService(c.Request.Context(), option.WithTokenSource(googleOauthConfig.TokenSource(c.Request.Context(), token)))
	if err != nil {
		log.Printf("google oauth2 service error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create OAuth2 service"})
		return
	}

	userInfo, err := oauth2Service.Userinfo.Get().Do()
	if err != nil {
		log.Printf("google userinfo error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get user info from Google"})
		return
	}

	user, err := findOrCreateGoogleUser(userInfo.Id, userInfo.Email, userInfo.Name)
	if err != nil {
		log.Printf("find or create google user error: %v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to process user"})
		return
	}

	claims := jwt.MapClaims{
		"sub":      user.ID,
		"email":    user.Email,
		"username": user.Username,
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
	}
	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := jwtToken.SignedString(jwtSecret)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token": tokenString,
		"user":  user,
	})
}

func findOrCreateGoogleUser(googleID, email, name string) (*models.User, error) {
	var user models.User

	// Try to find by google_id first
	query := `SELECT id, email, username, COALESCE(password_hash, ''), google_id, created_at FROM users WHERE google_id = $1`
	err := database.DB.QueryRow(query, googleID).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.GoogleID, &user.CreatedAt)
	if err == nil {
		return &user, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Try to find by email and link the Google account
	query = `SELECT id, email, username, COALESCE(password_hash, ''), google_id, created_at FROM users WHERE email = $1`
	err = database.DB.QueryRow(query, email).Scan(&user.ID, &user.Email, &user.Username, &user.PasswordHash, &user.GoogleID, &user.CreatedAt)
	if err == nil {
		// Link Google ID to existing account
		_, err = database.DB.Exec(`UPDATE users SET google_id = $1 WHERE id = $2`, googleID, user.ID)
		if err != nil {
			return nil, err
		}
		user.GoogleID = &googleID
		return &user, nil
	}
	if err != sql.ErrNoRows {
		return nil, err
	}

	// Create new user
	query = `INSERT INTO users (email, username, google_id, created_at) VALUES ($1, $2, $3, $4) RETURNING id, email, username, google_id, created_at`
	err = database.DB.QueryRow(query, email, name, googleID, time.Now()).Scan(&user.ID, &user.Email, &user.Username, &user.GoogleID, &user.CreatedAt)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
