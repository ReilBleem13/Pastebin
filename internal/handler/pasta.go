package handler

import (
	"context"
	"errors"
	"log"
	customerrors "pastebin/internal/errors"
	errorme "pastebin/internal/errors"
	"pastebin/internal/models"
	"time"

	"github.com/gin-gonic/gin"
)

type SuccessPostResponse struct {
	Status   int          `json:"status"`
	Message  string       `json:"message"`
	Data     interface{}  `json:"data,omitempty"`
	Metadata models.Pasta `json:"metadata,omitempty"`
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

	pasta, err := h.servises.Pasta.Create(ctx, req, userID)
	if errors.Is(err, errorme.ErrInvalidExpirationFormat) ||
		errors.Is(err, errorme.ErrInvalidLanguageFormat) ||
		errors.Is(err, errorme.ErrInvalidVisibilityFormat) {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(201, SuccessPostResponse{
		Status:  201,
		Message: "File uploaded successfully",
		Data:    urlForGet + pasta.Hash,
		Metadata: models.Pasta{
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
	Password string `json:"password"`
}

func (h *Handler) GetPastaHandler(c *gin.Context) {
	start := time.Now()
	var newRequest getPastaRequest
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
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
	ctx := context.Background()

	err = h.servises.Pasta.Permission(ctx, hash, newRequest.Password, visibility, userID)
	if errors.Is(err, customerrors.ErrPastaNotFound) {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	} else if errors.Is(err, customerrors.ErrNoAccess) ||
		errors.Is(err, customerrors.ErrWrongPassword) {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	result, err := h.servises.Pasta.Get(ctx, hash, hasMetadata)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"result": result})
	log.Printf("GetPastaHandler. Time: %v", time.Since(start).Seconds())
}

// func (h *Handler) DeletePastaHandler(c *gin.Context) {
// 	var newRequest getPastaRequest
// 	if c.Request.Body == nil || c.Request.ContentLength == 0 {
// 		log.Println("Empty request body, proceeding with empty request:", newRequest)
// 	} else {
// 		if err := c.BindJSON(&newRequest); err != nil {
// 			c.JSON(400, gin.H{"error": err.Error()})
// 			return
// 		}
// 	}
// 	visibility, err := h.GetVisibility(c)
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": err.Error()})
// 		return
// 	}

// 	userID, err := h.GetUserID(c)
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": err.Error()})
// 		return
// 	}

// 	hash := c.Param("objectID")
// 	ctx := context.Background()

// 	var needPassword bool
// 	if visibility == "private" {
// 		exists, err := h.servises.DBMinio.CheckPrivatePermission(ctx, userID, hash)
// 		if err != nil {
// 			if err.Error() == "no rights" {
// 				c.JSON(403, gin.H{"error": err.Error()})
// 				return
// 			} else {
// 				c.JSON(500, gin.H{"error": err.Error()})
// 				return
// 			}
// 		}
// 		if exists {
// 			needPassword = exists
// 		}
// 	} else {
// 		exists, err := h.servises.DBMinio.CheckPublicPermission(ctx, hash)
// 		if err != nil {
// 			c.JSON(500, gin.H{"error": err.Error()})
// 			return
// 		}
// 		if exists {
// 			needPassword = exists
// 		}
// 	}

// 	if needPassword {
// 		if newRequest.Password != nil {
// 			if err := h.servises.DBMinio.CheckPastaPassword(ctx, *newRequest.Password, hash); err != nil {
// 				if err.Error() == "wrong password" {
// 					c.JSON(403, gin.H{"error": err.Error()})
// 					return
// 				} else {
// 					c.JSON(500, gin.H{"error": err.Error()})
// 					return
// 				}
// 			}
// 		} else {
// 			c.JSON(403, gin.H{"error": "need password"})
// 			return
// 		}
// 	}

// 	if err := h.servises.Minio.DeleteOne(ctx, hash); err != nil {
// 		if strings.Contains(err.Error(), "metadata not found for hash") {
// 			c.JSON(404, gin.H{"error": err.Error()})
// 			return
// 		}
// 		c.JSON(500, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(200, gin.H{"status": "deleted"})
// }

// func (h *Handler) PaginatePublicHandler(c *gin.Context) {
// 	maxKeys := c.Query("offset")
// 	startAfter := c.Query("starting_from")

// 	ctx := context.Background()
// 	result, nextKey, err := h.servises.Minio.Paginate(ctx, maxKeys, startAfter, nil)
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(200, gin.H{
// 		"nextKey": nextKey,
// 		"pastas":  result,
// 	})
// }

// func (h *Handler) PaginateUserIdHandler(c *gin.Context) {
// 	ctx := context.Background()
// 	userID, err := h.GetUserID(c)
// 	if err != nil {
// 		c.JSON(400, gin.H{"error": err.Error()})
// 		return
// 	}

// 	maxKeys := c.Query("offset")
// 	startAfter := c.Query("starting_from")

// 	result, nextKey, err := h.servises.Minio.Paginate(ctx, maxKeys, startAfter, &userID)
// 	if err != nil {
// 		c.JSON(500, gin.H{"error": err.Error()})
// 		return
// 	}

// 	c.JSON(200, gin.H{
// 		"nextKey": nextKey,
// 		"pastas":  result,
// 	})
// }
