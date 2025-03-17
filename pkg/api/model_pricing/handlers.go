package model_pricing

import (
	"net/http"
	"strconv"

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

// UpdateModelPricingData handles the request to update model pricing data for candlestick charts
func (h *Handler) UpdateModelPricingData(c echo.Context) error {
	count, err := h.service.UpdateModelPricingData(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, UpdatePricingDataResponse{
		Status: "Model pricing data updated successfully",
		Count:  count,
	})
}

// GetModelPricingData handles the request to get model pricing data for candlestick charts
func (h *Handler) GetModelPricingData(c echo.Context) error {
	modelName := c.Param("model_name")
	if modelName == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "model_name is required")
	}

	limitStr := c.QueryParam("limit")
	limit := 60 // Default to 60 minutes of data
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil {
			return echo.NewHTTPError(http.StatusBadRequest, "invalid limit parameter")
		}
	}

	response, err := h.service.GetModelPricingData(c.Request().Context(), modelName, limit)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, response)
}
