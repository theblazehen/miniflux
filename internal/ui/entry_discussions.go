// SPDX-FileCopyrightText: Copyright The Miniflux Authors. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package ui // import "miniflux.app/v2/internal/ui"

import (
	"errors"
	"net/http"

	"miniflux.app/v2/internal/discussion"
	"miniflux.app/v2/internal/http/request"
	"miniflux.app/v2/internal/http/response"
)

func (h *handler) fetchDiscussions(w http.ResponseWriter, r *http.Request) {
	loggedUserID := request.UserID(r)
	entryID := request.RouteInt64Param(r, "entryID")

	if entryID == 0 {
		response.JSONBadRequest(w, r, errors.New("invalid entry ID"))
		return
	}

	// Upstream dropped model.EntryStatusRemoved, so the raw status value the
	// fork used to exclude is kept here: removed entries stay invisible, and
	// the query is scoped to the logged-in user through loggedUserID.
	entry, err := h.store.NewEntryQueryBuilder(loggedUserID).
		WithEntryIDs(entryID).
		WithoutStatus("removed").
		GetEntry()
	if err != nil {
		response.JSONServerError(w, r, err)
		return
	}

	if entry == nil {
		response.JSONNotFound(w, r)
		return
	}

	if entry.URL == "" {
		response.JSON(w, r, discussion.DiscussionResponse{Discussions: []discussion.DiscussionLink{}})
		return
	}

	result := h.discussions.Find(r.Context(), entry.URL)
	response.JSON(w, r, result)
}
