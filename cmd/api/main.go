package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"code-execution/internal/config"
	"code-execution/internal/db"
	"code-execution/internal/discovery"
	"code-execution/internal/execution/module"
	"code-execution/internal/messaging"
	"code-execution/internal/server"
)

func main() {
	log.Printf("Iniciando aplicación...")
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config error: %v", err)
	}
	log.Printf("Configuración cargada correctamente")

	ctx := context.Background()

	log.Printf("Fase: Inicialización de base de datos")
	pool, err := db.NewPool(ctx, cfg.PostgresURL)
	if err != nil {
		log.Fatalf("postgres connection error: %v", err)
	}
	defer pool.Close()

	log.Printf("Fase: Inicialización de mensajería (RabbitMQ)")
	rabbitConn, err := messaging.NewConnection(cfg.RabbitMQURL)
	if err != nil {
		log.Fatalf("rabbitmq connection error: %v", err)
	}
	defer rabbitConn.Close()

	log.Printf("Fase: Inicialización del Módulo de Ejecución")
	executionModule, err := module.New(ctx, cfg, pool, rabbitConn)
	if err != nil {
		log.Fatalf("execution module initialization error: %v", err)
	}

	consumerCtx, consumerCancel := context.WithCancel(context.Background())
	defer consumerCancel()

	if err := executionModule.StartConsumers(consumerCtx); err != nil {
		log.Fatalf("execution consumers initialization error: %v", err)
	}

	log.Printf("Fase: Configuración de Service Discovery (Eureka)")
	eurekaClient := discovery.NewClient(cfg.Eureka)
	heartbeatCtx, heartbeatCancel := context.WithCancel(context.Background())
	defer heartbeatCancel()

	if cfg.Eureka.Enabled {
		registerCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := eurekaClient.Register(registerCtx); err != nil {
			cancel()
			log.Fatalf("eureka register error: %v", err)
		}
		cancel()

		eurekaClient.StartHeartbeat(heartbeatCtx, func(err error) {
			log.Printf("eureka heartbeat error: %v", err)
		})
	}

	log.Printf("Fase: Configuración del servidor HTTP")
	router := server.NewRouter(cfg, executionModule.HTTPHandler)

	httpServer := &http.Server{
		Addr:         ":" + cfg.AppPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	serverErr := make(chan error, 1)
	go func() {
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
		}
	}()

	log.Printf("%s listening on port %s", cfg.AppName, cfg.AppPort)

	signalCh := make(chan os.Signal, 1)
	signal.Notify(signalCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-signalCh:
		log.Printf("Señal de apagado recibida: %s", sig.String())
	case err := <-serverErr:
		log.Printf("Error crítico en el servidor: %v", err)
	}

	log.Printf("Iniciando proceso de apagado gradual (Graceful Shutdown)...")
	heartbeatCancel()
	consumerCancel()

	if cfg.Eureka.Enabled {
		log.Printf("Dando de baja instancia en Eureka...")
		deregisterCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := eurekaClient.Deregister(deregisterCtx); err != nil {
			log.Printf("Error al dar de baja en Eureka: %v", err)
		}
		cancel()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Error durante el apagado del servidor HTTP: %v", err)
	}
	log.Printf("Aplicación finalizada correctamente")
}
