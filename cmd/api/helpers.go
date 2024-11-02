package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ReynerioSamos/reviews/internal/validator"
	"github.com/julienschmidt/httprouter"
)

// This Method will accept:
// response write (w)
// status code to send (default is 200)
// actual data to encode in Json
// a map of the headers to set for the response

// create and envelope type
type envelope map[string]any

func (a *applicationDependencies) writeJson(w http.ResponseWriter, status int, data envelope, headers http.Header) error {
	jsResponse, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return err
	}
	jsResponse = append(jsResponse, '\n')
	for key, value := range headers {
		w.Header()[key] = value
		//w.Header().Set(key, value[0])
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, err = w.Write(jsResponse)
	if err != nil {
		return err
	}
	return nil
}

func (a *applicationDependencies) readJson(w http.ResponseWriter, r *http.Request, destination any) error {
	// what is the max size of the request body (250KB seems reasonable)
	maxBytes := 256_000
	r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
	// our decoder will check for unknown fields
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	//let start the decoding

	//err := json.NewDecoder(r.Body).Decode(destination)
	err := dec.Decode(destination) //OCT 22,2024
	if err != nil {
		var syntaxError *json.SyntaxError
		var unmarshalTypeError *json.UnmarshalTypeError
		var invalidUnmarshalError *json.InvalidUnmarshalError
		var maxBytesError *http.MaxBytesError

		switch {
		case errors.As(err, &syntaxError):
			return fmt.Errorf("the body contains badly-formed JSON (at character %d)", syntaxError.Offset)

		case errors.Is(err, io.ErrUnexpectedEOF):
			return errors.New("the body contains badly-formed JSON")

		case errors.As(err, &unmarshalTypeError):
			if unmarshalTypeError.Field != "" {
				return fmt.Errorf("the body contains incorrect JSON type for field %q", unmarshalTypeError.Field)
			}
			return fmt.Errorf("the body contains incorrect JSON type (at character %d)", unmarshalTypeError.Offset)

		case errors.Is(err, io.EOF):
			return errors.New("the body must not be empty")

		// check for unknown field error
		case strings.HasPrefix(err.Error(), "json: unknown field "):
			fieldName := strings.TrimPrefix(err.Error(), "json: unknown field")
			return fmt.Errorf("body contain unknown key %s", fieldName)

		// does the body exceed our limit of 250KB?
		case errors.As(err, &maxBytesError):
			return fmt.Errorf("the body must not be larger than %d bytes", maxBytesError.Limit)
		// the programmer messed up
		case errors.As(err, &invalidUnmarshalError):
			panic(err)
		// return other type of error
		default:
			return err
		}
	}
	// Check if there is any data after the valid JSON data.
	// Person might be trying to send multiple bodies during one request
	// call decode once more to see if it gives us back anything
	// we use a throw away struct 'struct{}{}' to hold result
	err = dec.Decode(&struct{}{})
	if !errors.Is(err, io.EOF) {
		return errors.New("the body must only contain a single JSON value")
	}
	return nil
}

func (a *applicationDependencies) readIDParam(r *http.Request) (int64, error) {
	// get the URL parameters
	params := httprouter.ParamsFromContext(r.Context())
	// Convert the id from string into int
	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)
	if err != nil || id < 1 {
		return 0, errors.New("invalid id parameter")
	}
	return id, nil
}

func (a *applicationDependencies) getSingleQueryParameter(queryParameters url.Values,
	key string,
	defaultValue string) string {
	// url.values is a key:value hash map of the query parameter
	result := queryParameters.Get(key)
	if result == "" {
		return defaultValue
	}
	return result
}

// call when we have multiple comma-separate values
func (a *applicationDependencies) getMultipleQueryParameters(queryParameters url.Values,
	key string,
	defaultValue []string) []string {
	result := queryParameters.Get(key)
	if result == "" {
		return defaultValue
	}
	return strings.Split(result, ",")
}

// this method can cause a validation error when trying to convert the string to a valid integer value
func (a *applicationDependencies) getSingleIntegerParameter(queryParameters url.Values,
	key string,
	defaultValue int,
	v *validator.Validator) int {
	result := queryParameters.Get(key)
	if result == "" {
		return defaultValue
	}
	// try to convert to an integer
	intValue, err := strconv.Atoi(result)
	if err != nil {
		v.AddError(key, "must be an integer value")
		return defaultValue
	}
	return intValue
}
