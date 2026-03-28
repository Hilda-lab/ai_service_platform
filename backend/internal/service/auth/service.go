package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	goredis "github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"ai-service-platform/backend/internal/domain/entity"
	"ai-service-platform/backend/internal/domain/repository"
	jwtpkg "ai-service-platform/backend/pkg/jwt"
)

var (
	ErrInvalidCredentials = errors.New("invalid email or password")
	ErrEmailExists        = errors.New("email already exists")
)

type Service struct {
	userRepo   repository.UserRepository
	redis      *goredis.Client
	jwtSecret  string
	jwtExpires time.Duration
}

func NewService(userRepo repository.UserRepository, redisClient *goredis.Client, jwtSecret string, jwtExpires time.Duration) *Service {
	return &Service{
		userRepo:   userRepo,
		redis:      redisClient,
		jwtSecret:  jwtSecret,
		jwtExpires: jwtExpires,
	}
}

func (s *Service) Register(ctx context.Context, email, password string) (*entity.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	existing, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return nil, ErrEmailExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &entity.User{
		Email:    email,
		Password: string(hash),
	}
	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *Service) Login(ctx context.Context, email, password string) (string, *entity.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.userRepo.GetByEmail(ctx, email)
	if err != nil {
		return "", nil, err
	}
	if user == nil {
		return "", nil, ErrInvalidCredentials
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password)); err != nil {
		return "", nil, ErrInvalidCredentials
	}

	token, err := jwtpkg.GenerateToken(user.ID, s.jwtSecret, s.jwtExpires)
	if err != nil {
		return "", nil, err
	}

	if s.redis != nil {
		key := fmt.Sprintf("session:user:%d", user.ID)
		if err := s.redis.Set(ctx, key, token, s.jwtExpires).Err(); err != nil {
			return "", nil, err
		}
	}

	return token, user, nil
}

func (s *Service) GetProfile(ctx context.Context, userID uint) (*entity.User, error) {
	return s.userRepo.GetByID(ctx, userID)
}
