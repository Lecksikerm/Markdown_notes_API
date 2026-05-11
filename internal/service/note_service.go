package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"markdown-notes/internal/model"
	"markdown-notes/internal/repository"
)

type NoteService struct {
	repo *repository.NoteRepository
}

func NewNoteService(repo *repository.NoteRepository) *NoteService {
	return &NoteService{repo: repo}
}

func (s *NoteService) Create(ctx context.Context, req model.CreateNoteRequest) (*model.Note, error) {
	note := &model.Note{
		ID:        uuid.New(),
		Title:     req.Title,
		Content:   req.Content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Create(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *NoteService) GetByID(ctx context.Context, id uuid.UUID) (*model.Note, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *NoteService) GetAll(ctx context.Context) ([]model.Note, error) {
	return s.repo.GetAll(ctx)
}

func (s *NoteService) Update(ctx context.Context, id uuid.UUID, req model.UpdateNoteRequest) (*model.Note, error) {
	note, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Title != "" {
		note.Title = req.Title
	}
	if req.Content != "" || req.Content == "" { 
		note.Content = req.Content
	}
	note.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, note); err != nil {
		return nil, err
	}
	return note, nil
}

func (s *NoteService) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.Delete(ctx, id)
}
