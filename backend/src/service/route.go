package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
	"launchs/shared/model"
	"launchs/shared/temporal"
	apperrors "launchs/shared/errors"
	"backend/repository"
)

// CreateServiceWorkflowInput は CreateServiceWorkflow の入力です。
type CreateServiceWorkflowInput struct {
	RouteID     string `json:"route_id"`
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Namespace   string `json:"namespace"`
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
}

// CreateIngressWorkflowInput は CreateIngressWorkflow の入力です。
type CreateIngressWorkflowInput struct {
	RouteID     string `json:"route_id"`
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Namespace   string `json:"namespace"`
	Port        int    `json:"port"`
	Subdomain   string `json:"subdomain"`
}

// DeleteRouteWorkflowInput はルート削除ワークフローの入力です。
type DeleteRouteWorkflowInput struct {
	RouteID     string `json:"route_id"`
	RouteType   string `json:"route_type"`
	ContainerID string `json:"container_id"`
	ProjectID   string `json:"project_id"`
	Namespace   string `json:"namespace"`
}

type routeService struct {
	projectRepo   repository.ProjectRepository
	containerRepo repository.ContainerRepository
	routeRepo     repository.NetworkRouteRepository
	temporal      client.Client
}

// NewRouteService は RouteService の実装を返します。
func NewRouteService(
	projectRepo repository.ProjectRepository,
	containerRepo repository.ContainerRepository,
	routeRepo repository.NetworkRouteRepository,
	temporalClient client.Client,
) RouteService {
	return &routeService{
		projectRepo:   projectRepo,
		containerRepo: containerRepo,
		routeRepo:     routeRepo,
		temporal:      temporalClient,
	}
}

func (s *routeService) List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]model.NetworkRoute, error) {
	if err := s.checkAccess(ctx, userID, projectID, containerID); err != nil {
		return nil, err
	}
	return s.routeRepo.FindByContainerID(ctx, containerID)
}

func (s *routeService) CreateService(ctx context.Context, userID string, projectID, containerID uuid.UUID, port int, protocol string) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	routeID := uuid.New()
	route := &model.NetworkRoute{
		ID:          routeID,
		ContainerID: containerID,
		Type:        string(model.NetworkRouteTypeService),
		Port:        port,
		Protocol:    protocol,
	}
	if err := s.routeRepo.Create(ctx, route); err != nil {
		return "", fmt.Errorf("failed to create route: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-service-%s", routeID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateService, CreateServiceWorkflowInput{
		RouteID:     routeID.String(),
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Namespace:   project.Namespace,
		Port:        port,
		Protocol:    protocol,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start CreateServiceWorkflow: %w", err)
	}

	return we.GetID(), nil
}

func (s *routeService) CreateIngress(ctx context.Context, userID string, projectID, containerID uuid.UUID, port int) (string, string, string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", "", "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", "", "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return "", "", "", &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}

	// サブドメイン: {container-name}-{project-uuid}.launchs.org
	subdomain := fmt.Sprintf("%s-%s.launchs.org", container.Name, projectID.String())

	routeID := uuid.New()
	route := &model.NetworkRoute{
		ID:          routeID,
		ContainerID: containerID,
		Type:        string(model.NetworkRouteTypeIngress),
		Port:        port,
		Subdomain:   &subdomain,
	}
	if err := s.routeRepo.Create(ctx, route); err != nil {
		return "", "", "", fmt.Errorf("failed to create route: %w", err)
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-ingress-%s", routeID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateIngress, CreateIngressWorkflowInput{
		RouteID:     routeID.String(),
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Namespace:   project.Namespace,
		Port:        port,
		Subdomain:   subdomain,
	})
	if err != nil {
		return "", "", "", fmt.Errorf("failed to start CreateIngressWorkflow: %w", err)
	}

	return routeID.String(), subdomain, we.GetID(), nil
}

func (s *routeService) Delete(ctx context.Context, userID string, projectID, containerID, routeID uuid.UUID) (string, error) {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return "", &apperrors.ForbiddenError{Message: "access denied"}
	}

	route, err := s.routeRepo.FindByID(ctx, routeID)
	if err != nil {
		return "", &apperrors.NotFoundError{Resource: "route", ID: routeID.String()}
	}

	if err := s.routeRepo.Delete(ctx, routeID); err != nil {
		return "", fmt.Errorf("failed to delete route: %w", err)
	}

	// ルート種別に応じてワークフローを切り替えます
	wfName := temporal.WorkflowDeleteService
	if route.Type == string(model.NetworkRouteTypeIngress) {
		wfName = temporal.WorkflowDeleteIngress
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-route-%s-%d", routeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, wfName, DeleteRouteWorkflowInput{
		RouteID:     routeID.String(),
		RouteType:   route.Type,
		ContainerID: containerID.String(),
		ProjectID:   projectID.String(),
		Namespace:   project.Namespace,
	})
	if err != nil {
		return "", fmt.Errorf("failed to start delete route workflow: %w", err)
	}

	return we.GetID(), nil
}

func (s *routeService) checkAccess(ctx context.Context, userID string, projectID, containerID uuid.UUID) error {
	project, err := s.projectRepo.FindByID(ctx, projectID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "project", ID: projectID.String()}
	}
	if project.UserID != userID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}
	container, err := s.containerRepo.FindByID(ctx, containerID)
	if err != nil {
		return &apperrors.NotFoundError{Resource: "container", ID: containerID.String()}
	}
	if container.ProjectID != projectID {
		return &apperrors.ForbiddenError{Message: "access denied"}
	}
	return nil
}
