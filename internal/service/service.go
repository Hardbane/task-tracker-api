package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"task-team-api/internal/auth"
	"task-team-api/internal/email"
	"task-team-api/internal/models"
	"task-team-api/internal/repository"
)

var (
	ErrBadRequest = errors.New("bad request")
	ErrForbidden  = errors.New("forbidden")
	ErrConflict   = errors.New("conflict")
)

type Service struct {
	repo     *repository.Repository
	jwt      *auth.Manager
	redis    *redis.Client
	cacheTTL time.Duration
	email    email.Client
}

func New(repo *repository.Repository, jwt *auth.Manager, redisClient *redis.Client, cacheTTL time.Duration, emailClient email.Client) *Service {
	return &Service{repo: repo, jwt: jwt, redis: redisClient, cacheTTL: cacheTTL, email: emailClient}
}

func (s *Service) Register(ctx context.Context, name, userEmail, password string) (models.User, error) {
	name = strings.TrimSpace(name)
	userEmail = strings.ToLower(strings.TrimSpace(userEmail))
	if name == "" || userEmail == "" || len(password) < 8 {
		return models.User{}, fmt.Errorf("%w: name, valid email and password length >= 8 are required", ErrBadRequest)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return models.User{}, err
	}
	user, err := s.repo.CreateUser(ctx, name, userEmail, string(hash))
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") {
			return models.User{}, fmt.Errorf("%w: email already exists", ErrConflict)
		}
		return models.User{}, err
	}
	return user, nil
}

func (s *Service) Login(ctx context.Context, userEmail, password string) (string, models.User, error) {
	userEmail = strings.ToLower(strings.TrimSpace(userEmail))
	user, err := s.repo.GetUserByEmail(ctx, userEmail)
	if err != nil {
		return "", models.User{}, fmt.Errorf("%w: invalid email or password", ErrBadRequest)
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", models.User{}, fmt.Errorf("%w: invalid email or password", ErrBadRequest)
	}
	token, err := s.jwt.Generate(user.ID)
	if err != nil {
		return "", models.User{}, err
	}
	return token, user, nil
}

func (s *Service) CreateTeam(ctx context.Context, userID int64, name string) (models.Team, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return models.Team{}, fmt.Errorf("%w: team name is required", ErrBadRequest)
	}
	return s.repo.CreateTeamWithOwner(ctx, name, userID)
}

func (s *Service) ListTeams(ctx context.Context, userID int64) ([]models.Team, error) {
	return s.repo.ListTeamsForUser(ctx, userID)
}

func (s *Service) InviteUser(ctx context.Context, actorID, teamID int64, inviteEmail string, role models.Role) error {
	if role == "" {
		role = models.RoleMember
	}
	if role != models.RoleAdmin && role != models.RoleMember {
		return fmt.Errorf("%w: invite role must be admin or member", ErrBadRequest)
	}
	actorRole, err := s.repo.GetTeamRole(ctx, teamID, actorID)
	if err != nil {
		return fmt.Errorf("%w: you are not a member of this team", ErrForbidden)
	}
	if actorRole != models.RoleOwner && actorRole != models.RoleAdmin {
		return fmt.Errorf("%w: only owner/admin can invite", ErrForbidden)
	}

	user, err := s.repo.GetUserByEmail(ctx, strings.ToLower(strings.TrimSpace(inviteEmail)))
	if err != nil {
		return fmt.Errorf("%w: user to invite was not found", ErrBadRequest)
	}
	if err := s.repo.AddTeamMember(ctx, teamID, user.ID, role); err != nil {
		return err
	}
	team, _ := s.repo.GetTeam(ctx, teamID, actorID)
	return s.email.SendInvite(ctx, user.Email, team.Name)
}

func (s *Service) CreateTask(ctx context.Context, actorID int64, input models.Task) (models.Task, error) {
	if input.TeamID == 0 || strings.TrimSpace(input.Title) == "" {
		return models.Task{}, fmt.Errorf("%w: team_id and title are required", ErrBadRequest)
	}
	if input.Status == "" {
		input.Status = models.StatusTodo
	}
	if !validStatus(input.Status) {
		return models.Task{}, fmt.Errorf("%w: invalid task status", ErrBadRequest)
	}
	ok, err := s.repo.IsTeamMember(ctx, input.TeamID, actorID)
	if err != nil {
		return models.Task{}, err
	}
	if !ok {
		return models.Task{}, fmt.Errorf("%w: only team member can create tasks", ErrForbidden)
	}
	if input.AssigneeID != nil {
		assigneeIsMember, err := s.repo.IsTeamMember(ctx, input.TeamID, *input.AssigneeID)
		if err != nil {
			return models.Task{}, err
		}
		if !assigneeIsMember {
			return models.Task{}, fmt.Errorf("%w: assignee must be a team member", ErrBadRequest)
		}
	}
	input.CreatedBy = actorID
	task, err := s.repo.CreateTask(ctx, input)
	if err != nil {
		return models.Task{}, err
	}
	s.invalidateTaskCache(ctx, input.TeamID)
	return task, nil
}

func (s *Service) ListTasks(ctx context.Context, actorID, teamID int64, status *models.TaskStatus, assigneeID *int64, page, limit int) ([]models.Task, error) {
	if teamID == 0 {
		return nil, fmt.Errorf("%w: team_id is required", ErrBadRequest)
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}
	ok, err := s.repo.IsTeamMember(ctx, teamID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: only team member can list tasks", ErrForbidden)
	}

	key := taskCacheKey(teamID, status, assigneeID, page, limit)
	if s.redis != nil {
		if raw, err := s.redis.Get(ctx, key).Bytes(); err == nil {
			var cached []models.Task
			if err := json.Unmarshal(raw, &cached); err == nil {
				return cached, nil
			}
		}
	}

	offset := (page - 1) * limit
	tasks, err := s.repo.ListTasks(ctx, teamID, status, assigneeID, limit, offset)
	if err != nil {
		return nil, err
	}
	if s.redis != nil {
		if raw, err := json.Marshal(tasks); err == nil {
			_ = s.redis.Set(ctx, key, raw, s.cacheTTL).Err()
		}
	}
	return tasks, nil
}

func (s *Service) UpdateTask(ctx context.Context, actorID, taskID int64, title *string, description *string, status *models.TaskStatus, assigneeID *int64) (models.Task, error) {
	old, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return models.Task{}, err
	}
	if status != nil && !validStatus(*status) {
		return models.Task{}, fmt.Errorf("%w: invalid task status", ErrBadRequest)
	}
	role, err := s.repo.GetTeamRole(ctx, old.TeamID, actorID)
	if err != nil {
		return models.Task{}, fmt.Errorf("%w: only team member can update tasks", ErrForbidden)
	}
	allowed := role == models.RoleOwner || role == models.RoleAdmin || old.CreatedBy == actorID || (old.AssigneeID != nil && *old.AssigneeID == actorID)
	if !allowed {
		return models.Task{}, fmt.Errorf("%w: not enough rights to update task", ErrForbidden)
	}
	if assigneeID != nil {
		assigneeIsMember, err := s.repo.IsTeamMember(ctx, old.TeamID, *assigneeID)
		if err != nil {
			return models.Task{}, err
		}
		if !assigneeIsMember {
			return models.Task{}, fmt.Errorf("%w: assignee must be a team member", ErrBadRequest)
		}
	}
	updated, err := s.repo.UpdateTask(ctx, taskID, actorID, old, title, description, status, assigneeID)
	if err != nil {
		return models.Task{}, err
	}
	s.invalidateTaskCache(ctx, old.TeamID)
	return updated, nil
}

func (s *Service) TaskHistory(ctx context.Context, actorID, taskID int64) ([]models.TaskHistory, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	ok, err := s.repo.IsTeamMember(ctx, task.TeamID, actorID)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: only team member can read history", ErrForbidden)
	}
	return s.repo.ListTaskHistory(ctx, taskID)
}

func (s *Service) TeamSummaries(ctx context.Context) ([]models.TeamSummary, error) {
	return s.repo.TeamSummaries(ctx)
}

func (s *Service) TopCreatorsByTeam(ctx context.Context, month time.Time) ([]models.TopCreator, error) {
	return s.repo.TopCreatorsByTeam(ctx, month)
}

func (s *Service) InvalidAssigneeTasks(ctx context.Context) ([]models.InvalidAssigneeTask, error) {
	return s.repo.InvalidAssigneeTasks(ctx)
}

func validStatus(status models.TaskStatus) bool {
	return status == models.StatusTodo || status == models.StatusInProgress || status == models.StatusDone
}

func taskCacheKey(teamID int64, status *models.TaskStatus, assigneeID *int64, page, limit int) string {
	statusPart := "all"
	if status != nil {
		statusPart = string(*status)
	}
	assigneePart := "all"
	if assigneeID != nil {
		assigneePart = fmt.Sprintf("%d", *assigneeID)
	}
	return fmt.Sprintf("team_tasks:%d:%s:%s:%d:%d", teamID, statusPart, assigneePart, page, limit)
}

func (s *Service) invalidateTaskCache(ctx context.Context, teamID int64) {
	if s.redis == nil {
		return
	}
	iter := s.redis.Scan(ctx, 0, fmt.Sprintf("team_tasks:%d:*", teamID), 100).Iterator()
	for iter.Next(ctx) {
		_ = s.redis.Del(ctx, iter.Val()).Err()
	}
}
