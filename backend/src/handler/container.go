package handler

import (
	"net/http"

	"backend/repository"
	"backend/response"
	"backend/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
	"launchs/shared/model"
	"launchs/shared/config"
)

// ContainerHandler はコンテナ関連のリクエストを処理します。
type ContainerHandler struct {
	svc            service.ContainerService
	statusHistRepo repository.ContainerStatusHistoryRepository
}

func NewContainerHandler(svc service.ContainerService, statusHistRepo repository.ContainerStatusHistoryRepository) *ContainerHandler {
	return &ContainerHandler{svc: svc, statusHistRepo: statusHistRepo}
}

func (h *ContainerHandler) List(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}

	containers, err := h.svc.List(c.Request().Context(), projectID)
	if err != nil {
		return response.Error(c, err)
	}

	result := make([]map[string]interface{}, len(containers))
	for i := range containers {
		result[i] = containerSummaryJSON(&containers[i])
	}
	return response.OK(c, result)
}

func (h *ContainerHandler) Get(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	container, err := h.svc.Get(c.Request().Context(), projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, containerDetailJSON(container))
}

func (h *ContainerHandler) BuildDeploy(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	userID := c.Get("UserID").(string)

	var req struct {
		Name         string            `json:"name"`
		GitRepo      string            `json:"git_repo"`
		GitBranch    string            `json:"git_branch"`
		GitCommit    string            `json:"git_commit"`
		Subdir       string            `json:"git_subdir"`
		ResourceSize string            `json:"resource_size"`
		Replicas     int               `json:"replicas"`
		EnvVars      []envVarInputJSON `json:"env_vars"`
		Ports        []portInputJSON   `json:"ports"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	envVars := make([]service.EnvVarInput, len(req.EnvVars))
	for i, v := range req.EnvVars {
		envVars[i] = service.EnvVarInput{Key: v.Key, Value: v.Value}
	}
	ports := make([]service.PortInput, len(req.Ports))
	for i, p := range req.Ports {
		ports[i] = service.PortInput{Port: p.Port, Protocol: p.Protocol}
	}

	container, workflowID, err := h.svc.BuildDeploy(c.Request().Context(), projectID, userID, service.BuildDeployRequest{
		Name:         req.Name,
		GitRepo:      req.GitRepo,
		GitBranch:    req.GitBranch,
		GitCommit:    req.GitCommit,
		Subdir:       req.Subdir,
		ResourceSize: req.ResourceSize,
		Replicas:     req.Replicas,
		EnvVars:      envVars,
		Ports:        ports,
	})
	if err != nil {
		return response.Error(c, err)
	}
	_ = workflowID

	return response.Created(c, containerSummaryJSON(container))
}

func (h *ContainerHandler) DeployImage(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	userID := c.Get("UserID").(string)

	var req struct {
		Name         string            `json:"name"`
		Image        string            `json:"image"`
		ResourceSize string            `json:"resource_size"`
		Replicas     int               `json:"replicas"`
		EnvVars      []envVarInputJSON `json:"env_vars"`
		Ports        []portInputJSON   `json:"ports"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}
	if req.Name == "" || req.Image == "" {
		return badRequest(c, "name and image are required")
	}

	envVars := make([]service.EnvVarInput, len(req.EnvVars))
	for i, v := range req.EnvVars {
		envVars[i] = service.EnvVarInput{Key: v.Key, Value: v.Value}
	}
	ports := make([]service.PortInput, len(req.Ports))
	for i, p := range req.Ports {
		ports[i] = service.PortInput{Port: p.Port, Protocol: p.Protocol}
	}

	container, workflowID, err := h.svc.DeployImage(c.Request().Context(), projectID, userID, service.ImageDeployRequest{
		Name:         req.Name,
		Image:        req.Image,
		ResourceSize: req.ResourceSize,
		Replicas:     req.Replicas,
		EnvVars:      envVars,
		Ports:        ports,
	})
	if err != nil {
		return response.Error(c, err)
	}
	_ = workflowID

	return response.Created(c, containerSummaryJSON(container))
}

func (h *ContainerHandler) FromTemplate(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	userID := c.Get("UserID").(string)

	var req struct {
		Name         string            `json:"name"`
		TemplateName string            `json:"template_name"`
		ResourceSize string            `json:"resource_size"`
		Replicas     int               `json:"replicas"`
		Params       map[string]string `json:"params"`
		VolumeID     *string           `json:"volume_id"`
		MountPath    *string           `json:"mount_path"`
		CreateVolume bool              `json:"create_volume"`
		VolumeSize   int               `json:"volume_size"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	treq := service.TemplateDeployRequest{
		Name:         req.Name,
		TemplateName: req.TemplateName,
		ResourceSize: req.ResourceSize,
		Replicas:     req.Replicas,
		Params:       req.Params,
		MountPath:    req.MountPath,
		CreateVolume: req.CreateVolume,
		VolumeSize:   req.VolumeSize,
	}
	if req.VolumeID != nil {
		vid, err := uuid.Parse(*req.VolumeID)
		if err != nil {
			return badRequest(c, "invalid volume_id")
		}
		treq.VolumeID = &vid
	}

	container, workflowID, err := h.svc.DeployFromTemplate(c.Request().Context(), projectID, userID, treq)
	if err != nil {
		return response.Error(c, err)
	}
	_ = workflowID

	return response.Created(c, containerSummaryJSON(container))
}

func (h *ContainerHandler) Update(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}
	userID := c.Get("UserID").(string)

	var req struct {
		ResourceSize string `json:"resource_size"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	if err := h.svc.Update(c.Request().Context(), projectID, containerID, userID, req.ResourceSize); err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]interface{}{})
}

func (h *ContainerHandler) Delete(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	workflowID, err := h.svc.Delete(c.Request().Context(), projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *ContainerHandler) Redeploy(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	workflowID, err := h.svc.Redeploy(c.Request().Context(), projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *ContainerHandler) Scale(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	var req struct {
		Replicas int `json:"replicas"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	// レプリカ数を検証する
	if req.Replicas < config.GetMinReplicas() || req.Replicas > config.GetMaxReplicas() {
		return badRequest(c, "invalid replicas")
	}

	workflowID, err := h.svc.Scale(c.Request().Context(), projectID, containerID, req.Replicas)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *ContainerHandler) Rebuild(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	workflowID, err := h.svc.Rebuild(c.Request().Context(), projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *ContainerHandler) CreateWebhook(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	webhookURL, token, err := h.svc.CreateWebhook(c.Request().Context(), projectID, containerID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.Created(c, map[string]string{
		"webhook_url": webhookURL,
		"token":       token,
	})
}

func (h *ContainerHandler) GetStatusHistories(c *echo.Context) error {
	containerID, err := uuid.Parse(c.Param("container_id"))
	if err != nil {
		return badRequest(c, "invalid container_id")
	}

	histories, err := h.statusHistRepo.FindByContainerID(c.Request().Context(), containerID)
	if err != nil {
		return response.Error(c, err)
	}

	result := make([]map[string]interface{}, len(histories))
	for i, h := range histories {
		result[i] = map[string]interface{}{
			"id":              h.ID.String(),
			"status":          h.Status,
			"replicas":        h.Replicas,
			"ready_replicas":  h.ReadyReplicas,
			"failed_replicas": h.FailedReplicas,
			"created_at":      h.CreatedAt,
		}
	}
	return response.OK(c, result)
}

// ---- レスポンス型 ----

func containerSummaryJSON(c *model.Container) map[string]interface{} {
	pods := buildPodsJSON(c)
	return map[string]interface{}{
		"id":                        c.ID.String(),
		"name":                      c.Name,
		"status":                    c.Status,
		"replicas":                  c.Replicas,
		"ready_replicas":            c.ReadyReplicas,
		"failed_replicas":           c.FailedReplicas,
		"resource_size":             c.ResourceSize,
		"active_deploy_workflow_id": c.ActiveDeployWorkflowID,
		"active_scale_workflow_id":  c.ActiveScaleWorkflowID,
		"is_template":               c.IsTemplate,
		"is_image_deploy":           c.IsImageDeploy,
		"image_ref":                 c.ImageRef,
		"pods":                      pods,
		"created_at":                c.CreatedAt,
		"updated_at":                c.UpdatedAt,
	}
}

func containerDetailJSON(c *model.Container) map[string]interface{} {
	m := containerSummaryJSON(c)
	m["git_repo"] = c.GitRepo
	m["git_branch"] = c.GitBranch
	m["git_subdir"] = c.GitSubdir
	envVars := make([]map[string]interface{}, len(c.EnvVars))
	for i, e := range c.EnvVars {
		envVars[i] = map[string]interface{}{"id": e.ID.String(), "key": e.Key, "value": e.Value}
	}
	m["env_vars"] = envVars
	ports := make([]map[string]interface{}, len(c.Ports))
	for i, p := range c.Ports {
		ports[i] = map[string]interface{}{"id": p.ID.String(), "port": p.Port, "protocol": p.Protocol}
	}
	m["ports"] = ports
	routes := make([]map[string]interface{}, len(c.Routes))
	for i, r := range c.Routes {
		routes[i] = map[string]interface{}{
			"id": r.ID.String(), "type": r.Type, "port": r.Port,
			"protocol": r.Protocol, "subdomain": r.Subdomain, "created_at": r.CreatedAt,
		}
	}
	m["routes"] = routes
	mounts := []interface{}{}
	m["mounts"] = mounts
	return m
}

func buildPodsJSON(c *model.Container) []map[string]interface{} {
	pods := make([]map[string]interface{}, len(c.PodStatuses))
	for i, p := range c.PodStatuses {
		pods[i] = map[string]interface{}{
			"pod_name":      p.PodName,
			"status":        p.Status,
			"ready":         p.Ready,
			"restart_count": p.RestartCount,
			"node_name":     p.NodeName,
			"started_at":    p.StartedAt,
		}
	}
	return pods
}

// ---- 共通ヘルパー ----

type envVarInputJSON struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

type portInputJSON struct {
	Port     int    `json:"port"`
	Protocol string `json:"protocol"`
}

func badRequest(c *echo.Context, msg string) error {
	return c.JSON(http.StatusBadRequest, map[string]interface{}{
		"data":  nil,
		"error": map[string]string{"code": "BAD_REQUEST", "message": msg},
	})
}
