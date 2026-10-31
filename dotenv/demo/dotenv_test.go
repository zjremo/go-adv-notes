package main

import (
	"os"
	"strconv"
	"testing"

	dotenv "github.com/joho/godotenv"
)

func TestDotenv(t *testing.T) {
	err := dotenv.Load()
	if err != nil {
		t.Fatal("Load env failed, err:", err)
	}

	targetName, targetJob := "zjremo", "student"
	targetAge := 22

	name, job := os.Getenv("name"), os.Getenv("job")
	if name != targetName {
		t.Errorf("name != targetName, name: %s, targetName: %s\n", name, targetName)
	}
	if job != targetJob {
		t.Errorf("job != targetJob, job: %s, targetJob: %s\n", job, targetJob)
	}
	age, err := strconv.Atoi(os.Getenv("age"))
	if err != nil {
		t.Fatal("strconv.Atoi failed when get age, err:", err)
	}

	if age != targetAge {
		t.Errorf("name != targetName, name: %s, targetName: %s\n", name, targetName)
	}
}
