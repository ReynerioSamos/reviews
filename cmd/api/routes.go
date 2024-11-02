package main

import (
	"net/http"

	"github.com/julienschmidt/httprouter"
)

func (a *applicationDependencies) routes() http.Handler {
	//set up new router
	router := httprouter.New()
	// handle 404
	router.NotFound = http.HandlerFunc(a.notFoundResponse)
	// handle 405
	router.MethodNotAllowed = http.HandlerFunc(a.methodNotAllowedResponse)

	//set up routes
	//route for health checker
	router.HandlerFunc(http.MethodGet, "/v1/healthcheck", a.healthCheckHandler)

	// routes for products CRUD functionality
	router.HandlerFunc(http.MethodPost, "/v1/product", a.createProductHandler)
	router.HandlerFunc(http.MethodGet, "/v1/product/:id", a.displayProductHandler)
	router.HandlerFunc(http.MethodPatch, "/v1/product/:id", a.updateProductHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/product/:id", a.deleteProductHandler)

	//routes for reviews CRUD functionality
	router.HandlerFunc(http.MethodPost, "/v1/preview", a.createReviewHandler)
	router.HandlerFunc(http.MethodGet, "/v1/review/:id", a.displayReviewHandler)
	router.HandlerFunc(http.MethodPatch, "/v1/review/:id", a.updateReviewHandler)
	router.HandlerFunc(http.MethodDelete, "/v1/review/:id", a.deleteReviewHandler)

	// route for List All Product handler
	router.HandlerFunc(http.MethodGet, "v1/product", a.ListProductHandler)
	//route for List All Reviews handler
	router.HandlerFunc(http.MethodGet, "/v1/review", a.ListReviewsHandler)

	//panic recover
	return a.recoverPanic(router)
}
