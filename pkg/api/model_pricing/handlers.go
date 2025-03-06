package model_pricing

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// GetModelPrices handles the request to get pricing information for a list of models
func (h *Handler) GetModelPrices(c echo.Context) error {
	var req GetPricesRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, err.Error())
	}

	if len(req.Models) == 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "models list cannot be empty")
	}

	response, err := h.service.GetModelPrices(c.Request().Context(), req.Models)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, response)
}

// UpdateModelCosts handles the request to update average costs for all models
func (h *Handler) UpdateModelCosts(c echo.Context) error {
	if err := h.service.UpdateModelCosts(c.Request().Context()); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, UpdateCostsResponse{
		Status: "Model costs updated successfully",
	})
}
