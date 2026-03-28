package vision

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"ai-service-platform/backend/internal/domain/entity"
	"ai-service-platform/backend/internal/domain/repository"
	"ai-service-platform/backend/internal/infrastructure/mq/rabbitmq"
)

type AnalyzeFunc func(ctx context.Context, model, prompt, mimeType string, imageBytes []byte) (string, error)

type Service struct {
	repo            repository.VisionRepository
	analyzeFn       AnalyzeFunc
	rabbitURL       string
	queueName       string
	defaultProvider string
	defaultModel    string
	mock            bool
}

type RecognizeRequest struct {
	UserID     uint
	Provider   string
	Model      string
	Prompt     string
	FileName   string
	MimeType   string
	ImageBytes []byte
}

func NewService(repo repository.VisionRepository, analyzeFn AnalyzeFunc, rabbitURL, queueName, defaultProvider, defaultModel string, mock bool) *Service {
	if defaultProvider == "" {
		defaultProvider = "openai"
	}
	if defaultModel == "" {
		defaultModel = "gpt-4.1-mini"
	}
	if queueName == "" {
		queueName = "vision_tasks"
	}
	return &Service{repo: repo, analyzeFn: analyzeFn, rabbitURL: rabbitURL, queueName: queueName, defaultProvider: defaultProvider, defaultModel: defaultModel, mock: mock}
}

func (s *Service) RecognizeSync(ctx context.Context, req RecognizeRequest) (*entity.VisionTask, error) {
	task := &entity.VisionTask{
		UserID:   req.UserID,
		Status:   "processing",
		Provider: choose(req.Provider, s.defaultProvider),
		Model:    choose(req.Model, s.defaultModel),
		Prompt:   req.Prompt,
		FileName: req.FileName,
		MimeType: req.MimeType,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}

	result, err := s.analyze(ctx, task.Model, req.Prompt, req.MimeType, req.ImageBytes)
	if err != nil {
		task.Status = "failed"
		task.ErrorMessage = err.Error()
		_ = s.repo.UpdateTask(ctx, task)
		return nil, err
	}

	task.Status = "completed"
	task.Result = result
	if err := s.repo.UpdateTask(ctx, task); err != nil {
		return nil, err
	}
	return task, nil
}

func (s *Service) SubmitAsync(ctx context.Context, req RecognizeRequest) (*entity.VisionTask, error) {
	encoded := base64.StdEncoding.EncodeToString(req.ImageBytes)
	task := &entity.VisionTask{
		UserID:      req.UserID,
		Status:      "pending",
		Provider:    choose(req.Provider, s.defaultProvider),
		Model:       choose(req.Model, s.defaultModel),
		Prompt:      req.Prompt,
		FileName:    req.FileName,
		MimeType:    req.MimeType,
		ImageBase64: encoded,
	}
	if err := s.repo.CreateTask(ctx, task); err != nil {
		return nil, err
	}

	if err := rabbitmq.PublishVisionTask(ctx, s.rabbitURL, s.queueName, task.ID); err != nil {
		task.Status = "failed"
		task.ErrorMessage = "publish task failed: " + err.Error()
		_ = s.repo.UpdateTask(ctx, task)
		return nil, err
	}

	return task, nil
}

func (s *Service) ProcessTask(ctx context.Context, taskID uint) error {
	task, err := s.repo.GetTaskByIDAnyUser(ctx, taskID)
	if err != nil {
		return err
	}
	if task == nil {
		return fmt.Errorf("task %d not found", taskID)
	}
	if task.Status == "completed" {
		return nil
	}

	data, err := base64.StdEncoding.DecodeString(task.ImageBase64)
	if err != nil {
		task.Status = "failed"
		task.ErrorMessage = "decode image failed"
		_ = s.repo.UpdateTask(ctx, task)
		return err
	}

	task.Status = "processing"
	_ = s.repo.UpdateTask(ctx, task)

	result, err := s.analyze(ctx, task.Model, task.Prompt, task.MimeType, data)
	if err != nil {
		task.Status = "failed"
		task.ErrorMessage = err.Error()
		_ = s.repo.UpdateTask(ctx, task)
		return err
	}

	task.Status = "completed"
	task.Result = result
	task.ImageBase64 = ""
	task.ErrorMessage = ""
	return s.repo.UpdateTask(ctx, task)
}

func (s *Service) GetTask(ctx context.Context, userID, taskID uint) (*entity.VisionTask, error) {
	task, err := s.repo.GetTaskByID(ctx, taskID, userID)
	if err != nil {
		return nil, err
	}
	if task == nil {
		return nil, errors.New("task not found")
	}
	return task, nil
}

func (s *Service) analyze(ctx context.Context, model, prompt, mime string, image []byte) (string, error) {
	if len(image) == 0 {
		return "", errors.New("image is empty")
	}
	if s.mock {
		return fmt.Sprintf("[mock] 图片识别成功，mime=%s，size=%d bytes", mime, len(image)), nil
	}
	if s.analyzeFn == nil {
		return "", errors.New("vision analyzer is not configured")
	}
	result, err := s.analyzeFn(ctx, model, prompt, mime, image)
	if err != nil {
		return "", err
	}
	result = strings.TrimSpace(result)
	if result == "" {
		return "", errors.New("vision result is empty")
	}
	return result, nil
}

func choose(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
