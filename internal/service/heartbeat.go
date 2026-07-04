package service

import (
	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

type HeartbeatService struct {
	heartbeatRepo *repository.HeartbeatRepo
	versionRepo   *repository.VersionRepo
}

func NewHeartbeatService(heartbeatRepo *repository.HeartbeatRepo, versionRepo *repository.VersionRepo) *HeartbeatService {
	return &HeartbeatService{heartbeatRepo: heartbeatRepo, versionRepo: versionRepo}
}

func (s *HeartbeatService) Report(userID int64, req model.HeartbeatRequest, ip string) (*model.HeartbeatResponse, error) {
	h := &model.HeartbeatLog{
		UserID:   userID,
		DeviceID: req.DeviceID,
	}
	if req.Platform != "" {
		h.Platform = &req.Platform
	}
	if req.AppVersion != "" {
		h.AppVersion = &req.AppVersion
	}
	if ip != "" {
		h.IPAddress = &ip
	}

	if err := s.heartbeatRepo.Create(h); err != nil {
		return nil, err
	}

	resp := &model.HeartbeatResponse{HasNewVersion: false}

	if req.Platform != "" {
		latest, _ := s.versionRepo.FindLatest(req.Platform)
		if latest != nil {
			resp.HasNewVersion = true
			resp.LatestVersionName = latest.VersionName
			resp.IsForceUpdate = latest.IsForce == 1
		}
	}

	return resp, nil
}
