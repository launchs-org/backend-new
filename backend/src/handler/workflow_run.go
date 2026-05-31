package handler

import (
	"backend/response"
	"backend/service"

	"github.com/google/uuid"
	"github.com/labstack/echo/v5"
)

type WorkflowRunHandler struct {
	svc service.WorkflowRunService
}

func NewWorkflowRunHandler(svc service.WorkflowRunService) *WorkflowRunHandler {
	return &WorkflowRunHandler{svc: svc}
}

func (h *WorkflowRunHandler) List(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return response.Error(c, err)
	}

	limit := 50
	runs, err := h.svc.List(c.Request().Context(), userID, projectID, limit)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, runs)
}

func (h *WorkflowRunHandler) GetEvents(c *echo.Context) error {
	userID := c.Get("UserID").(string)
	projectID, err := uuid.Parse(c.Param("project_id"))
	if err != nil {
		return response.Error(c, err)
	}
	runID, err := uuid.Parse(c.Param("run_id"))
	if err != nil {
		return response.Error(c, err)
	}
	events, err := h.svc.GetEvents(c.Request().Context(), userID, projectID, runID)
	if err != nil {
		return response.Error(c, err)
	}
	return response.OK(c, events)
}
