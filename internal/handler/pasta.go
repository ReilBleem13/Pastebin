package handler

import (
	"errors"
	"log"
	customerrors "pastebin/internal/errors"
	"pastebin/pkg/dto"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	urlForGet = "http://localhost:8080/receive/"
)

func (h *Handler) CreatePastaHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() CreatePastaHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	req, err := h.GetRequest(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting request from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	ctx := c.Request.Context()
	pasta, err := h.servises.Pasta.Create(ctx, req, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidExpirationFormat) ||
			errors.Is(err, customerrors.ErrInvalidLanguageFormat) ||
			errors.Is(err, customerrors.ErrInvalidVisibilityFormat) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		h.logger.Errorf("internal server error during pasta creation: %v", err)
		c.JSON(500, gin.H{"error": "internal error occured while creating the pasta"})
		return
	}

	h.logger.Infof("User:%d successfully uploaded file", userID)
	c.JSON(201, dto.SuccessCreatePastaResponse{
		Status:  201,
		Message: "File uploaded successfully",
		Link:    urlForGet + pasta.Hash,
		Metadata: dto.PastaMetadataDTO{
			Key:        pasta.ObjectID,
			Size:       pasta.Size,
			Language:   pasta.Language,
			Visibility: pasta.Visibility,
			CreatedAt:  pasta.CreatedAt,
			ExpiresAt:  pasta.ExpiresAt,
		},
	})
}

func (h *Handler) GetPastaHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() GetPastaHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	log.Println(1)
	var passwordRequest dto.Password
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.BindJSON(&passwordRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	log.Println(2)
	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	log.Println(3)
	visibility, err := h.GetVisibility(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting visibility from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	log.Println(4)
	hash := c.Param("objectID")

	hasMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}
	log.Println(hasMetadata)
	ctx := c.Request.Context()
	log.Println(5)
	err = h.servises.Pasta.Permission(ctx, hash, passwordRequest.Password, visibility, userID)
	if errors.Is(err, customerrors.ErrPastaNotFound) {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	} else if errors.Is(err, customerrors.ErrNoAccess) ||
		errors.Is(err, customerrors.ErrWrongPassword) {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		h.logger.Errorf("internal server error during checking permission: %v", err)
		c.JSON(500, gin.H{"error": "internal error occured while checking permission"})
		return
	}
	log.Println(6)
	pasta, err := h.servises.Pasta.Get(ctx, hash, hasMetadata)
	if err != nil {
		h.logger.Errorf("internal server error during getting creation: %v", err)
		c.JSON(500, gin.H{"error": "internal error occured while getting the pasta"})
		return
	}
	log.Println(7)
	c.JSON(200, dto.GetPastaResponse{
		Status:   200,
		Message:  "Successfully got pasta",
		Text:     pasta.Text,
		Metadata: pasta.Metadata,
	})
}

func (h *Handler) DeletePastaHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() DeletePastaHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	var passwordRequest dto.Password

	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.BindJSON(&passwordRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	visibility, err := h.GetVisibility(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting visibility from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	hash := c.Param("objectID")
	ctx := c.Request.Context()

	err = h.servises.Pasta.Permission(ctx, hash, passwordRequest.Password, visibility, userID)
	if errors.Is(err, customerrors.ErrPastaNotFound) {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	} else if errors.Is(err, customerrors.ErrNoAccess) ||
		errors.Is(err, customerrors.ErrWrongPassword) {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		h.logger.Errorf("internal server error during checking permission: %v", err)
		c.JSON(500, gin.H{"error": "internal error occured while checking permission"})
		return
	}

	if err := h.servises.Pasta.Delete(ctx, hash); err != nil {
		if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": "pasta not found"})
		} else {
			h.logger.Errorf("internal server error during deleting pasta: %v", err)
			c.JSON(500, gin.H{"error": "internal error occured while deleting pasta"})
		}
		return
	}
	c.JSON(200, gin.H{"status": "deleted"})
}

func (h *Handler) PaginatePublicHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() PaginatePublicHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	rawMaxObject := c.Query("limit")
	rawStartAfter := c.Query("starting")

	ctx := c.Request.Context()
	result, nextKey, err := h.servises.Pasta.Paginate(ctx, rawMaxObject, rawStartAfter, nil)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"error": "invalid request, limit should be more than 5."})
		} else {
			h.logger.Errorf("internal server error during paginating public: %v", err)
			c.JSON(500, gin.H{"error": "internal error occured while paginating public"})
		}
		return
	}

	c.JSON(200, dto.PaginatedPastaDTO{
		Status:       200,
		NextObjectID: nextKey,
		Pastas:       *result,
	})
}

func (h *Handler) PaginateUserIdHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() PaginateUserIdHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	rawLimit := c.Query("limit")
	startAfter := c.Query("starting")

	ctx := c.Request.Context()

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	result, nextKey, err := h.servises.Pasta.Paginate(ctx, rawLimit, startAfter, &userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"error": "invalid request, limit should be more than 5."})
		} else {
			h.logger.Errorf("internal server error during paginating by ID: %v", err)
			c.JSON(500, gin.H{"error": "internal error occured while paginating by id"})
		}
		return
	}
	c.JSON(200, dto.PaginatedPastaDTO{
		Status:       200,
		NextObjectID: nextKey,
		Pastas:       *result,
	})
}
