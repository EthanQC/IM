package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spf13/viper"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	imv1 "github.com/EthanQC/IM/api/gen/im/v1"
)

type Config struct {
	Server struct {
		HTTPPort         int           `mapstructure:"http_port"`
		GrpcAddrIdentity string        `mapstructure:"grpc_addr_identity"`
		GrpcTimeout      time.Duration `mapstructure:"grpc_timeout"`
		ReadTimeout      time.Duration `mapstructure:"read_timeout"`
		WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	} `mapstructure:"server"`
	JWT struct {
		Secret string `mapstructure:"secret"`
	} `mapstructure:"jwt"`
}

type Gateway struct {
	cfg     Config
	router  *gin.Engine
	idc     imv1.IdentityServiceClient
	timeout time.Duration
}

func main() {
	cfgPath := flag.String("config", "configs/dev/api_gateway.yaml", "配置文件路径")
	flag.Parse()

	cfg, err := loadConfig(*cfgPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	idConn, err := grpc.Dial(cfg.Server.GrpcAddrIdentity, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("dial identity_service: %v", err)
	}
	defer idConn.Close()

	gw := &Gateway{
		cfg:     cfg,
		router:  gin.Default(),
		idc:     imv1.NewIdentityServiceClient(idConn),
		timeout: cfg.Server.GrpcTimeout,
	}
	gw.registerRoutes()

	addr := fmt.Sprintf(":%d", cfg.Server.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      gw.router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}
	log.Printf("api_gateway listening on %s", addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("gateway failed: %v", err)
	}
}

func loadConfig(path string) (Config, error) {
	viper.SetConfigFile(path)
	viper.SetConfigType("yaml")
	viper.SetDefault("server.grpc_timeout", "3s")
	viper.SetDefault("server.read_timeout", "5s")
	viper.SetDefault("server.write_timeout", "5s")
	var cfg Config
	if err := viper.ReadInConfig(); err != nil {
		return cfg, err
	}
	if err := viper.Unmarshal(&cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (g *Gateway) registerRoutes() {
	g.router.GET("/healthz", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"status": "ok"}) })
	g.router.POST("/api/auth/login", g.handleLogin)
	g.router.POST("/api/auth/refresh", g.handleRefresh)
}

type loginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type authResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int64  `json:"expires_in"`
}

func (g *Gateway) handleLogin(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()
	resp, err := g.idc.Login(ctx, &imv1.LoginRequest{Username: req.Username, Password: req.Password})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
	})
}

func (g *Gateway) handleRefresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), g.timeout)
	defer cancel()
	resp, err := g.idc.Refresh(ctx, &imv1.RefreshRequest{RefreshToken: req.RefreshToken})
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, authResponse{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresIn:    resp.ExpiresIn,
	})
}
