package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/ReynerioSamos/reviews/internal/validator"
)

const (
	minRating = 1
	maxRating = 5
	// defaultTimeout = 3*time.seconds
)

type Review struct {
	RID           int64     `json:"rid"`                    // unique value for each product
	Prod_ID       int64     `json:"prod_id"`                // associated product ID
	Rating        int8      `json:"rating"`                 // rating field from 1-5
	Helpful_Count int       `json:"helpful_count"`          // helpful_count integer
	CreatedAt     time.Time `json:"-"`                      // database timestamp
	ProductName   string    `json:"product_name,omitempty"` // additional field to help with joins
}

type ReviewModel struct {
	DB *sql.DB
}

func ValidateReview(v *validator.Validator, review *Review, vdb ReviewModel) {
	log.Printf("Validating review for product ID: %d", review.Prod_ID)
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()
	// Check if ProdID is positive
	v.Check(review.Prod_ID > 0, "Prod_ID:", "must be valid")
	// Check if Rating is valid
	v.Check(review.Rating >= minRating && review.Rating <= maxRating, "Rating:", "must be between 1 and 5")
	// Empty values check validators
	v.Check(review.Prod_ID != 0, "Prod_ID:", "must be provided")
	v.Check(review.Rating != 0, "Rating:", "must be prodivded")

	// Check if product exists using a prepared statement
	log.Printf("Checking product existance in database...")
	var exists bool
	err := vdb.DB.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM product WHERE pid = $1)`,
		review.Prod_ID).Scan(&exists)

	if err != nil {
		log.Printf("Database error: %v", err)
		v.AddError("database", fmt.Sprintf("error checking product existance: %s", err))
		return
	}
	if !exists {
		v.AddError("Prod_ID:", "referenced prodcut does not exist")
	}
}

func (r ReviewModel) Insert(review *Review) error {
	// Begin a transaction since we'll need to update two tables atomically
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Insert the review and update product's avg_rating in a single query
	query := `
        WITH inserted_review AS (
            INSERT INTO review (prod_id, rating, helpful_count)
            VALUES ($1, $2, 0)
            RETURNING rid, created_at, prod_id, rating, helpful_count
        ),
        update_avg AS (
            UPDATE product p
            SET avg_rating = (
                SELECT ROUND(AVG(rating)::numeric, 2)
                FROM review r
                WHERE r.prod_id = p.pid
            )
            WHERE pid = $1
            RETURNING pname
        )
        SELECT 
            ir.rid,
            ir.created_at,
            ir.prod_id,
			ua.pname,
            ir.rating,
			ir.helpful_count
        FROM inserted_review ir
        CROSS JOIN update_avg ua;
    `

	err = tx.QueryRowContext(
		ctx,
		query,
		review.Prod_ID,
		review.Rating,
	).Scan(
		&review.RID,
		&review.CreatedAt,
		&review.Prod_ID,
		&review.ProductName,
		&review.Rating,
		&review.Helpful_Count,
	)
	if err != nil {
		return fmt.Errorf("failed to insert review: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// Get/Read Functionality
// Get a specific review from the review table
func (r ReviewModel) Get(id int64) (*Review, error) {
	// check if the id is valid
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	// the SQL query to be executed against the database table
	query := `
		SELECT r.rid, r.created_at, r.prod_id, p.pname , r.rating, r.helpful_count
		FROM review r
		JOIN product p ON r.prod_id = p.pid
		WHERE r.rid = $1
		`
	// declare a variable of type product to store the returned product
	var review Review

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&review.RID,
		&review.CreatedAt,
		&review.Prod_ID,
		&review.ProductName,
		&review.Rating,
		&review.Helpful_Count,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, fmt.Errorf("getting review: %w", err)
		}
	}
	return &review, nil
}

// Post/Update Functionality
// U - in CRUD also applies to the helpful_count attribute
func (r ReviewModel) Update(review *Review) error {

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Update review and product's avg_rating in a single query using CTEs
	query := `
        WITH updated_review AS (
            UPDATE review
            SET rating = $1
            WHERE rid = $2
            RETURNING rid, rating, prod_id
        ),
        update_avg AS (
            UPDATE product p
            SET avg_rating = (
                SELECT ROUND(AVG(rating)::numeric, 2)
                FROM review r
                WHERE r.prod_id = (
                    SELECT prod_id 
                    FROM updated_review
                )
            )
            WHERE pid = (
                SELECT prod_id 
                FROM updated_review
            )
            RETURNING pname
        )
        SELECT 
            ur.rid,
            ur.rating,
            ua.pname
        FROM updated_review ur
        CROSS JOIN update_avg ua
    `
	err = tx.QueryRowContext(
		ctx,
		query,
		review.Rating,
		review.RID,
	).Scan(
		&review.RID,
		&review.Rating,
		&review.ProductName,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return fmt.Errorf("updating review: %w", err)
		}
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("committing transaction: %w", err)
	}

	return nil
}

// increments helpful_count attribute by 1 or -1 depending on input
func (r ReviewModel) UpdateHelpfulCount(rid int64, increment int8) error {
	// Validate input
	if increment != 1 && increment != -1 {
		return fmt.Errorf("invalid increment value: must be 1 pr -1")
	}

	query := `
        UPDATE review
        SET helpful_count = GREATEST(0, helpful_count + $1)
        WHERE rid = $2
		RETURNING helpful_count
    `

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	var newCount int64
	err := r.DB.QueryRowContext(ctx, query, increment, rid).Scan(&newCount)
	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return ErrRecordNotFound
		default:
			return fmt.Errorf("updating helpful count: %w", err)
		}
	}
	return nil
}

// Cut/Delete Functionaity

func (r ReviewModel) Delete(id int64) error {
	// Input validation
	if id < 1 {
		return fmt.Errorf("invalid review ID: %d", id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete review and update product's avg_rating in a single query
	query := `
        WITH deleted_review AS (
            DELETE FROM review
            WHERE rid = $1
            RETURNING prod_id
        ),
        update_avg AS (
            UPDATE product p
            SET 
                avg_rating = COALESCE(
                    (SELECT ROUND(AVG(rating)::numeric,2)
                     FROM review r
                     WHERE r.prod_id = (SELECT prod_id FROM deleted_review)), 0
                     -- Set to 0 if this was the last review
                )
            WHERE pid = (SELECT prod_id FROM deleted_review)
		)
		SELECT EXISTS(SELECT 1 FROM deleted_review)
    `
	// deleted bool to track whether it did successfully
	var deleted bool
	err = tx.QueryRowContext(ctx, query, id).Scan(&deleted)
	if err != nil {
		return fmt.Errorf("deleting review: %w", err)
	}

	if !deleted {
		return ErrRecordNotFound
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("commiting transaction: %w", err)
	}

	return nil
}

// Get all comments
func (r ReviewModel) GetAll(prod_id int, rating int, helpful_count int, filters Filters) ([]*Review, Metadata, error) {
	// The SQL query to be executed against database table

	// We will use Postgresql built in full text search feature
	// which allows us to do natural language searches
	// $? = '' allows for content and author to be optional

	// Query formatted string to be able to add the sort values, We are not sure what will be the column
	// sort by or the order
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), 
			r.rid, r.created_at, r.prod_id,
			r.rating, r.helpful_count, p.name
		FROM review r
		JOIN product p ON p.pid = r.prod_id
		WHERE 	(CAST(r.prod_id AS TEXT) = $1 OR $1 = '')
		AND		(CAST(r.rating AS TEXT) = $2 OR $2 = '')
		AND		(CAST(r.helpful_count AS TEXT) = $3 OR $3 = '')
		ORDER BY %s %s, r.rid ASC
		LIMIT $4 OFFSET $5
		`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeout)
	defer cancel()

	// Query context returns multiple rows
	rows, err := r.DB.QueryContext(ctx, query, prod_id, rating, helpful_count, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, fmt.Errorf("querying reviews: %w", err)
	}
	// cleanup the memory that was used
	defer rows.Close()

	totalRecords := 0
	//we wil store the address of each review in our slice
	reviews := []*Review{}

	// process each row that is in the var rows
	for rows.Next() {
		var review Review
		err := rows.Scan(
			&totalRecords,
			&review.RID,
			&review.CreatedAt,
			&review.Prod_ID,
			&review.Rating,
			&review.Helpful_Count,
			&review.ProductName)
		if err != nil {
			return nil, Metadata{}, fmt.Errorf("scanning review row: %w", err)
		}
		// add the row to our slice
		reviews = append(reviews, &review)
	} // end of the loop

	// create the metadata
	metadata := calculateMetaData(totalRecords, filters.Page, filters.PageSize)

	return reviews, metadata, nil
}
