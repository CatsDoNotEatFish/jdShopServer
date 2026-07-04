package service

import (
	"errors"

	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var ErrAnnouncementNotFound = errors.New("announcement not found")

type AnnouncementService struct {
	repo *repository.AnnouncementRepo
}

func NewAnnouncementService(repo *repository.AnnouncementRepo) *AnnouncementService {
	return &AnnouncementService{repo: repo}
}

func (s *AnnouncementService) Create(title, content, level string, createdBy int64) (*model.Announcement, error) {
	if level == "" {
		level = "info"
	}
	a := &model.Announcement{
		Title:       title,
		Content:     content,
		Level:       level,
		IsPublished: 0,
		CreatedBy:   createdBy,
	}
	if err := s.repo.Create(a); err != nil {
		return nil, err
	}
	return s.repo.FindByID(a.ID)
}

func (s *AnnouncementService) Update(id int64, title, content, level *string) (*model.Announcement, error) {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrAnnouncementNotFound
	}
	if err := s.repo.Update(id, title, content, level); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func (s *AnnouncementService) Publish(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrAnnouncementNotFound
	}
	return s.repo.Publish(id)
}

func (s *AnnouncementService) Unpublish(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrAnnouncementNotFound
	}
	return s.repo.Unpublish(id)
}

func (s *AnnouncementService) Delete(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrAnnouncementNotFound
	}
	return s.repo.Delete(id)
}

func (s *AnnouncementService) GetByID(id int64) (*model.Announcement, error) {
	return s.repo.FindByID(id)
}

func (s *AnnouncementService) ListPublic(page, pageSize int, level string) ([]model.Announcement, int64, error) {
	return s.repo.List(page, pageSize, level, true)
}

func (s *AnnouncementService) ListAll(page, pageSize int, level string) ([]model.Announcement, int64, error) {
	return s.repo.List(page, pageSize, level, false)
}
