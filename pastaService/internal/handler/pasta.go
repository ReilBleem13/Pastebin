package handler

import (
	"errors"
	customerrors "pastebin/internal/errors"
	"pastebin/pkg/dto"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	urlForGet = "http://localhost:10002/receive/"
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

	var passwordRequest dto.Password
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.BindJSON(&passwordRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	visibility, err := h.GetVisibility(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting visibility from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}
	h.logger.Info("before hash")
	hash := c.Param("hash")
	h.logger.Infof("after hash: %s", hash)

	hasMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}
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

	pasta, err := h.servises.Pasta.Get(ctx, hash, hasMetadata)
	if err != nil {
		h.logger.Errorf("internal server error during getting creation: %v", err)
		c.JSON(500, gin.H{"error": "internal error occured while getting the pasta"})
		return
	}

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

	hash := c.Param("hash")
	ctx := c.Request.Context()

	err = h.servises.Pasta.Permission(ctx, hash, passwordRequest.Password, visibility, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": err.Error()})
		} else if errors.Is(err, customerrors.ErrNoAccess) ||
			errors.Is(err, customerrors.ErrWrongPassword) {
			c.JSON(403, gin.H{"error": err.Error()})
		} else {
			h.logger.Errorf("internal server error during checking permission: %v", err)
			c.JSON(500, gin.H{"error": "internal error occured while checking permission"})
		}
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

	rawLimit := c.Query("limit")
	rawPage := c.Query("page")

	hasMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}

	ctx := c.Request.Context()
	result, err := h.servises.Pasta.Paginate(ctx, rawLimit, rawPage, hasMetadata, nil)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"error": "invalid request, limit should be more than 5 and page more than 0."})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(200, gin.H{"status": "notes not found"})
		} else {
			h.logger.Errorf("internal server error during paginating public: %v", err)
			c.JSON(500, gin.H{"error": "internal error occured while paginating public"})
		}
		return
	}
	result.Status = 200
	c.JSON(200, result)
}

func (h *Handler) PaginateForUserHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() PaginateForUserHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	rawLimit := c.Query("limit")
	rawPage := c.Query("page")

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	hasMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}
	ctx := c.Request.Context()

	result, err := h.servises.Pasta.Paginate(ctx, rawLimit, rawPage, hasMetadata, &userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"error": "invalid request, limit should be more than 5 and page more than 0."})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(200, gin.H{"status": "notes not found"})
		} else {
			h.logger.Errorf("internal server error during paginating by userID: %v", err)
			c.JSON(500, gin.H{"error": "internal error occured while paginating by userID"})
		}
		return
	}

	result.Status = 200
	c.JSON(200, result)
}

func (h *Handler) SearchHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() SearchHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	word := c.Query("search")
	ctx := c.Request.Context()

	result, err := h.servises.Pasta.Search(ctx, word)
	if err != nil {
		if errors.Is(err, customerrors.ErrEmptySearch) {
			c.JSON(404, gin.H{"status": "not found"})
		} else {
			c.JSON(500, gin.H{"status": "internal server error"})
		}
		return
	}

	c.JSON(200, dto.SearchedPastas{
		Status: 200,
		Pastas: result,
	})
}

func (h *Handler) UpdateHandler(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() UpdateHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	var updateRequest dto.UpdateRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
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

	hash := c.Param("hash")

	ctx := c.Request.Context()

	err = h.servises.Pasta.Permission(ctx, hash, updateRequest.Password, visibility, userID)
	if errors.Is(err, customerrors.ErrPastaNotFound) {
		c.JSON(404, gin.H{"error": err.Error()})
		return
	} else if errors.Is(err, customerrors.ErrNoAccess) ||
		errors.Is(err, customerrors.ErrWrongPassword) {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		h.logger.Errorf("internal server error during checking permission: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	metadata, err := h.servises.Pasta.Update(ctx, []byte(updateRequest.NewText), hash)
	if err != nil {
		h.logger.Errorf("internal server error duting updating: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
	}

	c.JSON(200, dto.SuccessUpdatedPastaResponse{
		Status:   200,
		Message:  "Successfully updated",
		Metadata: *metadata,
	})
}
