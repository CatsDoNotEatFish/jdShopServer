package service

import (
	"errors"

	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var ErrAnnouncementNotFound = errors.New("announcement not found")

type AnnouncementService struct {
	repo       *repository.AnnouncementRepo
	controlHub *ControlHub
}

func NewAnnouncementService(repo *repository.AnnouncementRepo, controlHub *ControlHub) *AnnouncementService {
	return &AnnouncementService{repo: repo, controlHub: controlHub}
}

func (s *AnnouncementService) Create(a *model.Announcement, createdBy int64) (*model.Announcement, error) {
	a.CreatedBy = createdBy
	if err := s.repo.Create(a); err != nil {
		return nil, err
	}
	return s.repo.FindByID(a.ID)
}

func (s *AnnouncementService) Update(id int64, req model.UpdateAnnouncementRequest) (*model.Announcement, error) {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if a == nil {
		return nil, ErrAnnouncementNotFound
	}
	if req.Title != nil {
		a.Title = *req.Title
	}
	if req.Content != nil {
		a.Content = *req.Content
	}
	if req.Level != nil {
		a.Level = *req.Level
	}
	if req.DisplayMode != nil {
		a.DisplayMode = *req.DisplayMode
	}
	if req.ShowPolicy != nil {
		a.ShowPolicy = *req.ShowPolicy
	}
	if req.StartsAt != nil {
		a.StartsAt = cleanOptionalString(req.StartsAt)
	}
	if req.EndsAt != nil {
		a.EndsAt = cleanOptionalString(req.EndsAt)
	}
	if req.TargetType != nil {
		a.TargetType = *req.TargetType
	}
	if req.TargetPlatform != nil {
		a.TargetPlatform = *req.TargetPlatform
	}
	if req.MinVersionCode != nil {
		a.MinVersionCode = positiveInt64(req.MinVersionCode)
	}
	if req.MaxVersionCode != nil {
		a.MaxVersionCode = positiveInt64(req.MaxVersionCode)
	}
	if req.ActionText != nil {
		a.ActionText = cleanOptionalString(req.ActionText)
	}
	if req.ActionURL != nil {
		a.ActionURL = cleanOptionalString(req.ActionURL)
	}
	if req.TargetUserIDs != nil {
		a.TargetUserIDs = req.TargetUserIDs
	}
	if err := s.repo.Update(a); err != nil {
		return nil, err
	}
	updated, err := s.repo.FindByID(id)
	if err == nil && updated != nil && updated.IsPublished == 1 {
		s.controlHub.Broadcast(model.ControlEvent{
			Type:           "announcement_changed",
			AnnouncementID: updated.ID,
			Revision:       updated.Revision,
		})
	}
	return updated, err
}

func (s *AnnouncementService) Publish(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrAnnouncementNotFound
	}
	if err := s.repo.Publish(id); err != nil {
		return err
	}
	updated, err := s.repo.FindByID(id)
	if err == nil && updated != nil {
		s.notify(*updated, "announcement_published")
	}
	return err
}

func (s *AnnouncementService) Unpublish(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrAnnouncementNotFound
	}
	if err := s.repo.Unpublish(id); err != nil {
		return err
	}
	s.controlHub.Broadcast(model.ControlEvent{Type: "announcement_changed", AnnouncementID: id})
	return nil
}

func (s *AnnouncementService) Delete(id int64) error {
	a, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if a == nil {
		return ErrAnnouncementNotFound
	}
	if err := s.repo.Delete(id); err != nil {
		return err
	}
	if a.IsPublished == 1 {
		s.controlHub.Broadcast(model.ControlEvent{Type: "announcement_changed", AnnouncementID: id})
	}
	return nil
}

func (s *AnnouncementService) GetByID(id int64) (*model.Announcement, error) {
	return s.repo.FindByID(id)
}

func (s *AnnouncementService) ListPublic(page, pageSize int, level string) ([]model.Announcement, int64, error) {
	items, total, err := s.repo.List(page, pageSize, level, true)
	for index := range items {
		items[index].DeliveredCount = 0
		items[index].ReadCount = 0
		items[index].AcknowledgedCount = 0
	}
	return items, total, err
}

func (s *AnnouncementService) ListAll(page, pageSize int, level string) ([]model.Announcement, int64, error) {
	return s.repo.List(page, pageSize, level, false)
}

func (s *AnnouncementService) ListForUser(userID int64, platform string, versionCode int64) ([]model.Announcement, error) {
	return s.repo.ListForUser(userID, platform, versionCode)
}

func (s *AnnouncementService) MarkRead(id, userID int64) error {
	available, err := s.repo.AvailableForUser(id, userID)
	if err != nil {
		return err
	}
	if !available {
		return ErrAnnouncementNotFound
	}
	return s.repo.MarkRead(id, userID)
}

func (s *AnnouncementService) Acknowledge(id, userID int64) error {
	available, err := s.repo.AvailableForUser(id, userID)
	if err != nil {
		return err
	}
	if !available {
		return ErrAnnouncementNotFound
	}
	return s.repo.Acknowledge(id, userID)
}

func (s *AnnouncementService) notify(a model.Announcement, eventType string) {
	event := model.ControlEvent{
		Type:           eventType,
		AnnouncementID: a.ID,
		Revision:       a.Revision,
	}
	if a.TargetType == "users" {
		for _, userID := range a.TargetUserIDs {
			s.controlHub.PublishEvent(userID, event)
		}
		return
	}
	s.controlHub.Broadcast(event)
}

func cleanOptionalString(value *string) *string {
	if value == nil || *value == "" {
		return nil
	}
	return value
}

func positiveInt64(value *int64) *int64 {
	if value == nil || *value <= 0 {
		return nil
	}
	return value
}
