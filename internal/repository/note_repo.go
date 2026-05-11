package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"markdown-notes/internal/model"
)

type NoteRepository struct {
	db *pgxpool.Pool
}

func NewNoteRepository(db *pgxpool.Pool) *NoteRepository {
	return &NoteRepository{db: db}
}

func (r *NoteRepository) Create(ctx context.Context, note *model.Note) error {
	query := `
		INSERT INTO notes (id, title, content, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.db.Exec(ctx, query, note.ID, note.Title, note.Content, note.CreatedAt, note.UpdatedAt)
	return err
}

func (r *NoteRepository) GetByID(ctx context.Context, id uuid.UUID) (*model.Note, error) {
	query := `
		SELECT id, title, content, created_at, updated_at
		FROM notes WHERE id = $1
	`
	note := &model.Note{}
	err := r.db.QueryRow(ctx, query, id).Scan(
		&note.ID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return note, nil
}

func (r *NoteRepository) GetAll(ctx context.Context) ([]model.Note, error) {
	query := `
		SELECT id, title, content, created_at, updated_at
		FROM notes ORDER BY updated_at DESC
	`
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var notes []model.Note
	for rows.Next() {
		var note model.Note
		if err := rows.Scan(&note.ID, &note.Title, &note.Content, &note.CreatedAt, &note.UpdatedAt); err != nil {
			return nil, err
		}
		notes = append(notes, note)
	}
	return notes, rows.Err()
}

func (r *NoteRepository) Update(ctx context.Context, note *model.Note) error {
	query := `
		UPDATE notes 
		SET title = $1, content = $2, updated_at = $3
		WHERE id = $4
	`
	_, err := r.db.Exec(ctx, query, note.Title, note.Content, note.UpdatedAt, note.ID)
	return err
}

func (r *NoteRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM notes WHERE id = $1`
	_, err := r.db.Exec(ctx, query, id)
	return err
}
