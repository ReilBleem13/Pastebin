package handler

import (
	"context"
	"log"
	"net/http"
	"pastebin/internal/models"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type SuccessPostResponse struct {
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

func (h *Handler) CreatePastaHandler(c *gin.Context) {
	start := time.Now()

	req, err := h.GetRequest(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	ctx := context.Background()
	pasta, err := h.servises.Minio.CreateOne(ctx, userID, req.Visibility, req.Password, []byte(req.Message))
	if err != nil {
		if strings.Contains(err.Error(), "invalid visibility format") {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	pasta.UserID = userID
	err = h.servises.DBMinio.CreatePasta(req, &pasta)
	if err != nil {
		if strings.Contains(err.Error(), "invalid visibility format") ||
			strings.Contains(err.Error(), "invalid language format") ||
			strings.Contains(err.Error(), "invalid time format") {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, SuccessPostResponse{
		Status:  201,
		Message: "File uploaded successfully",
		Data:    urlForGet + pasta.Hash,
		Metadata: models.Paste{
			Key:        pasta.Key,
			Size:       pasta.Size,
			Language:   pasta.Language,
			Visibility: pasta.Visibility,
			CreatedAt:  pasta.CreatedAt,
			ExpiresAt:  pasta.ExpiresAt,
		},
	})
	log.Printf("CreatePastaHandler. Time: %v", time.Since(start).Seconds())
}

type getPastaRequest struct {
	Password *string `json:"password,omitempty"`
}

func (h *Handler) GetPastaHandler(c *gin.Context) {
	start := time.Now()
	var newRequest getPastaRequest
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		log.Println("Empty request body, proceeding with empty request:", newRequest)
	} else {
		if err := c.BindJSON(&newRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}

	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	visibility, err := h.GetVisibility(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	hash := c.Param("objectID")
	metadata := c.Query("metadata")

	var hasMetadata bool
	if metadata != "" {
		if metadata == "true" {
			hasMetadata = true
		} else {
			c.JSON(400, gin.H{"error": "invalid query format"})
			return
		}
	}

	var needPassword bool
	if visibility == "private" {
		exists, err := h.servises.DBMinio.CheckPrivatePermission(userID, hash)
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
	} else {
		exists, err := h.servises.DBMinio.CheckPublicPermission(hash)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if exists {
			needPassword = exists
		}
	}

	if needPassword {
		if newRequest.Password != nil {
			if err := h.servises.DBMinio.CheckPastaPassword(*newRequest.Password, hash); err != nil {
				if err.Error() == "wrong password" {
					c.JSON(403, gin.H{"error": err.Error()})
					return
				} else {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
			}
		} else {
			c.JSON(403, gin.H{"error": "need password"})
			return
		}
	}

	var data models.PasteWithData
	data.Metadata.Hash = hash
	data.Metadata.UserID = userID
	data.Metadata.Visibility = &visibility

	ctx := context.Background()
	err = h.servises.GetOne(ctx, &data, hasMetadata)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	if hasMetadata {
		log.Printf("GetPastaHandler. Time: %v", time.Since(start).Seconds())
		c.JSON(200, SuccessGetResponse{
			Status:  http.StatusOK,
			Message: "File received successfully",
			Data:    data,
		})
	} else {
		log.Printf("GetPastaHandler. Time: %v", time.Since(start).Seconds())
		c.JSON(200, SuccessGetResponse{
			Status:  http.StatusOK,
			Message: "File received successfully",
			Data:    data.Text,
		})
	}
}

func (h *Handler) DeletePastaHandler(c *gin.Context) {
	var newRequest getPastaRequest
	if c.Request.Body == nil || c.Request.ContentLength == 0 {
		log.Println("Empty request body, proceeding with empty request:", newRequest)
	} else {
		if err := c.BindJSON(&newRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	visibility, err := h.GetVisibility(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	userID, err := h.GetUserID(c)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	hash := c.Param("objectID")

	var needPassword bool
	if visibility == "private" {
		exists, err := h.servises.DBMinio.CheckPrivatePermission(userID, hash)
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
	} else {
		exists, err := h.servises.DBMinio.CheckPublicPermission(hash)
		if err != nil {
			c.JSON(500, gin.H{"error": err.Error()})
			return
		}
		if exists {
			needPassword = exists
		}
	}

	if needPassword {
		if newRequest.Password != nil {
			if err := h.servises.DBMinio.CheckPastaPassword(*newRequest.Password, hash); err != nil {
				if err.Error() == "wrong password" {
					c.JSON(403, gin.H{"error": err.Error()})
					return
				} else {
					c.JSON(500, gin.H{"error": err.Error()})
					return
				}
			}
		} else {
			c.JSON(403, gin.H{"error": "need password"})
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
