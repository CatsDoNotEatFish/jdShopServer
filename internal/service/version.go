package service

import (
	"errors"

	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

var ErrVersionNotFound = errors.New("version not found")

type VersionService struct {
	repo *repository.VersionRepo
}

func NewVersionService(repo *repository.VersionRepo) *VersionService {
	return &VersionService{repo: repo}
}

func (s *VersionService) Create(req model.CreateVersionRequest) (*model.AppVersion, error) {
	v := &model.AppVersion{
		Platform:    req.Platform,
		VersionCode: req.VersionCode,
		VersionName: req.VersionName,
		Title:       req.Title,
	}

	if req.Description != "" {
		v.Description = &req.Description
	}
	if req.DownloadURL != "" {
		v.DownloadURL = &req.DownloadURL
	}
	if req.FileHash != "" {
		v.FileHash = &req.FileHash
	}
	if req.FileSize != nil {
		v.FileSize = req.FileSize
	}
	if req.IsForce != nil {
		if *req.IsForce {
			v.IsForce = 1
		}
	}

	if err := s.repo.Create(v); err != nil {
		return nil, err
	}
	return s.repo.FindByID(v.ID)
}

func (s *VersionService) Update(id int64, req model.UpdateVersionRequest) (*model.AppVersion, error) {
	v, err := s.repo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, ErrVersionNotFound
	}

	if err := s.repo.Update(id, req.Title, req.Description, req.DownloadURL,
		req.FileHash, req.FileSize, boolInt(req.IsForce), boolInt(req.IsLatest)); err != nil {
		return nil, err
	}
	return s.repo.FindByID(id)
}

func boolInt(value *bool) *int {
	if value == nil {
		return nil
	}
	result := 0
	if *value {
		result = 1
	}
	return &result
}

func (s *VersionService) Delete(id int64) error {
	v, err := s.repo.FindByID(id)
	if err != nil {
		return err
	}
	if v == nil {
		return ErrVersionNotFound
	}
	return s.repo.Delete(id)
}

func (s *VersionService) GetByID(id int64) (*model.AppVersion, error) {
	return s.repo.FindByID(id)
}

func (s *VersionService) List(page, pageSize int, platform string) ([]model.AppVersion, int64, error) {
	return s.repo.List(page, pageSize, platform)
}

func (s *VersionService) CheckLatest(platform string, currentVersionCode int64) (*model.CheckVersionResponse, error) {
	latest, err := s.repo.FindLatest(platform)
	if err != nil {
		return nil, err
	}

	resp := &model.CheckVersionResponse{HasUpdate: false}
	if latest != nil && latest.VersionCode > currentVersionCode {
		resp.HasUpdate = true
		resp.IsForce = latest.IsForce == 1
		resp.Version = latest
	}
	return resp, nil
}
