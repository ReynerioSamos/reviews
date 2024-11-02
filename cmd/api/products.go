package main

import (
	_ "encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/ReynerioSamos/reviews/internal/data"
	"github.com/ReynerioSamos/reviews/internal/validator"
)

func (a *applicationDependencies) createProductHandler(w http.ResponseWriter, r *http.Request) {
	// create a struct to hold a product
	// we use struct tags [``] to make the names display in lowercase
	var incomingData struct {
		Pname            string `json:"pname"`
		Product_Category string `json:"product_category"`
		Image_URL        string `json:"image_url"`
	}

	// perform the decoding
	err := a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	// Copy the values from incomingData to a new product struct
	// At this point in our code the JSON is well-formed JSON so now
	// we will validate it using the Validators which expects a product
	product := &data.Product{
		Pname:            incomingData.Pname,
		Product_Category: incomingData.Product_Category,
		Image_URL:        incomingData.Image_URL,
	}
	// Intialize Validator instance
	v := validator.New()
	// Do the validation
	data.ValidateProduct(v, product)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	// Add the product to the database table
	err = a.productModel.Insert(product)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	// for now display the result
	fmt.Fprintf(w, "%+v\n", incomingData)

	// Set a Location header. The path to the newly created product
	headers := make(http.Header)
	headers.Set("Location", fmt.Sprintf("/v1/product/%d", product.PID))

	// Send a JSON response with 201 (new resource created) status code
	data := envelope{
		"product": product,
	}
	err = a.writeJson(w, http.StatusCreated, data, headers)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) displayProductHandler(w http.ResponseWriter, r *http.Request) {
	// get the id from the URL /v1/products/:id so that we can use it to query the products table
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	//Call Get() to retrieve the product with the specified id
	product, err := a.productModel.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	// display the product
	data := envelope{
		"product": product,
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) updateProductHandler(w http.ResponseWriter, r *http.Request) {
	// Get the id from the URL
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	// Call Get() to retrieve the comment with specified id
	product, err := a.productModel.Get(id)
	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}

	// Use our temporary incomingData struct to hold the data
	// Note: types have been changed to pointers to differentiate b/w the client
	// leaving a field empty intentionally and the field not needing to be updated
	var incomingData struct {
		Pname            *string `json:"pname"`
		Product_Category *string `json:"product_category"`
		Image_URL        *string `json:"image_url"`
	}

	// decoding
	err = a.readJson(w, r, &incomingData)
	if err != nil {
		a.badRequestResponse(w, r, err)
		return
	}

	// We need to now check the fields to see which ones need updating
	// if incomingData.Pname is nil, no update done
	if incomingData.Pname != nil {
		product.Pname = *incomingData.Pname
	}

	// if incomingData.Product_Category is nil, no update was provided
	if incomingData.Product_Category != nil {
		product.Product_Category = *incomingData.Product_Category
	}

	// if incomingData.Image_URL is nil, no update was provided
	if incomingData.Image_URL != nil {
		product.Image_URL = *incomingData.Image_URL
	}

	// Before we write the updates to the DB let's validate
	v := validator.New()
	data.ValidateProduct(v, product)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)
		return
	}

	// perform the update
	err = a.productModel.Update(product)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
	data := envelope{
		"product": product,
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}
}

func (a *applicationDependencies) deleteProductHandler(w http.ResponseWriter, r *http.Request) {
	id, err := a.readIDParam(r)
	if err != nil {
		a.notFoundResponse(w, r)
		return
	}

	err = a.productModel.Delete(id)

	if err != nil {
		switch {
		case errors.Is(err, data.ErrRecordNotFound):
			a.notFoundResponse(w, r)
		default:
			a.serverErrorResponse(w, r, err)
		}
		return
	}
	// display the product
	data := envelope{
		"message": "product successfully deleted",
	}
	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}

// create the list handler
// currently not working and I really don't know why, Will as Mr. Dalwin when I get some sleep
func (a *applicationDependencies) ListProductsHandler(w http.ResponseWriter, r *http.Request) {
	// create a struct to hold the query parameters
	// no field name for the type data.Filters
	var queryParametersData struct {
		Pname            string
		Product_Category string
		Image_URL        string
		Avg_Rating       float32
		data.Filters
	}
	// get the query parameters from the URL
	queryParameters := r.URL.Query()

	// load the query parameters into our struct
	queryParametersData.Pname = a.getSingleQueryParameter(
		queryParameters,
		"pname", "")

	queryParametersData.Product_Category = a.getSingleQueryParameter(
		queryParameters,
		"product_category", "")

	// Handle avg_rating as float32
	avgRatingStr := a.getSingleQueryParameter(
		queryParameters,
		"avg_rating", "")

	if avgRatingStr != "" {
		avgRating, err := strconv.ParseFloat(avgRatingStr, 32)
		if err != nil {
			v := validator.New()
			v.AddError("avg_rating", "must be a valid number")
			a.failedValidationResponse(w, r, v.Errors)
			return
		}
		queryParametersData.Avg_Rating = float32(avgRating)
	}

	// create a new validator instance
	v := validator.New()

	queryParametersData.Filters.Page = a.getSingleIntegerParameter(
		queryParameters, "page", 1, v)

	queryParametersData.Filters.PageSize = a.getSingleIntegerParameter(
		queryParameters, "page_size", 10, v)

	queryParametersData.Filters.Sort = a.getSingleQueryParameter(
		queryParameters, "sort", "pid")

	queryParametersData.Filters.SortSafeList = []string{"pid", "pname", "product_category", "avg_rating",
		"-pid", "-pname", "-product_category", "-avg_rating"}

	// Check if our filters are valid
	data.ValidateFilters(v, queryParametersData.Filters)
	if !v.IsEmpty() {
		a.failedValidationResponse(w, r, v.Errors)

		return
	}

	products, metadata, err := a.productModel.GetAll(queryParametersData.Pname, queryParametersData.Product_Category, queryParametersData.Avg_Rating, queryParametersData.Filters)
	if err != nil {
		a.serverErrorResponse(w, r, err)
		return
	}

	data := envelope{
		"products":  products,
		"@metadata": metadata,
	}

	err = a.writeJson(w, http.StatusOK, data, nil)
	if err != nil {
		a.serverErrorResponse(w, r, err)
	}
}
