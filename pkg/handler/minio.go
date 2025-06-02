package handler

import (
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

	link, err := h.servises.CreateOne(fileData)
	if err != nil {
		// Если не удается сохранить файл, возвращаем ошибку с соответствующим статусом и сообщением
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Status:  http.StatusInternalServerError,
			Error:   "Unable to save the file",
			Details: err,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  http.StatusOK,
		Message: "File uploaded successfully",
		Data:    link, // URL-адрес загруженного файла
	})
}

func (h *Handler) GetOne(c *gin.Context) {}
