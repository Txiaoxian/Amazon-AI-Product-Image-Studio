package auth

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

func (s *Service) setSessionCookies(c *gin.Context, session SessionResponse) {
	principal := Principal{
		UserID:         session.User.ID,
		TenantID:       session.Tenant.ID,
		Email:          session.User.Email,
		DisplayName:    session.User.DisplayName,
		Status:         session.User.Status,
		SessionVersion: session.SessionVersion,
		CSRFToken:      session.CSRFToken,
		Roles:          session.Roles,
		Permissions:    session.Permissions,
	}

	token, err := createToken(s.cfg.Auth.JWTSigningSecret, s.cfg.Auth.JWTIssuer, principal, s.cfg.Auth.AccessTokenTTL, s.now())
	if err != nil {
		panic(err)
	}

	maxAge := int(s.cfg.Auth.AccessTokenTTL.Seconds())
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.Auth.Cookie.Name,
		Value:    token,
		Path:     "/",
		Domain:   s.cfg.Auth.Cookie.Domain,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   s.cookieSecure(),
		SameSite: s.cookieSameSite(),
	})

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.Auth.CSRF.CookieName,
		Value:    session.CSRFToken,
		Path:     "/",
		Domain:   s.cfg.Auth.Cookie.Domain,
		MaxAge:   maxAge,
		HttpOnly: false,
		Secure:   s.cookieSecure(),
		SameSite: s.cookieSameSite(),
	})
}

func (s *Service) clearSessionCookies(c *gin.Context) {
	expiredAt := time.Unix(0, 0).UTC()
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.Auth.Cookie.Name,
		Value:    "",
		Path:     "/",
		Domain:   s.cfg.Auth.Cookie.Domain,
		MaxAge:   -1,
		Expires:  expiredAt,
		HttpOnly: true,
		Secure:   s.cookieSecure(),
		SameSite: s.cookieSameSite(),
	})
	http.SetCookie(c.Writer, &http.Cookie{
		Name:     s.cfg.Auth.CSRF.CookieName,
		Value:    "",
		Path:     "/",
		Domain:   s.cfg.Auth.Cookie.Domain,
		MaxAge:   -1,
		Expires:  expiredAt,
		HttpOnly: false,
		Secure:   s.cookieSecure(),
		SameSite: s.cookieSameSite(),
	})
}

func (s *Service) cookieSecure() bool {
	return s.cfg.Auth.Cookie.Secure || s.cfg.IsProduction()
}

func (s *Service) cookieSameSite() http.SameSite {
	switch s.cfg.Auth.Cookie.SameSite {
	case "Strict":
		return http.SameSiteStrictMode
	case "None":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
