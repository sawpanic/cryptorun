package db

import (
	"context"
	"database/sql"
	_ "github.com/jackc/pgx/v5/stdlib"
)

type Stores struct { Timescale *sql.DB; Postgres *sql.DB }

func Connect(pgDSN string) (*Stores, error) {
	db, err := sql.Open("pgx", pgDSN)
	if err != nil { return nil, err }
	return &Stores{ Timescale: db, Postgres: db }, nil
}

func (s *Stores) SaveAudit(ctx context.Context, msg string) error {
	_, err := s.Postgres.ExecContext(ctx, "INSERT INTO audit_log(msg) VALUES($1)", msg)
	return err
}
