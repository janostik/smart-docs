package auth

import (
	"html/template"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/securecookie"
)

var cookieHandler = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32),
)

const (
	apiKeyCookieName = "api_key"
)

func ValidateAPIKey(apiKey string) bool {
	return apiKey == os.Getenv("API_KEY")
}

func SetAPICookie(w http.ResponseWriter, apiKey string) {
	value := map[string]string{
		"api_key": apiKey,
	}
	encoded, err := cookieHandler.Encode(apiKeyCookieName, value)
	if err != nil {
		return
	}

	cookie := &http.Cookie{
		Name:     apiKeyCookieName,
		Value:    encoded,
		Path:     "/",
		MaxAge:   3600 * 24 * 7, // 7 days
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
	}
	http.SetCookie(w, cookie)
}

func GetAPICookie(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(apiKeyCookieName)
	if err != nil {
		return "", false
	}

	value := make(map[string]string)
	err = cookieHandler.Decode(apiKeyCookieName, cookie.Value, &value)
	if err != nil {
		return "", false
	}

	apiKey, exists := value["api_key"]
	return apiKey, exists
}

func AuthMiddleware(tmpl *template.Template, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip authentication for static assets and health check
		if strings.HasPrefix(r.URL.Path, "/assets/") ||
			strings.HasPrefix(r.URL.Path, "/images/") ||
			r.URL.Path == "/health" {
			next.ServeHTTP(w, r)
			return
		}

		if r.URL.Path == "/auth" && r.Method == http.MethodPost {
			apiKey := r.FormValue("api_key")
			if ValidateAPIKey(apiKey) {
				SetAPICookie(w, apiKey)
				http.Redirect(w, r, "/", http.StatusSeeOther)
				return
			}
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}

		apiKey, exists := GetAPICookie(r)
		if !exists || !ValidateAPIKey(apiKey) {
			tmpl.ExecuteTemplate(w, "login.go.html", nil)
			return
		}

		next.ServeHTTP(w, r)
	})
}
