package data

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/ReynerioSamos/reviews/internal/validator"
)

type Review struct {
	RID           int64     `json:"rid"`     // unique value for each product
	Prod_ID       int64     `json:"prod_id"` // associated product ID
	Rating        int8      `json:"rating`
	Helpful_Count int64     `json:"helpful_count"`
	CreatedAt     time.Time `json:"-"` // database timestamp

}

func ValidateReview(v *validator.Validator, review *Review, vdb ReviewModel) {
	// Empty values check validators
	v.Check(review.Prod_ID != 0, "Associated Product ID:", "must be provided")
	v.Check(review.Rating != 0, "Review Score (1-5):", "Must be prodivded")
	// Check if Prod_ID is a positive value
	v.Check(review.Prod_ID > 0, "Associated Product ID:", "must be a positive integer")

	// Check if Prod_ID is valid
	var count int
	err := vdb.DB.QueryRow("SELECT COUNT(*) FROM product WHERE pid = $1", review.Prod_ID).Scan(&count)
	if err != nil {
		v.AddError("Associated Product ID:", "database error")
		return
	}
	if count == 0 {
		v.AddError("Associated Product ID:", "product not found")
		return
	}

	// Check if Rating is valid
	v.Check(review.Rating >= 1 && review.Rating <= 5, "Review Score (1-5):", "must be between 1 and 5")
}

type ReviewModel struct {
	DB *sql.DB
}

func (r ReviewModel) Insert(review *Review) error {
	// Begin a transaction since we'll need to update two tables atomically
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback if we don't commit

	// Insert the review and update product's avg_rating in a single query
	query := `
        WITH inserted_review AS (
            INSERT INTO review (prod_id, rating)
            VALUES ($1, $2)
            RETURNING rid, created_at, prod_id, rating
        ),
        update_product_avg AS (
            UPDATE product p
            SET avg_rating = (
                SELECT AVG(rating)::DECIMAL(3,2)
                FROM review r
                WHERE r.prod_id = p.pid
            )
            WHERE pid = $1
            RETURNING name
        )
        SELECT 
            ir.rid,
            ir.created_at,
            ir.prod_id,
            up.name,
            ir.rating
        FROM inserted_review ir
        CROSS JOIN update_product_avg up;
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
		&review.Rating,
	)
	if err != nil {
		return fmt.Errorf("failed to insert review: %w", err)
	}

	// Commit the transaction
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// Get/Read Functionality
// Get a specific Coment from the products table
func (r ReviewModel) Get(id int64) (*Review, error) {
	// check if the id is valid
	if id < 1 {
		return nil, ErrRecordNotFound
	}

	// the SQL query to be executed against the database table
	query := `
		SELECT r.rid, r.created_at, r.prod_id, p.pname , r.rating, r.helpful_count
		FROM review AS r
		JOIN product P
		WHERE id = $1
		`
	// declare a variable of type product to store the returned product
	var review Review

	// Set a 3-second context/time
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := r.DB.QueryRowContext(ctx, query, id).Scan(
		&review.RID,
		&review.CreatedAt,
		&review.Prod_ID,
		&review.Rating,
		&review.Helpful_Count,
	)

	if err != nil {
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return nil, ErrRecordNotFound
		default:
			return nil, err
		}
	}
	return &review, nil
}

// Post/Update Functionality
// U - in CRUD also applies to the helpful_count attribute
func (r ReviewModel) Update(review *Review) error {

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
            SET 
                rating = $1,
            WHERE rid = $2
            RETURNING rid, rating, prod_id
        ),
        update_product_avg AS (
            UPDATE product p
            SET avg_rating = (
                SELECT AVG(rating)::DECIMAL(3,2)
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
            RETURNING pid, avg_rating, pname
        )
        SELECT 
            ur.rid,
            ur.rating,
            ur.prod_id,
            ur.version,
            ur.updated_at,
            up.name,
            up.avg_rating
        FROM updated_review ur
        CROSS JOIN update_product_avg up;
    `

	err = tx.QueryRowContext(
		ctx,
		query,
		review.Rating,
		review.RID,
	).Scan(
		&review.RID,
		&review.Rating,
	)

	if err == sql.ErrNoRows {
		return ErrRecordNotFound
	} else if err != nil {
		return fmt.Errorf("failed to update review: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// increments helpful_count attribute by 1 or -1 depending on input
func (r ReviewModel) Helpful_Count_Increment(rid int64, val int8) error {
	// Validate input
	if val != 1 && val != -1 {
		return fmt.Errorf("invalid increment value: %d, expected 1 or -1", val)
	}

	query := `
        UPDATE review
        SET helpful_count = helpful_count + $1
        WHERE id = $2
    `

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	result, err := r.DB.ExecContext(ctx, query, val, rid)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrRecordNotFound
	}

	return nil
}

// Cut/Delete Functionaity

func (r ReviewModel) Delete(id int64) error {
	// Input validation
	if id < 1 {
		return fmt.Errorf("invalid review ID: %d", id)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	tx, err := r.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Delete review and update product's avg_rating in a single query
	query := `
        WITH deleted_review AS (
            DELETE FROM review
            WHERE rid = $1
            RETURNING prod_id, rating
        ),
        update_product_avg AS (
            UPDATE product p
            SET 
                avg_rating = COALESCE(
                    (SELECT AVG(rating)::DECIMAL(3,2)
                     FROM review r
                     WHERE r.prod_id = (SELECT prod_id FROM deleted_review)
                     AND r.rid != $1),
                    0  -- Set to 0 if this was the last review
                )
            WHERE pid = (SELECT prod_id FROM deleted_review)
            RETURNING 
                pid,
                pname,
                avg_rating,
        )
        SELECT 
            dr.prod_id,
            up.pname,
            up.avg_rating,
        FROM deleted_review dr
        JOIN update_product_avg up ON up.pid = dr.prod_id;
    `
	result, err := r.DB.ExecContext(ctx, query, id)
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
func (r ReviewModel) GetAll(prod_id int, rating int, helpful_count int, filters Filters) ([]*Review, Metadata, error) {
	// The SQL query to be executed against database table

	// We will use Postgresql built in full text search feature
	// which allows us to do natural language searches
	// $? = '' allows for content and author to be optional

	// Query formatted string to be able to add the sort values, We are not sure what will be the column
	// sort by or the order
	query := fmt.Sprintf(`
		SELECT COUNT(*) OVER(), rid, prod_id, created_at, rating, helpful_count
		FROM review
		WHERE (to_tsvector('simple', prod_id) @@
				plainto_tsquery('simple', $1) OR $1 = '')
		AND (to_tsvector('simple', rating) @@
				plainto_tsquery('simple', $2) OR $2 = '')
		AND (to_tsvector('simple', helpful_count) @@
				plainto_tsquery('simple', $3) OR $3 = '')
		ORDER BY %s %s, pid ASC
		LIMIT $4 OFFSET $5
		`, filters.sortColumn(), filters.sortDirection())

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Query context returns multiple rows
	rows, err := r.DB.QueryContext(ctx, query, prod_id, rating, helpful_count, filters.limit(), filters.offset())
	if err != nil {
		return nil, Metadata{}, err
	}

	// cleanup the memory that was used
	defer rows.Close()
	totalRecords := 0
	//we wil store the address of each comment in our slice
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
			&review.Helpful_Count)
		if err != nil {
			return nil, Metadata{}, err
		}
		// add the row to our slice
		reviews = append(reviews, &review)
	} // end of the loop
	// after we exit the loop, we need to check if it generated any errors
	err = rows.Err()
	if err != nil {
		return nil, Metadata{}, err
	}

	// create the metadata
	metadata := calculateMetaData(totalRecords, filters.Page, filters.PageSize)

	return reviews, metadata, nil
}
