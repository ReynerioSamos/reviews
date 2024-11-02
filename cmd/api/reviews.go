package main

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ReynerioSamos/reviews/internal/data"
	"github.com/ReynerioSamos/reviews/internal/validator"
)

// handles Create/Post method
func (a *applicationDependencies) createReviewHandler(w http.ResponseWriter, r *http.Request) {
	// hold the incoming data in a struct
	var incomingData struct {
		Prod_ID int64 `json:"prod_id"`
		Rating  int8  `json:"rating"`
	}

	err := a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	// stores struct data for incommingdata into Review object
	review := &data.Review{
		Prod_ID: incomingData.Prod_ID,
		Rating:  incomingData.Rating,
	}

	// validate fields
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

	// print the review data being created
	fmt.Fprintf(w, "%+v\n", incomingData)

	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/review/%d", review.RID))

	// send it as a json envelope
	data := envelope{
		"review": review,
	}
	err = a.writeJson(w, http.StatusCreated, data, headers)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

// Display/Read functionality
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
	// return read data as an envelope
	data := envelope{
		"review": review,
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

// Update functionality
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

	// temp store data to be updated into a struct
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

	// validate the incoming data
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

	// return the newly updated data
	data := envelope{
		"review": review,
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

// separate update functionality for helpful_counter
// idea was if reviews had something like a text body people wrote too
// this would add functionality to the review to click a simple thumbs up or down button
// to give it +1 count or -1 count respectively
func (a *applicationDependencies) updateHelpfulCountHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	// temp store increment/decrement as a struct
	var incomingData struct {
		Increment int8 `json:"increment"`
	}

	err = a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	// validate and check whether it's a +1 or -1
	if incomingData.Increment != 1 && incomingData.Increment != -1 {
		v := validator.New()
		v.AddError("increment", "must be either 1 or -1")
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	err = a.reviewModel.UpdateHelpfulCount(id, incomingData.Increment)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	// sent evelope notify of successful helpful_count update
	data := envelope{
		"message": "helpful count updated successfully",
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

// Delete functionality
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
	// send envelope informing of successful deletion
	data := envelope{
		"message": "review successfully deleted",
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

// listReview for listing, searching and sorting
// currently doesn't work alongside ListProductHandler and I still don't know why,
// hoping some sleep will let me better see where I went wrong, currently up from 8AM - 4AM
// working on this Test # 1, only taking breaks for food and bathroom
func (a *applicationDependencies) ListReviewsHandler(w http.ResponseWriter, r *http.Request) {
	// store parameters to query data into a struct
	var queryParametersData struct {
		Prod_ID       int
		Rating        int
		Helpful_Count int
		data.Filters
	}
	// get the parameters from the url
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
