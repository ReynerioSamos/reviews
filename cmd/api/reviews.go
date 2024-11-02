package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ReynerioSamos/reviews/internal/data"
	"github.com/ReynerioSamos/reviews/internal/validator"
)

func (a *applicationDependencies) createReviewHandler(w http.ResponseWriter, r *http.Request) {
	var incomingData struct {
		Prod_ID int64 `json:"prod_id"`
		Rating  int8  `json:"rating"`
	}

	err := a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	review := &data.Review{
		Prod_ID: incomingData.Prod_ID,
		Rating:  incomingData.Rating,
	}

	v := validator.New()
	data.ValidateReview(v, review, a.reviewModel)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.reviewModel.Insert(review)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	fmt.Fprintf(w, "%+v\n", incomingData)

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/review/%d", review.RID))

	data := envelope{
		"review": review,
	}
	err = a.writeJson(w, http.StatusCreated, data, headers)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *applicationDependencies) displayReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	review, err := a.reviewModel.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	data := envelope{
		"review": review,
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *applicationDependencies) updateReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	review, err := a.reviewModel.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	var incomingData struct {
		Rating *int8 `json:"rating"`
	}

	err = a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	if incomingData.Rating != nil {
		review.Rating = *incomingData.Rating
	}

	v := validator.New()
	data.ValidateReview(v, review, a.reviewModel)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.reviewModel.Update(review)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	data := envelope{
		"review": review,
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *applicationDependencies) updateHelpfulCountHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	var incomingData struct {
		Increment int8 `json:"increment"`
	}

	err = a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	if incomingData.Increment != 1 && incomingData.Increment != -1 {
		v := validator.New()
		v.AddError("increment", "must be either 1 or -1")
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.reviewModel.UpdateHelpfulCOunt(id, incomingData.Increment)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	data := envelope{
		"message": "helpful count updated successfully",
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *applicationDependencies) deleteReviewHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	err = a.reviewModel.Delete(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	data := envelope{
		"message": "review successfully deleted",
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

func (a *applicationDependencies) ListReviewsHandler(w http.ResponseWriter, r *http.Request) {
	var queryParametersData struct {
		Prod_ID       int
		Rating        int
		Helpful_Count int
		data.Filters
	}

	queryParameters := r.URL.Query()

	prodIDStr := a.getSingleQueryParameter(queryParameters, "prod_id", "")
	if prodIDStr != "" {
		prodID, err := strconv.Atoi(prodIDStr)
		if err != nil {
			v := validator.New()
			v.AddError("prod_id", "must be a valid integer")
			a.failedValidationResponse(w, r, v.Errors)
			return
		}
		queryParametersData.Prod_ID = prodID
	}

	ratingStr := a.getSingleQueryParameter(queryParameters, "rating", "")
	if ratingStr != "" {
		rating, err := strconv.Atoi(ratingStr)
		if err != nil {
			v := validator.New()
			v.AddError("rating", "must be a valid integer")
			a.failedValidationResponse(w, r, v.Errors)
			return
		}
		queryParametersData.Rating = rating
	}

	helpfulStr := a.getSingleQueryParameter(queryParameters, "helpful_count", "")
	if helpfulStr != "" {
		helpful, err := strconv.Atoi(helpfulStr)
		if err != nil {
			v := validator.New()
			v.AddError("helpful_count", "must be a valid integer")
			a.failedValidationResponse(w, r, v.Errors)
			return
		}
		queryParametersData.Helpful_Count = helpful
	}

	v := validator.New()

	queryParametersData.Filters.Page = a.getSingleIntegerParameter(
		queryParameters, "page", 1, v)

	queryParametersData.Filters.PageSize = a.getSingleIntegerParameter(
		queryParameters, "page_size", 10, v)

	queryParametersData.Filters.Sort = a.getSingleQueryParameter(
		queryParameters, "sort", "rid")

	queryParametersData.Filters.SortSafeList = []string{
		"rid", "rating", "helpful_count", "created_at",
		"-rid", "-rating", "-helpful_count", "-created_at",
	}

	data.ValidateFilters(v, queryParametersData.Filters)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	reviews, metadata, err := a.reviewModel.GetAll(
		queryParametersData.Prod_ID,
		queryParametersData.Rating,
		queryParametersData.Helpful_Count,
		queryParametersData.Filters)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	data := envelope{
		"reviews":   reviews,
		"@metadata": metadata,
	}

	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}
