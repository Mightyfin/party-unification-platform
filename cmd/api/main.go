package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Mightyfin/party-unification-platform/internal/auth"
	"github.com/Mightyfin/party-unification-platform/internal/config"
	"github.com/Mightyfin/party-unification-platform/internal/database"
	"github.com/Mightyfin/party-unification-platform/internal/httpserver"
	"github.com/Mightyfin/party-unification-platform/internal/party"
)
func main(){
	logger:=slog.New(slog.NewJSONHandler(os.Stdout,nil));cfg,err:=config.Load();if err!=nil{logger.Error("invalid configuration","error",err);os.Exit(1)}
	ctx,cancel:=context.WithTimeout(context.Background(),30*time.Second);defer cancel();db,err:=database.Open(ctx,cfg.DatabaseURL);if err!=nil{logger.Error("database unavailable","error",err);os.Exit(1)};defer db.Close()
	var verifier auth.Verifier;if !cfg.AuthDisabled{verifier,err=auth.New(ctx,cfg.OIDCIssuer,cfg.OIDCAudience);if err!=nil{logger.Error("identity unavailable","error",err);os.Exit(1)}}
	server:=httpserver.New(cfg.HTTPAddress,cfg.Environment,cfg.AuthDisabled,verifier,db,party.NewStore(db,cfg.IdentifierHMACKey),logger)
	go func(){logger.Info("Party Platform starting","address",cfg.HTTPAddress);if err:=server.ListenAndServe();err!=nil&&!errors.Is(err,http.ErrServerClosed){logger.Error("server failed","error",err);os.Exit(1)}}()
	sig:=make(chan os.Signal,1);signal.Notify(sig,syscall.SIGINT,syscall.SIGTERM);<-sig;shutdown,c:=context.WithTimeout(context.Background(),10*time.Second);defer c();_ = server.Shutdown(shutdown)
}
