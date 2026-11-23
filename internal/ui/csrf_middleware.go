// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"miniflux.app/v2/internal/crypto"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
)

type csrfMiddleware struct {
	basePath string
}

func newCSRFMiddleware(basePath string) *csrfMiddleware {
	return &csrfMiddleware{basePath: basePath}
}

// handle validates the CSRF token on state-changing requests. It must be
// chained inside handleWebSession so that the session is already present
// in the request context.
func (m *csrfMiddleware) handle(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// The request passed down is a copy made by WithContext, so net/http
		// won't remove the multipart temporary files on its own.
		defer func() {
			if r.MultipartForm != nil {
				r.MultipartForm.RemoveAll()
			}
		}()

		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			// Safe methods don't require CSRF validation.
		default:
			if !m.validate(w, r) {
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// isCrossOrigin reports whether the request carries an explicit signal that it
// was issued by another origin. It is deliberately conservative: requests
// without an Origin header and without a Sec-Fetch-Site header (command-line
// clients, older browsers) are not cross-origin, and same-origin navigations
// ("none") or same-origin fetches are accepted, so the token check alone still
// decides those. Only a foreign Origin or an explicit "cross-site" fetch site
// is treated as cross-origin.
func isCrossOrigin(r *http.Request) bool {
	if origin := r.Header.Get("Origin"); origin != "" {
		parsedOrigin, err := url.Parse(origin)
		if err != nil {
			return true
		}
		if !strings.EqualFold(parsedOrigin.Host, r.Host) {
			return true
		}
	}

	return strings.EqualFold(r.Header.Get("Sec-Fetch-Site"), "cross-site")
}

func (m *csrfMiddleware) validate(w http.ResponseWriter, r *http.Request) bool {
	if isCrossOrigin(r) {
		slog.Warn("Rejected cross-origin state-changing request",
			slog.String("url", r.RequestURI),
			slog.String("origin", r.Header.Get("Origin")),
			slog.String("sec_fetch_site", r.Header.Get("Sec-Fetch-Site")),
		)
		response.HTMLForbidden(w, r)
		return false
	}

	session := request.WebSession(r)
	if session == nil {
		response.HTMLBadRequest(w, r, errors.New("invalid or missing CSRF"))
		return false
	}

	csrfToken := session.CSRF()
	if crypto.ConstantTimeCmp(csrfToken, r.Header.Get("X-Csrf-Token")) || crypto.ConstantTimeCmp(csrfToken, csrfFormValue(r)) {
		return true
	}

	slog.Warn("Invalid or missing CSRF token",
		slog.String("url", r.RequestURI),
	)

	if r.URL.Path == "/login" {
		response.HTMLRedirect(w, r, m.basePath+"/")
	} else {
		response.HTMLBadRequest(w, r, errors.New("invalid or missing CSRF"))
	}

	return false
}

// csrfFormValue returns the CSRF token from the request body, parsing multipart forms only for authenticated users.
func csrfFormValue(r *http.Request) string {
	if request.IsAuthenticated(r) {
		return r.PostFormValue("csrf")
	}

	if err := r.ParseForm(); err != nil {
		return ""
	}
	return r.PostForm.Get("csrf")
}
