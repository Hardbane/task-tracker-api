package app

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"task-team-api/internal/auth"
	apphandler "task-team-api/internal/handler"
	appmiddleware "task-team-api/internal/middleware"
)

func NewRouter(handler *apphandler.Handler, jwtManager *auth.Manager, requestsPerMinute int) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(appmiddleware.Metrics)

	r.Get("/health", handler.Health)
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/v1", func(r chi.Router) {
		r.Post("/register", handler.Register)
		r.Post("/login", handler.Login)

		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.Auth(jwtManager))
			r.Use(appmiddleware.NewRateLimiter(requestsPerMinute).Middleware)

			r.Post("/teams", handler.CreateTeam)
			r.Get("/teams", handler.ListTeams)
			r.Post("/teams/{id}/invite", handler.InviteUser)

			r.Post("/tasks", handler.CreateTask)
			r.Get("/tasks", handler.ListTasks)
			r.Put("/tasks/{id}", handler.UpdateTask)
			r.Get("/tasks/{id}/history", handler.TaskHistory)

			r.Get("/reports/team-summary", handler.TeamSummaries)
			r.Get("/reports/top-creators", handler.TopCreators)
			r.Get("/reports/invalid-assignees", handler.InvalidAssignees)
		})
	})
	return r
}
