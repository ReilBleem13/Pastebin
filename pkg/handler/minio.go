package handler

import (
	"crypto/sha256"
	"encoding/hex"
	"log"
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

const (
	urlForGet = "http://localhost:8080/files/"
)

func (h *Handler) CreateOne(c *gin.Context) {
	var req request
	if err := c.BindJSON(&req); err != nil {
		c.String(400, err.Error())
		return
	}

	log.Println("1::", req.Message)
	reqData := []byte(req.Message)
	log.Println("1::", reqData)

	fileData := helpers.FileDataType{
		FileName: "first-file",
		Data:     reqData,
	}

	objectID, err := h.servises.Minio.CreateOne(fileData)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   "Unable to save the file",
			Details: err,
		})
		return
	}

	hash := sha256.Sum256([]byte(objectID))
	hashStr := hex.EncodeToString(hash[:])

	err = h.servises.DBMinio.CreateLink(objectID, hashStr)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   "Unable to save the file",
			Details: err,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:  http.StatusOK,
		Message: "File uploaded successfully",
		Data:    urlForGet + hashStr,
	})
}

func (h *Handler) GetOne(c *gin.Context) {
	hash := c.Param("objectID")

	objectID, err := h.servises.DBMinio.GetLink(hash)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  501,
			Error:   "Enable to get objectdID",
			Details: err,
		})
	}

	link, err := h.servises.GetOne(objectID)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  502,
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
