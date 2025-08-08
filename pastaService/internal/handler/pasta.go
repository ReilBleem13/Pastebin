package handler

import (
	"errors"
	customerrors "pastebin/internal/errors"
	"pastebin/pkg/dto"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/theartofdevel/logging"
)

const (
	urlForGet = "http://localhost:10002/receive/"
)

func (h *Handler) CreatePastaHandler(c *gin.Context) {
	req, err := h.GetRequest(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting request from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	ctx := c.Request.Context()
	h.logger.Debug("Calling Create Method")
	pasta, err := h.servises.Pasta.Create(ctx, req, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidExpirationFormat) ||
			errors.Is(err, customerrors.ErrInvalidLanguageFormat) ||
			errors.Is(err, customerrors.ErrInvalidVisibilityFormat) {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
		h.logger.Error("Internal server error during pasta creation", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	c.JSON(201, dto.SuccessCreatePastaResponse{
		Status:  201,
		Message: "File uploaded successfully",
		Link:    urlForGet + pasta.Hash,
		Metadata: dto.MetadataResponse{
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
	var passwordRequest dto.Password
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.BindJSON(&passwordRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}
	visibility, err := h.GetVisibility(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting visibility from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}
	hash := c.Param("hash")

	hasMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}
	ctx := c.Request.Context()

	h.logger.Debug("Calling Permission Method")
	err = h.servises.Pasta.Permission(ctx, hash, passwordRequest.Password, visibility, userID, false)
	if errors.Is(err, customerrors.ErrPastaNotFound) {
		c.JSON(404, gin.H{"error": customerrors.ErrPastaNotFound.Error()})
		return
	} else if errors.Is(err, customerrors.ErrNoAccess) ||
		errors.Is(err, customerrors.ErrWrongPassword) {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		h.logger.Error("Internal server error during checking permission", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	h.logger.Debug("Calling Get Method")
	pasta, err := h.servises.Pasta.Get(ctx, hash, hasMetadata, true)
	if err != nil {
		h.logger.Error("Internal server error during getting creation", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
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
	var passwordRequest dto.Password
	if c.Request.Body != nil && c.Request.ContentLength != 0 {
		if err := c.BindJSON(&passwordRequest); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
			return
		}
	}
	visibility, err := h.GetVisibility(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting visibility from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	hash := c.Param("hash")
	ctx := c.Request.Context()

	h.logger.Debug("Calling Permission Method")
	err = h.servises.Pasta.Permission(ctx, hash, passwordRequest.Password, visibility, userID, false)
	if err != nil {
		if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": customerrors.ErrPastaNotFound.Error()})
		} else if errors.Is(err, customerrors.ErrNoAccess) ||
			errors.Is(err, customerrors.ErrWrongPassword) {
			c.JSON(403, gin.H{"error": err.Error()})
		} else {
			h.logger.Error("Internal server error during checking permission", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}

	h.logger.Debug("Calling Delete Method")
	if err := h.servises.Pasta.Delete(ctx, hash); err != nil {
		if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": customerrors.ErrPastaNotFound.Error()})
		} else {
			h.logger.Error("Internal server error during deleting pasta", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}
	c.JSON(200, gin.H{"Status": "Deleted"})
}

func (h *Handler) PaginatePublicHandler(c *gin.Context) {
	rawLimit := c.Query("limit")
	rawPage := c.Query("page")

	hasMetadata, err := strconv.ParseBool(c.DefaultQuery("metadata", "false"))
	if err != nil {
		c.JSON(400, gin.H{"error": "invalid 'metadata' query parameter, must be 'true' of 'false'"})
		return
	}

	ctx := c.Request.Context()
	h.logger.Debug("Calling PaginateOnlyPublic Method")
	result, err := h.servises.Pasta.PaginateOnlyPublic(ctx, rawLimit, rawPage, hasMetadata)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"error": customerrors.ErrInvalidQueryParament.Error()})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(200, gin.H{"status": customerrors.ErrPastaNotFound.Error()})
		} else {
			h.logger.Error("Internal server error during paginating public", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}
	result.Status = 200
	c.JSON(200, result)
}

func (h *Handler) PaginateForUserHandler(c *gin.Context) {
	rawLimit := c.Query("limit")
	rawPage := c.Query("page")

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
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
			c.JSON(400, gin.H{"error": customerrors.ErrInvalidQueryParament.Error()})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(200, gin.H{"status": customerrors.ErrPastaNotFound.Error()})
		} else {
			h.logger.Error("Internal server error during paginating by userID", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}

	result.Status = 200
	c.JSON(200, result)
}

func (h *Handler) SearchHandler(c *gin.Context) {
	word := c.Query("search")
	ctx := c.Request.Context()

	h.logger.Debug("Calling Search Method")
	result, err := h.servises.Pasta.Search(ctx, word)
	if err != nil {
		if errors.Is(err, customerrors.ErrEmptySearchField) {
			c.JSON(400, gin.H{"status": customerrors.ErrEmptySearchField.Error()})
		} else if errors.Is(err, customerrors.ErrEmptySearchResult) {
			c.JSON(200, gin.H{"status": customerrors.ErrEmptySearchResult.Error()})
		} else {
			h.logger.Error("Internal server error during searching", logging.ErrAttr(err))
			c.JSON(500, gin.H{"status": customerrors.ErrInternal.Error()})
		}
		return
	}

	c.JSON(200, dto.SearchedPastas{
		Status: 200,
		Pastas: result,
	})
}

func (h *Handler) UpdateHandler(c *gin.Context) {
	var updateRequest dto.UpdateRequest
	if err := c.ShouldBindJSON(&updateRequest); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	visibility, err := h.GetVisibility(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting visibility from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	hash := c.Param("hash")

	ctx := c.Request.Context()

	h.logger.Debug("Calling Permission Method")
	err = h.servises.Pasta.Permission(ctx, hash, updateRequest.Password, visibility, userID, true)
	if errors.Is(err, customerrors.ErrPastaNotFound) {
		c.JSON(404, gin.H{"error": customerrors.ErrPastaNotFound.Error()})
		return
	} else if errors.Is(err, customerrors.ErrNoAccess) ||
		errors.Is(err, customerrors.ErrWrongPassword) {
		c.JSON(403, gin.H{"error": err.Error()})
		return
	} else if err != nil {
		h.logger.Error("Internal server error during checking permission", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	h.logger.Debug("Calling Update Method")
	metadata, err := h.servises.Pasta.Update(ctx, []byte(updateRequest.NewText), hash)
	if err != nil {
		h.logger.Error("Internal server error duting updating", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
	}

	c.JSON(200, dto.SuccessUpdatedPastaResponse{
		Status:   200,
		Message:  "Successfully updated",
		Metadata: *metadata,
	})
}

func (h *Handler) CreateFavorite(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	hash := c.Param("hash")

	err = h.servises.Pasta.Favorite(ctx, hash, userID)
	if err != nil {
		if errors.Is(err, customerrors.ErrNotAllowed) {
			c.JSON(400, gin.H{"error": customerrors.ErrNotAllowed.Error()})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"error": customerrors.ErrPastaNotFound.Error()})
		} else {
			h.logger.Error("Internal server error during create favorite", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}
	c.JSON(200, gin.H{"Status": "Successfully created"})
}

func (h *Handler) GetFavorite(c *gin.Context) {
	ctx := c.Request.Context()

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
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
		if errors.Is(err, customerrors.ErrNotAllowed) {
			c.JSON(400, gin.H{"error": customerrors.ErrNotAllowed.Error()})
		} else {
			h.logger.Error("Internal server error during get favorite", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
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
	ctx := c.Request.Context()

	userID, err := h.GetUserID(c)
	if errors.Is(err, customerrors.ErrInternal) {
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
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
		if errors.Is(err, customerrors.ErrNotAllowed) {
			c.JSON(400, gin.H{"error": customerrors.ErrNotAllowed.Error()})
		} else {
			h.logger.Error("Internal server error during delete favorite", logging.ErrAttr(err))
			c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		}
		return
	}
	c.JSON(200, gin.H{"Status": "Successfully deleted"})
}

func (h *Handler) PaginateFavorites(c *gin.Context) {
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
		h.logger.Error("Internal server error during getting userID from context", logging.ErrAttr(err))
		c.JSON(500, gin.H{"error": customerrors.ErrInternal.Error()})
		return
	}

	paginatedPasta, err := h.servises.Pasta.PaginateFavorite(ctx, rawLimit, rawPage, userID, withMetadata)
	if err != nil {
		if errors.Is(err, customerrors.ErrInvalidQueryParament) {
			c.JSON(400, gin.H{"err": customerrors.ErrInvalidQueryParament.Error()})
		} else if errors.Is(err, customerrors.ErrPastaNotFound) {
			c.JSON(404, gin.H{"err": customerrors.ErrPastaNotFound.Error()})
		} else {
			h.logger.Error("Internal server error during paginate favorite", logging.ErrAttr(err))
			c.JSON(500, gin.H{"err": customerrors.ErrInternal.Error()})
		}
		return
	}
	h.logger.Debug("paginatedPasta", logging.AnyAttr("result", paginatedPasta))
	c.JSON(200, paginatedPasta)
}
