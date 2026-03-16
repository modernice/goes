package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/modernice/goes/internal/tourrunner"
)

func main() {
	logger := log.New(os.Stdout, "tour-runner: ", log.LstdFlags)
	repoRoot, err := tourrunner.ResolveRepoRoot(envOrDefault("TOUR_REPO_ROOT", "."))
	if err != nil {
		logger.Fatal(err)
	}

	lessonsDir := filepath.Join(repoRoot, envOrDefault("TOUR_LESSONS_DIR", "tour/lessons"))
	lessons, err := tourrunner.LoadLessons(lessonsDir)
	if err != nil {
		logger.Fatal(err)
	}

	handler := tourrunner.Handler{
		Service: tourrunner.Service{
			Lessons: lessons,
			Exec: tourrunner.DockerExecutor{
				DockerBin: envOrDefault("TOUR_DOCKER_BIN", "docker"),
				Image:     envOrDefault("TOUR_RUNNER_IMAGE", "goes-tour-runner:latest"),
				RepoRoot:  repoRoot,
				Memory:    envOrDefault("TOUR_RUNNER_MEMORY", "1g"),
				CPUs:      envOrDefault("TOUR_RUNNER_CPUS", "1"),
				PIDs:      envOrDefault("TOUR_RUNNER_PIDS", "128"),
			},
			TempDir: envOrDefault("TOUR_TMP_DIR", os.TempDir()),
			Timeout: durationFromEnv("TOUR_TIMEOUT", 60*time.Second),
		},
		AllowedFrom: os.Getenv("TOUR_ALLOWED_ORIGIN"),
		Logger:      logger,
	}

	addr := envOrDefault("TOUR_RUNNER_ADDR", ":8090")
	logger.Printf("serving %d lessons on %s", len(lessons), addr)
	logger.Fatal(http.ListenAndServe(addr, handler.Routes()))
}

func envOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func durationFromEnv(key string, fallback time.Duration) time.Duration {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return duration
}
