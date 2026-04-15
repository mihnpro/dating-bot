package grpc

import (
	"context"
	"database/sql"
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/dating-bot/user-profile-service/internal/domain/entity"
	"github.com/dating-bot/user-profile-service/internal/service"
	userprofilev1 "github.com/mihnpro/DatingBotProtos/gen/go/user-profile/v1"
)

type Server struct {
	userprofilev1.UnimplementedUserServiceServer
	svc *service.UserService
}

func NewServer(svc *service.UserService) *Server {
	return &Server{svc: svc}
}

// --- User RPCs ---

func (s *Server) RegisterUser(ctx context.Context, req *userprofilev1.RegisterUserRequest) (*userprofilev1.RegisterUserResponse, error) {
	user, err := s.svc.RegisterUser(
		ctx,
		req.TelegramId,
		req.Username,
		req.FirstName,
		req.LastName,
		req.ReferralBy,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "register user: %v", err)
	}
	return &userprofilev1.RegisterUserResponse{User: toUserProto(user)}, nil
}

func (s *Server) GetUser(ctx context.Context, req *userprofilev1.GetUserRequest) (*userprofilev1.GetUserResponse, error) {
	user, err := s.svc.GetUser(ctx, req.Id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "get user: %v", err)
	}
	return &userprofilev1.GetUserResponse{User: toUserProto(user)}, nil
}

func (s *Server) GetUserByTelegramID(ctx context.Context, req *userprofilev1.GetUserByTelegramIDRequest) (*userprofilev1.GetUserResponse, error) {
	user, err := s.svc.GetUserByTelegramID(ctx, req.TelegramId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "get user by telegram id: %v", err)
	}
	return &userprofilev1.GetUserResponse{User: toUserProto(user)}, nil
}

func (s *Server) UpdateUser(ctx context.Context, req *userprofilev1.UpdateUserRequest) (*userprofilev1.UpdateUserResponse, error) {
	var username, firstName, lastName *string
	if req.Username != nil {
		username = req.Username
	}
	if req.FirstName != nil {
		firstName = req.FirstName
	}
	if req.LastName != nil {
		lastName = req.LastName
	}
	var userStatus *entity.UserStatus
	if req.Status != nil {
		s := entity.UserStatus(*req.Status)
		userStatus = &s
	}

	user, err := s.svc.UpdateUser(ctx, req.Id, username, firstName, lastName, userStatus)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "update user: %v", err)
	}
	return &userprofilev1.UpdateUserResponse{User: toUserProto(user)}, nil
}

func (s *Server) DeleteUser(ctx context.Context, req *userprofilev1.DeleteUserRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteUser(ctx, req.Id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "user not found")
		}
		return nil, status.Errorf(codes.Internal, "delete user: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListUsers(ctx context.Context, req *userprofilev1.ListUsersRequest) (*userprofilev1.ListUsersResponse, error) {
	var userStatus *entity.UserStatus
	if req.Status != nil {
		st := entity.UserStatus(*req.Status)
		userStatus = &st
	}

	users, total, err := s.svc.ListUsers(ctx, req.Page, req.PageSize, userStatus)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list users: %v", err)
	}

	var protoUsers []*userprofilev1.User
	for _, u := range users {
		protoUsers = append(protoUsers, toUserProto(u))
	}

	return &userprofilev1.ListUsersResponse{
		Users:      protoUsers,
		TotalCount: total,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

// --- Profile RPCs ---

func (s *Server) CreateProfile(ctx context.Context, req *userprofilev1.CreateProfileRequest) (*userprofilev1.CreateProfileResponse, error) {
	profile, err := s.svc.CreateProfile(
		ctx,
		req.UserId,
		int(req.Age),
		entity.Gender(req.Gender),
		req.City,
		req.Interests,
	)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "create profile: %v", err)
	}
	return &userprofilev1.CreateProfileResponse{Profile: toProfileProto(profile)}, nil
}

func (s *Server) GetProfile(ctx context.Context, req *userprofilev1.GetProfileRequest) (*userprofilev1.GetProfileResponse, error) {
	profile, err := s.svc.GetProfile(ctx, req.UserId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return nil, status.Errorf(codes.Internal, "get profile: %v", err)
	}
	return &userprofilev1.GetProfileResponse{Profile: toProfileProto(profile)}, nil
}

func (s *Server) UpdateProfile(ctx context.Context, req *userprofilev1.UpdateProfileRequest) (*userprofilev1.UpdateProfileResponse, error) {
	var gender *entity.Gender
	if req.Gender != nil {
		g := entity.Gender(*req.Gender)
		gender = &g
	}

	profile, err := s.svc.UpdateProfile(
		ctx,
		req.UserId,
		req.Age,
		gender,
		req.City,
		req.Interests,
		req.PhotosCount,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return nil, status.Errorf(codes.Internal, "update profile: %v", err)
	}
	return &userprofilev1.UpdateProfileResponse{Profile: toProfileProto(profile)}, nil
}

func (s *Server) DeleteProfile(ctx context.Context, req *userprofilev1.DeleteProfileRequest) (*emptypb.Empty, error) {
	if err := s.svc.DeleteProfile(ctx, req.UserId); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, status.Error(codes.NotFound, "profile not found")
		}
		return nil, status.Errorf(codes.Internal, "delete profile: %v", err)
	}
	return &emptypb.Empty{}, nil
}

func (s *Server) ListProfiles(ctx context.Context, req *userprofilev1.ListProfilesRequest) (*userprofilev1.ListProfilesResponse, error) {
	var gender *entity.Gender
	if req.Gender != nil {
		g := entity.Gender(*req.Gender)
		gender = &g
	}
	var city *string
	if req.City != nil {
		city = req.City
	}

	profiles, total, err := s.svc.ListProfiles(ctx, req.Page, req.PageSize, gender, city, req.MinAge, req.MaxAge)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list profiles: %v", err)
	}

	var protoProfiles []*userprofilev1.Profile
	for _, p := range profiles {
		protoProfiles = append(protoProfiles, toProfileProto(p))
	}

	return &userprofilev1.ListProfilesResponse{
		Profiles:   protoProfiles,
		TotalCount: total,
		Page:       req.Page,
		PageSize:   req.PageSize,
	}, nil
}

// --- Mappers ---

func toUserProto(u *entity.User) *userprofilev1.User {
	pb := &userprofilev1.User{
		Id:           u.ID,
		TelegramId:   u.TelegramID,
		Username:     u.Username,
		FirstName:    u.FirstName,
		LastName:     u.LastName,
		RegisteredAt: timestamppb.New(u.RegisteredAt),
		LastActive:   timestamppb.New(u.LastActive),
		Status:       string(u.Status),
	}
	if u.ReferralBy != nil {
		val := *u.ReferralBy
		pb.ReferralBy = &val
	}
	return pb
}

func toProfileProto(p *entity.Profile) *userprofilev1.Profile {
	return &userprofilev1.Profile{
		Id:              p.ID,
		UserId:          p.UserID,
		Age:             int32(p.Age),
		Gender:          string(p.Gender),
		City:            p.City,
		Interests:       p.Interests,
		PhotosCount:     int32(p.PhotosCount),
		FullnessPercent: float32(p.FullnessPercent),
		UpdatedAt:       timestamppb.New(p.UpdatedAt),
	}
}
