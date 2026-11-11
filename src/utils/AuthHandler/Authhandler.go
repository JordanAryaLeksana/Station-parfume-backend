package authhandler

import (
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var GoogleOauthConfig *oauth2.Config

// UserInfoURL: endpoint userinfo OpenID Connect Google (memberi "sub" dan
// "email_verified"). Variabel supaya bisa diarahkan ke server palsu saat test.
var UserInfoURL = "https://openidconnect.googleapis.com/v1/userinfo"

func InitGoogleOauthConfig() {
	GoogleOauthConfig = &oauth2.Config{
		ClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		ClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		RedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		Scopes:       []string{"openid", "email", "profile"},
		Endpoint:     google.Endpoint,
	}
}

// FrontendURL dari env FRONTEND_URL tanpa "/" di akhir.
func FrontendURL() string { return strings.TrimRight(os.Getenv("FRONTEND_URL"), "/") }

// GoogleConfigured: semua env Google + FRONTEND_URL terisi.
func GoogleConfigured() bool {
	c := GoogleOauthConfig
	return c != nil && c.ClientID != "" && c.ClientSecret != "" && c.RedirectURL != "" && FrontendURL() != ""
}
