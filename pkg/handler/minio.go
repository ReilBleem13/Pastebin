package handler

import (
	"crypto/sha256"
	"encoding/json"
	"net/http"
	"pastebin/pkg/helpers"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string      `json:"error"`
	Status  int         `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

type SuccessResponse struct {
	Status  int         `json:"status"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

type request struct {
	Message string `json:"message"`
}

func (h *Handler) CreateOne(c *gin.Context) {
	var req request
	if err := c.BindJSON(&req); err != nil {
		c.String(400, err.Error())
		return
	}

	reqData, err := json.Marshal(req)
	if err != nil {
		c.String(400, err.Error())
	}

	fileData := helpers.FileDataType{
		FileName: "first-file",
		Data:     reqData,
	}

	link, err := h.servises.Minio.CreateOne(fileData)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   "Unable to save the file",
			Details: err,
		})
		return
	}

	h := sha256.New()

	err = h.servises.DBMinio.CreateLink(objectID, link)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  501,
			Error:   "Unable to save the file",
			Details: err,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  http.StatusOK,
		Message: "File uploaded successfully",
		Data:    objectID,
	})
}

func (h *Handler) GetOne(c *gin.Context) {
	objectID := c.Param("objectID")

	link, err := h.servises.GetOne(objectID)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   "Enable to get the object",
			Details: err,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  http.StatusOK,
		Message: "File received successfully",
		Data:    link,
	})
}
