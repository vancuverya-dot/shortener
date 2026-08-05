// Package grpcapi реализует gRPC-фасад над бизнес-логикой сервиса
// сокращения ссылок. Хэндлеры делегируют работу тому же слою storage,
// что и HTTP-обработчики.
package grpcapi

import (
	"context"
	"errors"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"

	"github.com/sixafter/nanoid"
	"github.com/vancuverya-dot/shortener/internal/auth"
	"github.com/vancuverya-dot/shortener/internal/grpcapi/proto"
	"github.com/vancuverya-dot/shortener/internal/observer"
	"github.com/vancuverya-dot/shortener/internal/service"
	"github.com/vancuverya-dot/shortener/internal/storage"
)

const authHeader = "authorization"

// Server реализует proto.ShortenerServiceServer.
type Server struct {
	proto.UnimplementedShortenerServiceServer

	gen      nanoid.Interface
	servPath string
	audit    *observer.Subject
}

// New создаёт gRPC-сервер сервиса сокращения ссылок.
// servPath — базовый адрес, добавляемый к сгенерированным идентификаторам;
// audit — издатель событий аудита, общий с HTTP-слоем.
func New(gen nanoid.Interface, servPath string, audit *observer.Subject) *Server {
	return &Server{
		gen:      gen,
		servPath: servPath,
		audit:    audit,
	}
}

// userIDFromContext извлекает идентификатор пользователя из метаданных
// запроса. Если токен отсутствует или недействителен, создаётся новый
// пользователь, а его токен отправляется клиенту в заголовках ответа.
func (s *Server) userIDFromContext(ctx context.Context) (string, error) {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(authHeader); len(values) > 0 {
			if userID, err := auth.ParseToken(values[0]); err == nil {
				return userID, nil
			}
		}
	}

	userID, token, err := auth.NewToken()
	if err != nil {
		return "", status.Error(codes.Internal, "не удалось создать токен пользователя")
	}
	if err := grpc.SetHeader(ctx, metadata.Pairs(authHeader, token)); err != nil {
		service.Log.Warnw(err.Error(), "event", "grpc set auth header failed")
	}
	return userID, nil
}

// ShortenURL создаёт короткую ссылку для переданного URL.
func (s *Server) ShortenURL(ctx context.Context, req *proto.URLShortenRequest) (*proto.URLShortenResponse, error) {
	if req.GetUrl() == "" {
		return nil, status.Error(codes.InvalidArgument, "url не может быть пустым")
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	id, err := s.gen.New()
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "grpc - Error generating Nano ID")
		return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	shortURL, err := storage.InsertURL(dbCtx, id.String(), req.GetUrl(), userID)
	if err != nil {
		if errors.Is(err, storage.ErrConflict) {
			return nil, status.Error(codes.AlreadyExists, s.servPath+shortURL)
		}
		service.Log.Errorw(err.Error(), "event", "grpc - Error inserting url")
		return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
	}

	s.audit.Notify(observer.NewEvent(observer.ActionShorten, userID, req.GetUrl()))

	return &proto.URLShortenResponse{Result: s.servPath + shortURL}, nil
}

// ExpandURL возвращает оригинальный URL по короткому идентификатору.
func (s *Server) ExpandURL(ctx context.Context, req *proto.URLExpandRequest) (*proto.URLExpandResponse, error) {
	if req.GetId() == "" {
		return nil, status.Error(codes.InvalidArgument, "id не может быть пустым")
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	target, isDeleted, err := storage.GetOriginalURL(dbCtx, req.GetId())
	if err != nil {
		return nil, status.Error(codes.NotFound, "ссылка не найдена")
	}
	if isDeleted {
		return nil, status.Error(codes.NotFound, "ссылка удалена")
	}

	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}
	s.audit.Notify(observer.NewEvent(observer.ActionFollow, userID, target))

	return &proto.URLExpandResponse{Result: target}, nil
}

// ListUserURLs возвращает все ссылки, созданные текущим пользователем.
func (s *Server) ListUserURLs(ctx context.Context, _ *emptypb.Empty) (*proto.UserURLsResponse, error) {
	userID, err := s.userIDFromContext(ctx)
	if err != nil {
		return nil, err
	}

	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	shortURLs, originalURLs, err := storage.GetURLsByUser(dbCtx, userID)
	if err != nil {
		service.Log.Errorw(err.Error(), "event", "grpc - Error retrieving URLs by user")
		return nil, status.Error(codes.Internal, "внутренняя ошибка сервера")
	}

	items := make([]*proto.URLData, len(shortURLs))
	for i := range shortURLs {
		items[i] = &proto.URLData{
			ShortUrl:    s.servPath + shortURLs[i],
			OriginalUrl: originalURLs[i],
		}
	}

	return &proto.UserURLsResponse{Url: items}, nil
}
