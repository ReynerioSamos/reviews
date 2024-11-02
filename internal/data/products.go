package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ReynerioSamos/reviews/internal/validator"
)

type Product struct {
	PID              int64     `json:"pid"` // unique value for each product
	Pname            string    `json:"pname`
	Product_Category string    `json:"product_category`
	Image_URL        string    `json:"image_URL"`
	Avg_Rating       float32   `json:"avg_rating"`
	CreatedAt        time.Time `json:"-"` // database timestamp

}

func ValidateProduct(v *validator.Validator, product *Product) {
	// check if product name field is empty
	v.Check(product.Pname != "", "Product Name", "must be provided")
	// check if product category field is empty
	v.Check(product.Product_Category != "", "Product Category", "must be provided")

	// check if the product name field is too long
	v.Check(len(product.Pname) <= 255, "Product Name", "must not be more than 255 bytes long")

	//check if Product Categroy is suitable
	//v.Check(len(product.Product_Category) <= 25, "Product", "must be of type:")
}

type ProductModel struct {
	DB *sql.DB
}

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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
		WHERE id = $1
		`
	// declare a variable of type product to store the returned product
	var product Product

	// Set a 3-second context/time
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
			return nil, err
		}
	}
	return &product, nil
}

// Post/Update Functionality
func (p ProductModel) Update(product *Product) error {
	// The SQL query to be executed against the database table
	query := `
		UPDATE product
		SET pname = $1, product_category = $2, img_url = $3
		WHERE id = $4
		RETURNING pname, product_category
		`

	args := []any{product.Pname, product.Product_Category, product.Image_URL, product.PID}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	return p.DB.QueryRowContext(ctx, query, args...).Scan(&product.Pname, &product.Product_Category)
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
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// ExecContext does not return any rows unlike QueryRowContext.
	// It only returns information about the query execution
	// such as how many rows were affected
	result, err := p.DB.ExecContext(ctx, query, id)
	if err != nil {
		return err
	}

	// Were any rows deleted?
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
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
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), pid, created_at, pname, product_category, image_URL, avg_rating
		FROM product
		WHERE (to_tsvector('simple', pname) @@
				plainto_tsquery('simple', $1) OR $1 = '')
		AND (to_tsvector('simple', product_category) @@
				plainto_tsquery('simple', $2) OR $2 = '')
		AND (to_tsvector('simple', avg_rating) @@
				plainto_tsquery('simple', $3) OR $3 = '')
		ORDER BY %s %s, pid ASC
		LIMIT $4 OFFSET $5
		`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Query context returns multiple rows
	rows, err := p.DB.QueryContext(ctx, query, pname, product_category, avg_rating, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, err
	}

	// cleanup the memory that was used
	defer rows.Close()
	totalRecords := 0
	//we wil store the address of each comment in our slice
	products := []*Product{}

	// process each row that is in the var rows
	for rows.Next() {
		var product Product
		err := rows.Scan(
			&totalRecords,
			&product.PID,
			&product.CreatedAt,
			&product.Pname,
			&product.Product_Category,
			&product.Image_URL,
			&product.Avg_Rating)
		if err != nil {
			return nil, Metadata{}, err
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
