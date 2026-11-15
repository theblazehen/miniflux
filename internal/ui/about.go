// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"net/http"
	"runtime"

	"miniflux.app/v2/internal/config"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
	"miniflux.app/v2/internal/ui/view"
	"miniflux.app/v2/internal/version"
)

func (h *handler) showAboutPage(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.UserByID(request.UserID(r))
	if err != nil {
		response.HTMLServerError(w, r, err)
		return
	}

	dbSize, dbErr := h.store.DBSize()

	view := view.New(h.tpl, r)
	view.Set("version", version.Version)
	view.Set("commit", version.Commit)
	view.Set("build_date", version.BuildDate)
	view.Set("menu", "settings")
	view.Set("user", user)

	// The About page reports the user's own profile, so render it in the
	// language stored on the user record instead of the copy cached in the web
	// session, which is only refreshed at login and can be stale (for example
	// when the language is changed through the API).
	if user.Language != "" {
		view.Set("language", user.Language)
	}
	navMetadata, _ := h.store.GetNavMetadata(user.ID)
	view.Set("countUnread", navMetadata.CountUnread)
	view.Set("countErrorFeeds", navMetadata.CountErrorFeeds)
	view.Set("globalConfigOptions", config.Opts.ConfigMap(true))
	view.Set("postgres_version", h.store.DatabaseVersion())
	view.Set("go_version", runtime.Version())

	if dbErr != nil {
		view.Set("db_usage", dbErr)
	} else {
		view.Set("db_usage", dbSize)
	}

	response.HTML(w, r, view.Render("about"))
}
