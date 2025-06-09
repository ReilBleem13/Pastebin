package handler

import (
	"context"
	"log"
	"net/http"
	"pastebin/internal/models"
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

const (
	urlForGet = "http://localhost:8080/files/"
)

/*
Добавьте поддержку дополнительных параметров в запросе:
language (например, "python", "javascript").
visibility ("public", "private", "password").
password (для защищенных паст).
expiration (например, "1h", "1d", "never").


Подумайте о добавлении поля views (количество просмотров) в метаданные.
expired_at можно сделать опциональным (например, null для паст без срока хранения).
*/

func (h *Handler) CreatePastaHandler(c *gin.Context) {
	start := time.Now()

	req, err := h.GetRequest(c)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   err.Error(),
			Details: err,
		})
		return
	}

	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   err.Error(),
			Details: err,
		})
		return
	}

	reqData := []byte(req.Message)
	ctx := context.Background()
	pasta, err := h.servises.Minio.CreateOne(ctx, reqData)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	pasta.UserID = userID
	err = h.servises.DBMinio.CreatePasta(req, &pasta)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, SuccessResponse{
		Status:   201,
		Message:  "File uploaded successfully",
		Data:     urlForGet + pasta.Hash,
		Metadata: pasta,
	})
	log.Println(time.Since(start).Seconds())
}

func (h *Handler) GetRawPastaHandler(c *gin.Context) {

}

func (h *Handler) GetPastaHandler(c *gin.Context) {
	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   err.Error(),
			Details: err,
		})
		return
	}

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

	if err := h.servises.DBMinio.GetPastaByUserID(userID, hash); err != nil {
		c.JSON(500, ErrorResponse{
			Status:  500,
			Error:   "failed check pasta",
			Details: err,
		})
		return
	}

	var data models.PasteWithData
	data.Metadata.Hash = hash
	data.Metadata.UserID = userID

	ctx := context.Background()
	err = h.servises.GetOne(ctx, &data, flag)
	if err != nil {
		c.JSON(500, ErrorResponse{
			Status:  502,
			Error:   "Enable to get the object",
			Details: err,
		})
		return
	}

	if flag {
		c.JSON(http.StatusOK, SuccessGetResponse{
			Status:  http.StatusOK,
			Message: "File received successfully",
			Data:    data,
		})
	} else {
		c.JSON(http.StatusOK, SuccessGetResponse{
			Status:  http.StatusOK,
			Message: "File received successfully",
			Data:    data.Text,
		})
	}
}
