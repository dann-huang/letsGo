package user

import (
	"gonext/internal/mdw"
	"gonext/internal/token"
	"gonext/pkg/util/httputil"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func Router(authMdw mdw.Middleware, ctxKey mdw.ContextKey) chi.Router {
	r := chi.NewRouter()
	r.Use(authMdw)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		user := r.Context().Value(ctxKey).(*token.UserPayload)
		httputil.RespondJSON(w, http.StatusOK, map[string]string{
			"message":     "Success",
			"username":    user.Username,
			"displayname": user.Displayname,
		})
	})

	return r
}
