package handler

import (
	"net/http"
	"time"

	"backend/response"
	"backend/service"
	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

// ProjectHandler はプロジェクト関連のリクエストを処理します。
type ProjectHandler struct {
	svc service.ProjectService
}

func NewProjectHandler(svc service.ProjectService) *ProjectHandler {
	return &ProjectHandler{svc: svc}
}

func (h *ProjectHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projects, err := h.svc.List(c.Request().Context(), userID)
	if err != nil {
		return response.Error(c, err)
	}

	type projectItem struct {
		ID             string     `json:"id"`
		Name           string     `json:"name"`
		Slug           string     `json:"slug"`
		Namespace      string     `json:"namespace"`
		ContainerCount int64      `json:"container_count"`
		LastDeployedAt *time.Time `json:"last_deployed_at"`
		CreatedAt      time.Time  `json:"created_at"`
	}

	items := make([]projectItem, len(projects))
	for i, p := range projects {
		items[i] = projectItem{
			ID:        p.ID.String(),
			Name:      p.Name,
			Slug:      p.Slug,
			Namespace: p.Namespace,
			CreatedAt: p.CreatedAt,
		}
	}
	return response.OK(c, items)
}

func (h *ProjectHandler) Create(c *echo.Context) error {
	userID := c.Get("UserID").(string)

	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"data": nil, "error": map[string]string{"code": "BAD_REQUEST", "message": err.Error()}})
	}

	project, workflowID, err := h.svc.Create(c.Request().Context(), userID, req.Name)
	if err != nil {
		return response.Error(c, err)
	}

	return response.Created(c, map[string]interface{}{
		"id":          project.ID.String(),
		"name":        project.Name,
		"slug":        project.Slug,
		"namespace":   project.Namespace,
		"workflow_id": workflowID,
		"created_at":  project.CreatedAt,
	})
}

func (h *ProjectHandler) Get(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"data": nil, "error": map[string]string{"code": "BAD_REQUEST", "message": "invalid project_id"}})
	}

	project, err := h.svc.Get(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, project)
}

func (h *ProjectHandler) Delete(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"data": nil, "error": map[string]string{"code": "BAD_REQUEST", "message": "invalid project_id"}})
	}

	workflowID, err := h.svc.Delete(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, map[string]string{"workflow_id": workflowID})
}

func (h *ProjectHandler) Deploy(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"data": nil, "error": map[string]string{"code": "BAD_REQUEST", "message": "invalid project_id"}})
	}

	snapshotID, workflowIDs, err := h.svc.DeployAll(c.Request().Context(), userID, projectID)
	if err != nil {
		return response.Error(c, err)
	}

	return response.OK(c, map[string]interface{}{
		"snapshot_id":  snapshotID,
		"workflow_ids": workflowIDs,
	})
}

func (h *ProjectHandler) GetJobs(c *echo.Context) error {
	// Temporal からアクティブなワークフロー一覧を取得する拡張ポイントです。
	// 現在はプレースホルダーとして空配列を返します。
	return response.OK(c, []interface{}{})
}
