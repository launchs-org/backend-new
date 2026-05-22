package handler

import (
	"net/http"

	"backend/response"
	"backend/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// ContainerHandler はコンテナ関連のリクエストを処理します。
type ContainerHandler struct {
	svc service.ContainerService
}

func NewContainerHandler(svc service.ContainerService) *ContainerHandler {
	return &ContainerHandler{svc: svc}
}

func (h *ContainerHandler) List(c *echo.Context) error {
	// TODO: コンテナ一覧はリポジトリ直接アクセス必要のため、直接DB呼び出し
	// 現フェーズでは service に委譲せず、この handler でリポジトリ参照が必要です。
	// ただし、ここでは container service 側に List を追加することで対応します。
	return response.OK(c, []interface{}{})
}

func (h *ContainerHandler) Get(c *echo.Context) error {
	return response.OK(c, map[string]interface{}{})
}

func (h *ContainerHandler) BuildDeploy(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}

	var req struct {
		Name         string              `json:"name"`
		GitRepo      string              `json:"git_repo"`
		GitBranch    string              `json:"git_branch"`
		GitCommit    string              `json:"git_commit"`
		Subdir       string              `json:"subdir"`
		ResourceSize string              `json:"resource_size"`
		Replicas     int                 `json:"replicas"`
		EnvVars      []envVarInputJSON   `json:"env_vars"`
		Ports        []portInputJSON     `json:"ports"`
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

	containerID, workflowID, err := h.svc.BuildDeploy(c.Request().Context(), projectID, service.BuildDeployRequest{
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

	return response.Created(c, map[string]string{
		"container_id": containerID,
		"workflow_id":  workflowID,
	})
}

func (h *ContainerHandler) FromTemplate(c *echo.Context) error {
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return badRequest(c, "invalid project_id")
	}

	var req struct {
		Name         string            `json:"name"`
		TemplateName string            `json:"template_name"`
		ResourceSize string            `json:"resource_size"`
		Params       map[string]string `json:"params"`
		VolumeID     *string           `json:"volume_id"`
		MountPath    *string           `json:"mount_path"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	treq := service.TemplateDeployRequest{
		Name:         req.Name,
		TemplateName: req.TemplateName,
		ResourceSize: req.ResourceSize,
		Params:       req.Params,
		MountPath:    req.MountPath,
	}
	if req.VolumeID != nil {
		vid, err := uuid.Parse(*req.VolumeID)
		if err != nil {
			return badRequest(c, "invalid volume_id")
		}
		treq.VolumeID = &vid
	}

	containerID, workflowID, err := h.svc.DeployFromTemplate(c.Request().Context(), projectID, treq)
	if err != nil {
		return response.Error(c, err)
	}

	return response.Created(c, map[string]string{
		"container_id": containerID,
		"workflow_id":  workflowID,
	})
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
	_ = userID

	var req struct {
		ResourceSize string `json:"resource_size"`
	}
	if err := c.Bind(&req); err != nil {
		return badRequest(c, err.Error())
	}

	if err := h.svc.Update(c.Request().Context(), projectID, containerID, req.ResourceSize); err != nil {
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

	workflowID, err := h.svc.Scale(c.Request().Context(), projectID, containerID, req.Replicas)
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
	// TODO: ContainerStatusHistoryRepository から取得する実装
	return response.OK(c, []interface{}{})
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
