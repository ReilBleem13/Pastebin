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
		c.JSON(500, gin.H{"error": "internal server error"})
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
	result, err := h.servises.Pasta.PaginateOnlyPublic(ctx, rawLimit, rawPage, hasMetadata)
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

	result, err := h.servises.Pasta.PaginateForUserByID(ctx, rawLimit, rawPage, userID, hasMetadata)
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
		if errors.Is(err, customerrors.ErrEmptySearchField) {
			c.JSON(404, gin.H{"status": "empty search field"})
		} else if errors.Is(err, customerrors.ErrEmptySearchResult) {
			c.JSON(200, gin.H{"status": "result is empty"})
		} else {
			h.logger.Errorf("internal server error during searching: %v", err)
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

func (h *Handler) CreateFavorite(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() CreateFavoriteHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	ctx := c.Request.Context()

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

	err = h.servises.Pasta.Favorite(ctx, hash, visibility, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrNotAllowed) {
			c.JSON(400, gin.H{"error": "not allowed to add private pasta"})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": "pasta not found"})
		} else {
			h.logger.Errorf("internal server error during create favorite: %v", err)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(200, gin.H{"status": "successfully created"})
}

func (h *Handler) GetFavorite(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() GetFavoriteHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	ctx := c.Request.Context()

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	rawFavoriteID := c.Param("favorite_id")
	favoriteID, err := strconv.Atoi(rawFavoriteID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid param"})
		return
	}

	withMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}

	pasta, err := h.servises.Pasta.GetFavorite(ctx, userID, favoriteID, withMetadata)
	if err != nil {
		if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": "pasta not found"})
		} else if errors.Is(err, customerrors.ErrNotAllowed) {
			c.JSON(400, gin.H{"error": "request from invalid user"})
		} else {
			h.logger.Errorf("internal server error during get favorite: %v", err)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(200, dto.GetPastaResponse{
		Status:   200,
		Message:  "Successfully got pasta",
		Text:     pasta.Text,
		Metadata: pasta.Metadata,
	})
}

func (h *Handler) DeleteFavorite(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() DeleteFavoriteHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	ctx := c.Request.Context()

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	rawFavoriteID := c.Param("favorite_id")
	favoriteID, err := strconv.Atoi(rawFavoriteID)
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid param"})
		return
	}

	err = h.servises.Pasta.DeleteFavorite(ctx, userID, favoriteID)
	if err != nil {
		if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": "pasta not found"})
		} else if errors.Is(err, customerrors.ErrNotAllowed) {
			c.JSON(400, gin.H{"error": "request from invalid user"})
		} else {
			h.logger.Errorf("internal server error during delete favorite: %v", err)
			c.JSON(500, gin.H{"error": "internal server error"})
		}
		return
	}
	c.JSON(200, gin.H{"status": "successfully deleted"})
}

func (h *Handler) PaginateFavorites(c *gin.Context) {
	start := time.Now()
	defer func() {
		h.logger.Tracef("func() PaginateFavoriteHandler. Execution time: %.2f", time.Since(start).Seconds())
	}()

	ctx := c.Request.Context()

	rawLimit := c.Query("limit")
	rawPage := c.Query("page")

	withMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Errorf("internal server error during getting userID from context: %v", err)
		c.JSON(500, gin.H{"error": "internal server error"})
		return
	}

	paginatedPasta, err := h.servises.Pasta.PaginateFavorite(ctx, rawLimit, rawPage, userID, withMetadata)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"err": err.Error()})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"err": customerrors.ErrPastaNotFound.Error()})
		} else {
			h.logger.Errorf("internal server error during paginate favorite: %v", err)
			c.JSON(500, gin.H{"err": "internal server error"})
		}
		return
	}
	c.JSON(200, paginatedPasta)
}
