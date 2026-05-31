package service

import (
	"context"
	"fmt"
	"time"

	apperrors "launchs/shared/errors"
	"launchs/shared/model"
	"backend/repository"
	"launchs/shared/temporal"

	"github.com/google/uuid"
	"go.temporal.io/sdk/client"
)

// serviceResourceName は routeID から Service の k8s リソース名を生成します。
func serviceResourceName(routeID uuid.UUID) string {
	return fmt.Sprintf("svc-%s", routeID.String())
}

// ingressResourceName は routeID から IngressRoute の k8s リソース名を生成します。
func ingressResourceName(routeID uuid.UUID) string {
	return fmt.Sprintf("ingress-%s", routeID.String())
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

func (s *routeService) List(ctx context.Context, userID string, projectID, containerID uuid.UUID) ([]RouteWithEndpoint, error) {
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

	svcName := serviceResourceName(routeID)

	type serviceSpec struct {
		Namespace      string `json:"Namespace"`
		Name           string `json:"Name"`
		Ports          []struct {
			Port     int    `json:"Port"`
			Protocol string `json:"Protocol"`
		} `json:"Ports"`
		SelectorLabels map[string]string `json:"SelectorLabels"`
	}
	type createServiceInput struct {
		ContainerID uuid.UUID   `json:"ContainerID"`
		ProjectID   uuid.UUID   `json:"ProjectID"`
		RouteID     uuid.UUID   `json:"RouteID"`
		ServiceSpec serviceSpec `json:"ServiceSpec"`
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-service-%s", routeID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateService, createServiceInput{
		ContainerID: containerID,
		ProjectID:   projectID,
		RouteID:     routeID,
		ServiceSpec: serviceSpec{
			Namespace: project.Namespace,
			Name:      svcName,
			Ports: []struct {
				Port     int    `json:"Port"`
				Protocol string `json:"Protocol"`
			}{
				{Port: port, Protocol: protocol},
			},
			SelectorLabels: map[string]string{
				"container-id": containerID.String(),
			},
		},
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

	// 対象ポートに対応する Service ルートを検索してService名を解決する
	existingRoutes, err := s.routeRepo.FindByContainerID(ctx, containerID)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to list routes: %w", err)
	}
	svcRouteID := uuid.UUID{}
	for _, r := range existingRoutes {
		if r.Type == string(model.NetworkRouteTypeService) && r.Port == port {
			svcRouteID = r.ID
			break
		}
	}
	if svcRouteID == (uuid.UUID{}) {
		return "", "", "", fmt.Errorf("port %d に対応する Service が見つかりません。先に Service を作成してください", port)
	}

	ingressName := ingressResourceName(routeID)
	svcName := serviceResourceName(svcRouteID)

	type ingressSpec struct {
		Namespace   string `json:"Namespace"`
		Name        string `json:"Name"`
		ServiceName string `json:"ServiceName"`
		Host        string `json:"Host"`
		Port        int    `json:"Port"`
	}
	type createIngressInput struct {
		ContainerID uuid.UUID   `json:"ContainerID"`
		ProjectID   uuid.UUID   `json:"ProjectID"`
		IngressSpec ingressSpec `json:"IngressSpec"`
	}

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("create-ingress-%s", routeID.String()),
		TaskQueue: temporal.ControllerQueue,
	}
	we, err := s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowCreateIngress, createIngressInput{
		ContainerID: containerID,
		ProjectID:   projectID,
		IngressSpec: ingressSpec{
			Namespace:   project.Namespace,
			Name:        ingressName,
			ServiceName: svcName,
			Host:        subdomain,
			Port:        port,
		},
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

	wfOpts := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("delete-route-%s-%d", routeID.String(), time.Now().UnixNano()),
		TaskQueue: temporal.ControllerQueue,
	}

	var we client.WorkflowRun
	if route.Type == string(model.NetworkRouteTypeIngress) {
		type deleteIngressInput struct {
			ContainerID uuid.UUID `json:"ContainerID"`
			ProjectID   uuid.UUID `json:"ProjectID"`
			Namespace   string    `json:"Namespace"`
			IngressName string    `json:"IngressName"`
		}
		we, err = s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteIngress, deleteIngressInput{
			ContainerID: containerID,
			ProjectID:   projectID,
			Namespace:   project.Namespace,
			IngressName: ingressResourceName(routeID),
		})
	} else {
		type deleteServiceInput struct {
			ContainerID uuid.UUID `json:"ContainerID"`
			ProjectID   uuid.UUID `json:"ProjectID"`
			Namespace   string    `json:"Namespace"`
			ServiceName string    `json:"ServiceName"`
		}
		we, err = s.temporal.ExecuteWorkflow(ctx, wfOpts, temporal.WorkflowDeleteService, deleteServiceInput{
			ContainerID: containerID,
			ProjectID:   projectID,
			Namespace:   project.Namespace,
			ServiceName: serviceResourceName(routeID),
		})
	}
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
