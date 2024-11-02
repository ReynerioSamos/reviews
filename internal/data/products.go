package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ReynerioSamos/reviews/internal/validator"
)

type Product struct {
	PID              int64     `json:"pid"`              // unique value for each product
	Pname            string    `json:"pname"`            // name of the product
	Product_Category string    `json:"product_category"` // category of the product
	Image_URL        string    `json:"image_url"`        // string containing URL for image for product
	Avg_Rating       float32   `json:"avg_rating"`       // avg_rating of product, updates on review creation, deletion and updates
	CreatedAt        time.Time `json:"-"`                // database timestamp

}

func ValidateProduct(v *validator.Validator, product *Product) {
	//string.TrimSpace() is used to treat long spaces as empty as well (____ vs _)
	// check if product name field is empty
	v.Check(strings.TrimSpace(product.Pname) != "", "Product Name", "must be provided")
	// check if product category field is empty
	v.Check(strings.TrimSpace(product.Product_Category) != "", "Product Category", "must be provided")

	// check if the product name field is too long
	v.Check(len(product.Pname) <= 255, "Product Name", "must not be more than 255 bytes long")

}

type ProductModel struct {
	DB *sql.DB
}

// Make defaultTimeout a const since it's used in every db connection and query
const (
	defaultTimeout = 3 * time.Second
)

// Insert/Create Functionality
func (p ProductModel) Insert(product *Product) error {
	// the SQL query to be executed against the database table
	query := `
		INSERT INTO product (pname, product_category, image_URL)
		VALUES ($1, $2, $3)
		RETURNING pid, created_at
		`
	// the actual values to replace $1, and $2
	args := []any{product.Pname, product.Product_Category, product.Image_URL}

	// Create a context with a 3-second timeout. No database
	// operation should take more than 3 seconds or we will quit it
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// executre the query against the products database table. We ask for the
	// id, created_at, and the version to be sent back to us which we will use
	// to update the product struct later on

	return p.DB.QueryRowContext(ctx, query, args...).Scan(&product.PID, &product.CreatedAt)
}

// Get/Read Functionality
// Get a specific Coment from the products table
func (p ProductModel) Get(id int64) (*Product, error) {
	// check if the id is valid
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	// the SQL query to be executed against the database table
	query := `
		SELECT pid, created_at, pname, product_category, image_URL, avg_rating
		FROM product
		WHERE pid = $1
		`
	// declare a variable of type product to store the returned product
	var product Product

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := p.DB.QueryRowContext(ctx, query, id).Scan(
		&product.PID,
		&product.CreatedAt,
		&product.Pname,
		&product.Product_Category,
		&product.Image_URL,
		&product.Avg_Rating,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, fmt.Errorf("getting product: %w", err)
		}
	}
	return &product, nil
}

// Post/Update Functionality
func (p ProductModel) Update(product *Product) error {
	// The SQL query to be executed against the database table
	query := `
		UPDATE product
		SET pname = $1, product_category = $2, image_url = $3
		WHERE pid = $4
		RETURNING pname, product_category
		`

	args := []any{product.Pname, product.Product_Category, product.Image_URL, product.PID}
	ctx, cancel := context.WithTimeout(context.Background(), 3*defaultTimeout)
	defer cancel()

	result := p.DB.QueryRowContext(ctx, query, args...)

	err := result.Scan(&product.Pname, &product.Product_Category)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return fmt.Errorf("updating product: %w", err)
		}
	}
	return nil
}

// Cut/Delete Functionaity

func (p ProductModel) Delete(id int64) error {
	// check if the id is valid
	if id < 1 {
		return ErrRecordNotFound
	}
	// the SQL query to be executied against the database table
	query := `
		DELETE FROM product
		WHERE pid = $1	
		`
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// ExecContext does not return any rows unlike QueryRowContext.
	// It only returns information about the query execution
	// such as how many rows were affected
	result, err := p.DB.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("deleting product: %w", err)
	}

	// Were any rows deleted?
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("checking affected rows: %w", err)
	}

	//Probably a wrong id was provided or the client is trying to delete an already deleted comment
	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// Get all comments
func (p ProductModel) GetAll(pname string, product_category string, avg_rating float32, filters Filters) ([]*Product, Metadata, error) {
	// The SQL query to be executed against database table

	// We will use Postgresql built in full text search feature
	// which allows us to do natural language searches
	// $? = '' allows for content and author to be optional

	// Query formatted string to be able to add the sort values, We are not sure what will be the column
	// sort by or the order

	// Format the avg_rating to text for tsquery compatibility
	avgRatingStr := fmt.Sprintf("%.2f", avg_rating)
	// It's then compared using CAST() from postgresql to better match corresponding decimals using LIKE

	query := `
		SELECT COUNT(*) OVER(), pid, created_at, pname, product_category, image_URL, avg_rating
		FROM product
		WHERE (to_tsvector('simple', pname) @@
				plainto_tsquery('simple', $1) OR $1 = '')
		AND (to_tsvector('simple', product_category) @@
				plainto_tsquery('simple', $2) OR $2 = '')
		AND (CAST(avg_rating AS TEXT) LIKE $3 OR $3 = '')
		ORDER BY pid
		LIMIT $4 OFFSET $5
		`

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Query context returns multiple rows
	// newly formatted avgRatingStr is used
	rows, err := p.DB.QueryContext(ctx, query, pname, product_category, avgRatingStr, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("querying products: %w", err)
	}

	// cleanup the memory that was used
	defer rows.Close()
	totalRecords := 0
	//we wil store the address of each product in our slice
	products := []*Product{}

	// process each row that is in the var rows
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&totalRecords, // window function result
			&product.PID,
			&product.CreatedAt,
			&product.Pname,
			&product.Product_Category,
			&product.Image_URL,
			&product.Avg_Rating)

		if err != nil {
			return nil, Metadata{}, fmt.Errorf("scanning product row: %w", err)
		}
		// add the row to our slice
		products = append(products, &product)
	} // end of the loop

	// after we exit the loop, we need to check if it generated any errors
	err = rows.Err()
	if err != nil {
		return nil, Metadata{}, err
	}

	// create the metadata
	metadata := calculateMetaData(totalRecords, filters.Page, filters.PageSize)

	return products, metadata, nil
}
