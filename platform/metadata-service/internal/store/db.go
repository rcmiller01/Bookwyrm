package store

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	"metadata-service/internal/config"
)

func NewPool(ctx context.Context, cfg *config.Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
	)
	return pgxpool.New(ctx, dsn)
}
