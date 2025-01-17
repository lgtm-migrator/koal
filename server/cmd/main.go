package main

import (
	"context"
	"net"
	"net/http"
	"time"

	sentryhttp "github.com/getsentry/sentry-go/http"

	"github.com/getsentry/sentry-go"

	todov1 "github.com/glyphack/koal/gen/proto/go/todo/v1"
	todoapi "github.com/glyphack/koal/internal/module/todo/api"
	grpc_middleware "github.com/grpc-ecosystem/go-grpc-middleware"
	grpc_auth "github.com/grpc-ecosystem/go-grpc-middleware/auth"
	grpc_ctxtags "github.com/grpc-ecosystem/go-grpc-middleware/tags"

	"github.com/glyphack/koal/ent"
	"github.com/glyphack/koal/ent/migrate"
	authv1 "github.com/glyphack/koal/gen/proto/go/auth/v1"
	"github.com/glyphack/koal/internal/config"
	authapi "github.com/glyphack/koal/internal/module/auth/api"
	"github.com/glyphack/koal/pkg/corsutils"
	"github.com/glyphack/koal/pkg/sentrygrpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	config.InitConfig()

	if viper.GetBool("DEBUG") {
		log.SetLevel(log.DebugLevel)
	}
	lis, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalln("Failed to listen:", err)
	}

	err = sentry.Init(sentry.ClientOptions{
		Dsn: viper.GetString("sentry_dsn"),
	})
	if err != nil {
		log.Fatalf("sentry.Init: %s", err)
	}
	defer sentry.Flush(2 * time.Second)

	ctx := context.Background()
	s := grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(
			grpc_auth.UnaryServerInterceptor(authapi.AuthFunc),
			grpc_ctxtags.UnaryServerInterceptor(),
			sentrygrpc.UnaryServerInterceptor(),
		),
	)
	client := newClient()
	if err := client.Schema.Create(ctx, migrate.WithDropIndex(true), migrate.WithDropColumn(true), migrate.WithGlobalUniqueID(true)); err != nil {
		log.Fatalf("failed creating schema resources: %v", err)
	}
	defer client.Close()
	authv1.RegisterAuthServiceServer(s, authapi.NewServer(client))
	todov1.RegisterTodoServiceServer(s, todoapi.NewServer(client))

	log.Println("Serving gRPC on 0.0.0.0:8080")
	go func() {
		log.Fatalln(s.Serve(lis))
	}()

	// Create a client connection to the gRPC server we just started
	// This is where the gRPC-Gateway proxies the requests
	conn, err := grpc.DialContext(
		ctx,
		"0.0.0.0:8080",
		grpc.WithBlock(),
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		log.Fatalln("Failed to dial server:", err)
	}

	r := http.NewServeMux()

	// Copied while building in docker
	fs := http.FileServer(http.Dir("./api-docs/"))
	r.Handle("/api-docs/", http.StripPrefix("/api-docs/", fs))

	gwmux := runtime.NewServeMux()
	r.Handle("/", corsutils.Cors(gwmux, corsutils.AllowOrigin))
	err = authv1.RegisterAuthServiceHandler(context.Background(), gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register auth service", err)
	}
	err = todov1.RegisterTodoServiceHandler(ctx, gwmux, conn)
	if err != nil {
		log.Fatalln("Failed to register todo service", err)
	}

	gwPortStr := viper.GetString("port")

	handler := sentryhttp.New(sentryhttp.Options{}).Handle(r)
	gwServer := &http.Server{
		Addr:    ":" + gwPortStr,
		Handler: corsutils.Cors(handler, corsutils.AllowOrigin),
	}

	log.Println("Serving gRPC-Gateway on http://0.0.0.0:" + gwPortStr)
	log.Fatalln(gwServer.ListenAndServe())
}

func newClient() *ent.Client {
	if viper.GetBool("debug") == true {
		client, err := ent.Open("sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
		if err != nil {
			log.Fatal(err)
		}
		return client.Debug()
	} else {
		client, err := ent.Open("postgres", viper.GetString("postgres_uri"))
		if err != nil {
			log.Fatal(err)
		}
		return client
	}
}
