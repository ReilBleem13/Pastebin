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
	urlForGet = "http://localhost:8080/receive/"
)

/*
Подумайте о добавлении поля views (количество просмотров) в метаданные.
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
	pasta, err := h.servises.Minio.CreateOne(ctx, userID, req.Visibility, req.Password, reqData)
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

type request struct {
	Password *string `json:"password,omitempty"`
}

func (h *Handler) GetPastaHandler(c *gin.Context) {
	var newRequest request
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		log.Println("Empty request body, proceeding with empty request:", newRequest)
	} else {
		if err := c.BindJSON(&newRequest); err != nil {
			c.JSON(400, ErrorResponse{
				Status:  400,
				Error:   "Invalid JSON" + err.Error(),
				Details: err,
			})
			return
		}
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

	visibility, err := h.GetVisibility(c)
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

	var needPassword bool
	if visibility == "private" {
		exists, err := h.servises.DBMinio.CheckPrivatePermission(userID, hash)
		if err != nil {
			if err.Error() == "no rights" {
				c.JSON(403, gin.H{"error": err})
				return
			} else {
				c.JSON(500, gin.H{"error": err})
				return
			}
		}
		if exists {
			needPassword = exists
		}
	} else {
		exists, err := h.servises.DBMinio.CheckPublicPermission(hash)
		if err != nil {
			if err.Error() == "no rights" {
				c.JSON(403, gin.H{"error": err})
				return
			} else {
				c.JSON(500, gin.H{"error": err})
				return
			}
		}
		if exists {
			needPassword = exists
		}
	}

	if needPassword {
		if newRequest.Password != nil {
			if err := h.servises.DBMinio.CheckPastaPassword(*newRequest.Password, hash); err != nil {
				if err.Error() == "wrong password" {
					c.JSON(403, gin.H{"error": err}) // не выдает ошибку
					return
				} else {
					c.JSON(400, gin.H{"error": err})
					return
				}
			}
		} else {
			c.JSON(400, gin.H{"error": "need password"})
			return
		}
	}

	var data models.PasteWithData
	data.Metadata.Hash = hash
	data.Metadata.UserID = userID
	data.Metadata.Visibility = &visibility

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

func (h *Handler) DeletePastaHandler(c *gin.Context) {
	var newRequest request
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		log.Println("Empty request body, proceeding with empty request:", newRequest)
	} else {
		if err := c.BindJSON(&newRequest); err != nil {
			c.JSON(400, ErrorResponse{
				Status:  400,
				Error:   "Invalid JSON" + err.Error(),
				Details: err,
			})
			return
		}
	}
	visibility, err := h.GetVisibility(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	hash := c.Param("objectID")

	var needPassword bool
	if visibility == "private" {
		exists, err := h.servises.DBMinio.CheckPrivatePermission(userID, hash)
		if err != nil {
			if err.Error() == "no rights" {
				log.Println("err", err)
				c.JSON(403, gin.H{"error": err.Error()})
				return
			} else {
				log.Println("err", err)
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}
		if exists {
			needPassword = exists
		}
	} else {
		exists, err := h.servises.DBMinio.CheckPublicPermission(hash)
		if err != nil {
			if err.Error() == "no rights" {
				c.JSON(403, gin.H{"error": err.Error()})
				return
			} else {
				c.JSON(500, gin.H{"error": err.Error()})
				return
			}
		}
		if exists {
			needPassword = exists
		}
	}

	if needPassword {
		if newRequest.Password != nil {
			if err := h.servises.DBMinio.CheckPastaPassword(*newRequest.Password, hash); err != nil {
				if err.Error() == "wrong password" {
					log.Println("неверный пароль")
					c.JSON(403, gin.H{"error": err.Error()}) // не выдает ошибку
					return
				} else {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}
			}
		} else {
			log.Println("нужен пароль")
			c.JSON(400, gin.H{"error": "need password"})
			return
		}
	}

	if err := h.servises.Minio.DeleteOne(hash); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"status": "deleted"})
}

func (h *Handler) PaginatePublicHandler(c *gin.Context) {
	maxKeys := c.Query("offset")
	startAfter := c.Query("starting_from")

	result, nextKey, err := h.servises.Minio.Paginate(maxKeys, startAfter, nil)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"nextKey": nextKey,
		"pastas":  result,
	})
}

func (h *Handler) PaginateUserIdHandler(c *gin.Context) {
	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	maxKeys := c.Query("offset")
	startAfter := c.Query("starting_from")

	result, nextKey, err := h.servises.Minio.Paginate(maxKeys, startAfter, &userID)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{
		"nextKey": nextKey,
		"pastas":  result,
	})
}
