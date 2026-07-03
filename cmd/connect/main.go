package main

import (
	"log"
	"net"
	"os"

	"github.com/sapliy/sapliy-core/internal/connect"
	"github.com/sapliy/sapliy-core/pkg/database"
	pb "github.com/sapliy/sapliy-core/proto/connect"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "postgres://user:password@127.0.0.1:5436/connect?sslmode=disable"
	}

	db, err := database.Connect(dsn)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer func() { _ = db.Close() }()

	if err := database.Migrate(db, "connect", "migrations/connect"); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	repo := connect.NewRepository(db)
	svc := connect.NewService(repo)

	port := os.Getenv("PORT")
	if port == "" {
		port = "50053"
	}

	lis, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	s := grpc.NewServer()
	pb.RegisterConnectServiceServer(s, svc)
	reflection.Register(s)

	log.Printf("Connect service starting on :%s", port)
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
