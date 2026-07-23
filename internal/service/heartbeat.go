package service

import (
	"jdShopServer/internal/model"
	"jdShopServer/internal/repository"
)

type HeartbeatService struct {
	heartbeatRepo *repository.HeartbeatRepo
	versionRepo   *repository.VersionRepo
	userRepo      *repository.UserRepo
	accessSvc     *AccessService
}

func NewHeartbeatService(heartbeatRepo *repository.HeartbeatRepo, versionRepo *repository.VersionRepo,
	userRepo *repository.UserRepo, accessSvc *AccessService) *HeartbeatService {
	return &HeartbeatService{
		heartbeatRepo: heartbeatRepo,
		versionRepo:   versionRepo,
		userRepo:      userRepo,
		accessSvc:     accessSvc,
	}
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

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotFound
	}
	access, err := s.accessSvc.Evaluate(user)
	if err != nil {
		return nil, err
	}

	resp := &model.HeartbeatResponse{HasNewVersion: false, Access: access}

	if req.Platform != "" {
		latest, _ := s.versionRepo.FindLatest(req.Platform)
		if latest != nil {
			resp.LatestVersionCode = latest.VersionCode
			resp.LatestVersionName = latest.VersionName
			resp.LatestDownloadURL = latest.DownloadURL
			resp.LatestFileHash = latest.FileHash
			resp.LatestFileSize = latest.FileSize
			if req.AppVersionCode > 0 {
				resp.HasNewVersion = latest.VersionCode > req.AppVersionCode
			} else if req.AppVersion != "" {
				resp.HasNewVersion = latest.VersionName != req.AppVersion
			}
			resp.IsForceUpdate = resp.HasNewVersion && latest.IsForce == 1
		}
	}

	return resp, nil
}
