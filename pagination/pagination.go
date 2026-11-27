package pagination

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	PageIndex = "page_index"
	PageSize  = "page_size"
)

// GetPageIndexPageSize reads positive pagination values from the query string.
func GetPageIndexPageSize(context *gin.Context) (int, int, error) {
	pageSize, err := strconv.Atoi(context.DefaultQuery(PageSize, "10"))
	if err != nil || pageSize <= 0 {
		return 0, 0, errors.New("page_size must be a positive integer")
	}

	pageIndex, err := strconv.Atoi(context.DefaultQuery(PageIndex, "1"))
	if err != nil || pageIndex <= 0 {
		return 0, 0, errors.New("page_index must be a positive integer")
	}
	return pageIndex, pageSize, nil
}
