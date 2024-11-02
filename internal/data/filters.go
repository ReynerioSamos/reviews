package data

import (
	"fmt"
	"strings"

	"github.com/ReynerioSamos/reviews/internal/validator"
)

// if an API does not pagination, it's not an API
// Also rate limiting, sorting, and graceful shutdown are the bare minimum of professional APIs
// The Filters type will contain the fields related to pagination
// and eventually the fields related to sorting

const (
	MaxPageSize     = 100
	MaxPageNumber   = 500
	DefaultPageSize = 20
	DefaultPage     = 1
)

type Filters struct {
	Page         int      `json:"page"`      // which page number does the client want
	PageSize     int      `json:"page_size"` // how records per page
	Sort         string   `json:"sort"`
	SortSafeList []string `json:"sort_safe_list"` //allowed sort fiels
}

type Metadata struct {
	CurrentPage  int `json:"current_page,omitempty"`
	PageSize     int `json:"page_size,omitempty"`
	FirstPage    int `json:"first_page,omitempty"`
	LastPage     int `json:"last_page,omitempty"`
	TotalRecords int `json:"total_records,omitempty"`
	// Some extra flags to check for next and prev pages
	HasNextPage bool `json:"has_next_page,omitempty"`
	HasPrevPage bool `json:"has_prev_page,omitempty"`
	// These help track next and prev pages
	NextPage int `json:"next_page,omitempty"`
	PrevPage int `json:"prev_page,omitempty"`
}

// NewFilters creates a new Filters instance with default values
func NewFilters(sortSafeList []string) Filters {
	return Filters{
		Page:         DefaultPage,
		PageSize:     DefaultPageSize,
		Sort:         sortSafeList[0], //Default to first safe sort option
		SortSafeList: sortSafeList,
	}
}

func ValidateFilters(v *validator.Validator, f Filters) {
	// Validate page number
	v.Check(f.Page > 0, "page", "must be greater than zero")
	v.Check(f.Page <= MaxPageNumber, "page", fmt.Sprintf("must be a maximum of %d", MaxPageNumber))

	// Validate page size
	v.Check(f.PageSize > 0, "page_size", "must be greater than zero")
	v.Check(f.PageSize <= MaxPageSize, "page_size", fmt.Sprintf("must be a maximum of %d", MaxPageSize))

	// Validate sort parameter
	if f.Sort != "" {
		sortField := strings.TrimPrefix(f.Sort, "-")
		v.Check(validator.PermittedValue(sortField, f.SortSafeList...), "sort",
			fmt.Sprintf("must be one of: %s", strings.Join(f.SortSafeList, ", ")))
	}
}

// Implement the sorting feature
func (f Filters) sortColumn() string {

	// Remove the leading hyphen if present
	sortField := strings.TrimPrefix(f.Sort, "-")

	// Validate against safe list
	for _, safeValue := range f.SortSafeList {
		if f.Sort == safeValue {
			return sortField
		}
	}

	// If sort field is not in safe list, we use first safe value
	if len(f.SortSafeList) > 0 {
		return f.SortSafeList[0]
	}

	// don't allow the operation to continue
	// if case of SQL injection attack
	panic("unsafe sort parameter: " + f.Sort)
}

// Get the sort order
func (f Filters) sortDirection() string {
	if strings.HasPrefix(f.Sort, "-") {
		return "DESC"
	}
	return "ASC"
}

// calculate how many records to send back
func (f Filters) limit() int {
	return f.PageSize
}

// calculate the offset so that we remember how many records have been sent
// and how many remail to be sent
func (f Filters) offset() int {
	return (f.Page - 1) * f.PageSize
}

// Calculate the metadata
func calculateMetaData(totalRecords, currentPage, pageSize int) Metadata {
	if totalRecords == 0 {
		return Metadata{}
	}

	lastPage := (totalRecords + pageSize - 1) / pageSize

	metadata := Metadata{
		CurrentPage:  currentPage,
		PageSize:     pageSize,
		FirstPage:    1,
		LastPage:     lastPage,
		TotalRecords: totalRecords,
		HasNextPage:  currentPage < lastPage,
		HasPrevPage:  currentPage > 1,
	}

	// calc next and prev pages
	if metadata.HasNextPage {
		metadata.NextPage = currentPage + 1
	}
	if metadata.HasPrevPage {
		metadata.PrevPage = currentPage - 1
	}

	return metadata
}
