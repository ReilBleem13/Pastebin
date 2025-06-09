package handler

import (
	"context"
	"log"
	"net/http"
	"pastebin/pkg/models"
	"time"

	"github.com/gin-gonic/gin"
)

type ErrorResponse struct {
	Error   string      `json:"error"`
	Status  int         `json:"code,omitempty"`
	Details interface{} `json:"details,omitempty"`
}

type SuccessResponse struct {
	Status   int          `json:"status"`
	Message  string       `json:"message"`
	Data     interface{}  `json:"data,omitempty"`
	Metadata models.Paste `json:"metadata,omitempty"`
}

type SuccessGetResponse struct {
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
	start := time.Now()
	var req request
	if err := c.BindJSON(&req); err != nil {
		c.String(400, err.Error())
		return
	}
	reqData := []byte(req.Message)

	ctx := context.Background()
	pasta, err := h.servises.Minio.CreateOne(ctx, reqData)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   "Unable to save the file",
			Details: err,
		})
		return
	}

	err = h.servises.DBMinio.CreatePasta(pasta)
	if err != nil {
		c.JSON(500, gin.H{"error": err})
		return
	}

	c.JSON(http.StatusOK, SuccessResponse{
		Status:   http.StatusOK,
		Message:  "File uploaded successfully",
		Data:     urlForGet + pasta.Hash,
		Metadata: pasta,
	})
	log.Println(time.Since(start).Seconds())
}

func (h *Handler) GetOne(c *gin.Context) {
	hash := c.Param("objectID")
	metadata := c.Query("metadata")

	var flag bool
	if metadata != "" {
		if metadata == "true" {
			flag = true
		} else {
			c.JSON(400, ErrorResponse{
				Status: 400,
				Error:  "Invalid query format",
			})
			return
		}
	}

	objectID, err := h.servises.DBMinio.GetLink(hash)
	if err != nil {
		c.JSON(500, gin.H{"err": err})
		return
	}

	ctx := context.Background()

	var data models.PasteWithData
	data.Metadata.Hash = hash
	data.Metadata.StorageKey = objectID

	err = h.servises.GetOne(ctx, &data, flag)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  502,
			Error:   "Enable to get the object",
			Details: err,
		})
		return
	}

	c.JSON(http.StatusOK, SuccessGetResponse{
		Status:  http.StatusOK,
		Message: "File received successfully",
		Data:    data,
	})
}
