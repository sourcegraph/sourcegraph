package api

import (
	"math/rand"
	"time"

	"github.com/sourcegraph/sourcegraph/internal/appliance/maintenance/backend/operator"
)

const InstallTaskWaitForCluster = 0
const InstallTaskSetup = 1
const InstallTaskStart = 2

func createInstallTasks() []operator.Task {
	return []operator.Task{
		{
			Title:       "Warming up",
			Description: "Setting up basic resources",
			Started:     true,
			Finished:    false,
			Weight:      1,
		},
		{
			Title:       "Setup",
			Description: "Setting up Sourcegraph Search",
			Started:     false,
			Finished:    false,
			Weight:      25,
		},
		{
			Title:       "Start",
			Description: "Start Sourcegraph",
			Started:     false,
			Finished:    false,
			Weight:      1,
		},
	}
}

func createFakeUpgradeTasks() []operator.Task {
	return []operator.Task{
		{
			Title:       "Upgrade",
			Description: "Upgrade Sourcegraph",
			Started:     false,
			Finished:    false,
			Weight:      5,
		},
		{
			Title:       "Migrate",
			Description: "Run migration tasks",
			Started:     false,
			Finished:    false,
			Weight:      13,
		},
	}
}

func progressTasks(tasks []operator.Task) []operator.Task {
	var result []operator.Task

	var previousStarted bool = true
	var previousFinished bool = true

	for _, task := range tasks {
		var beforeStarted bool = task.Started
		task.Started = previousFinished && (task.Started || (previousStarted && rand.Intn(2) == 0))
		previousStarted = task.Started
		task.Finished = beforeStarted && (task.Progress == 100)
		previousFinished = task.Finished
		task.LastUpdate = time.Now()

		result = append(result, task)
	}

	return result
}

func calculateProgress() ([]operator.Task, int) {
	var result []operator.Task

	var taskWeights int = 0
	for _, t := range installTasks {
		taskWeights += t.Weight
	}

	var progress float32 = 0

	for _, t := range installTasks {
		if t.Finished {
			progress += float32(t.Weight)
		} else if t.Started {
			if t.Progress > 100 {
				t.Progress = 100
			}
			progress += float32(t.Weight * t.Progress / 100)
		}

		result = append(result, t)
	}

	return result, int(progress / float32(taskWeights) * 100)
}
